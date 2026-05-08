package d4s

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"paperlink/ptf"
	"paperlink/pvf"
	"paperlink/util"
	"path/filepath"
	"strconv"

	"paperlink/db/entity"
	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TakeBookResponse struct {
	ID string `json:"id"`
}

// TakeBook godoc
// @Summary      Take Digi4School book
// @Description  Marks a Digi4School book as taken (currently a no-op placeholder) and returns the book ID.
// @Tags         digi4school
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  TakeBookResponse
// @Failure      400  {object}  routes.ErrorResponse "Invalid book ID"
// @Failure      401  {object}  routes.ErrorResponse "Unauthorized"
// @Failure      404  {object}  routes.ErrorResponse "Book not found"
// @Router       /api/v1/d4s/takeBook/{id} [post]
// @Security     BearerAuth
func TakeBook(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid book id")
		return
	}

	userID := c.GetInt("userId")

	book, err := repo.Digi4SchoolBook.Get(id)
	if err != nil || book == nil {
		routes.JSONError(c, http.StatusNotFound, "book not found")
		return
	}

	if book.FileUUID == "" {
		routes.JSONError(c, http.StatusBadRequest, "book has no file")
		return
	}

	file := repo.FileDocument.GetByUUID(book.FileUUID)
	if file == nil {
		routes.JSONError(c, http.StatusNotFound, "book file not found")
		return
	}

	thumbDst := "./data/uploads/" + book.FileUUID + "_thumb.ptf"
	if _, err := os.Stat(thumbDst); os.IsNotExist(err) {
		if genErr := generateD4SThumbnailPTF(file.Path, thumbDst); genErr != nil {
			log.Errorf("failed to generate thumbnail ptf for book %d: %v", id, genErr)
		}
	}

	doc := entity.Document{
		UUID:        uuid.NewString(),
		Name:        book.BookName,
		Description: "Digi4School book",
		UserID:      userID,
		FileUUID:    book.FileUUID,
	}

	if err := repo.Document.Save(&doc); err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to create document")
		return
	}

	routes.JSONSuccessOK(c, TakeBookResponse{ID: doc.UUID})
}

func generateD4SThumbnailPTF(pvfPath, thumbDst string) error {
	metadata, err := pvf.ReadMetadata(pvfPath)
	if err != nil {
		return fmt.Errorf("failed to read pvf metadata: %w", err)
	}

	tmpDir, err := os.MkdirTemp("./data/tmp/uploads", "d4s-thumb-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pagePaths := make([]string, 0, metadata.PageCount)
	for i := uint64(1); i <= metadata.PageCount; i++ {
		pageData, err := pvf.ReadPage(pvfPath, i)
		if err != nil {
			return fmt.Errorf("failed to read page %d: %w", i, err)
		}
		pagePath := filepath.Join(tmpDir, fmt.Sprintf("page-%04d.pdf", i))
		if err := os.WriteFile(pagePath, pageData, 0o644); err != nil {
			return fmt.Errorf("failed to write page %d: %w", i, err)
		}
		pagePaths = append(pagePaths, pagePath)
	}

	mergedPDF := filepath.Join(tmpDir, "merged.pdf")
	args := append([]string{"--empty", "--pages"}, pagePaths...)
	args = append(args, "--", mergedPDF)
	if output, err := exec.Command("qpdf", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("qpdf merge failed: %w: %s", err, string(output))
	}

	thumbPTFFile, err := ptf.WriteThumbnailPTFFromPDF(mergedPDF)
	if err != nil {
		return fmt.Errorf("failed to write thumbnail ptf: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(thumbPTFFile))

	if err := util.CopyFile(thumbPTFFile, thumbDst); err != nil {
		return fmt.Errorf("failed to copy thumbnail ptf: %w", err)
	}

	return nil
}
