package d4s

import (
	"net/http"
	"os"
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

	doc := entity.Document{
		UUID:        uuid.NewString(),
		Name:        book.BookName,
		Description: "Digi4School book",
		UserID:      userID,
		FileUUID:    book.FileUUID,
	}

	file := repo.FileDocument.GetByUUID(book.FileUUID)
	if file == nil {
		routes.JSONError(c, http.StatusNotFound, "book file not found")
		return
	}

	if err := repo.Document.Save(&doc); err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to create document")
		return
	}

	thumbDst := "./data/uploads/" + book.FileUUID + "_thumb.ptf"
	if _, err := os.Stat(thumbDst); os.IsNotExist(err) {
		pageData, err := pvf.ReadPage(file.Path, 1)
		if err != nil {
			log.Warnf("failed to read first pvf page for thumbnail: %v", err)
		} else {
			if err := os.MkdirAll("./data/tmp/uploads", 0750); err == nil {
				tmpPDF := "./data/tmp/uploads/" + book.FileUUID + "_thumb_src.pdf"
				defer os.Remove(tmpPDF)

				if err := os.WriteFile(tmpPDF, pageData, 0o644); err != nil {
					log.Warnf("failed to write temp pdf for thumbnail: %v", err)
				} else {
					thumbPTFFile, err := ptf.WriteThumbnailPTFFromPDF(tmpPDF)
					if err != nil {
						log.Warnf("failed to generate thumbnail ptf: %v", err)
					} else {
						defer os.RemoveAll(filepath.Dir(thumbPTFFile))
						if err := util.CopyFile(thumbPTFFile, thumbDst); err != nil {
							log.Warnf("failed to copy thumbnail ptf: %v", err)
						}
					}
				}
			}
		}
	}

	routes.JSONSuccessOK(c, TakeBookResponse{ID: doc.UUID})
}
