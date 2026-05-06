package d4s

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"paperlink/pvf"
	"path"
	"strconv"
	"strings"
	"time"

	"paperlink/db/repo"

	"github.com/gin-gonic/gin"
)

const d4sThumbCacheDir = "./data/d4s/thumbs"
const d4sThumbDPI = "80"
const d4sThumbWebPQuality = "60"

func d4sThumbCachePath(uuid string) string {
	return path.Join(d4sThumbCacheDir, uuid+".webp")
}

func ensureD4SThumbnail(uuid string, pvfPath string) (string, error) {
	thumbPath := d4sThumbCachePath(uuid)
	if st, err := os.Stat(thumbPath); err == nil && !st.IsDir() {
		return thumbPath, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(d4sThumbCacheDir, 0o750); err != nil {
		return "", err
	}

	// Extract page 1 from the PVF into a temp PDF for Ghostscript
	pageData, err := pvf.ReadPage(pvfPath, 1)
	if err != nil {
		return "", fmt.Errorf("failed to read first pvf page: %w", err)
	}

	stamp := time.Now().UnixNano()
	tmpPDFPath := fmt.Sprintf("%s.%d.tmp.pdf", thumbPath, stamp)
	tmpPNGPath := fmt.Sprintf("%s.%d.tmp.png", thumbPath, stamp)
	tmpWebPPath := fmt.Sprintf("%s.%d.tmp.webp", thumbPath, stamp)
	defer os.Remove(tmpPDFPath)
	defer os.Remove(tmpPNGPath)
	defer os.Remove(tmpWebPPath)

	if err := os.WriteFile(tmpPDFPath, pageData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write temp pdf: %w", err)
	}

	gsCmd := exec.Command(
		"gs",
		"-sDEVICE=png16m",
		"-r"+d4sThumbDPI,
		"-dFirstPage=1",
		"-dLastPage=1",
		"-dTextAlphaBits=4",
		"-dGraphicsAlphaBits=4",
		"-dBATCH",
		"-dNOPAUSE",
		"-sOutputFile="+tmpPNGPath,
		tmpPDFPath, // temp PDF instead of the .pvf
	)
	out, err := gsCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ghostscript failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	webpCmd := exec.Command("cwebp", "-quiet", "-q", d4sThumbWebPQuality, tmpPNGPath, "-o", tmpWebPPath)
	out, err = webpCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cwebp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(tmpWebPPath, thumbPath); err != nil {
		return "", err
	}
	return thumbPath, nil
}

// GetThumbnail godoc
// @Summary      Get Digi4School thumbnail
// @Description  Returns the first thumbnail page as WebP for a Digi4School book.
// @Tags         digi4school
// @Produce      image/webp
// @Param        id   path      int  true  "Digi4School book ID"
// @Failure      400  {object}  routes.ErrorResponse "Invalid book ID"
// @Failure      401  {object}  routes.ErrorResponse "Unauthorized"
// @Failure      404  {object}  routes.ErrorResponse "Book not found"
// @Failure      500  {object}  routes.ErrorResponse "Failed to load thumbnail"
// @Router       /api/v1/d4s/thumbnail/{id} [get]
// @Security     BearerAuth
func GetThumbnail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid book id")
		return
	}

	book, err := repo.Digi4SchoolBook.Get(id)
	if err != nil || book == nil {
		c.String(http.StatusNotFound, "book not found")
		return
	}
	if book.FileUUID == "" {
		c.String(http.StatusNotFound, "book has no file")
		return
	}

	file := repo.FileDocument.GetByUUID(book.FileUUID)
	if file == nil {
		c.String(http.StatusNotFound, "file not found")
		return
	}

	if _, err := os.Stat(file.Path); err != nil {
		c.String(http.StatusNotFound, "pdf file missing on disk")
		return
	}

	thumbPath, err := ensureD4SThumbnail(book.FileUUID, file.Path)
	if err != nil {
		fmt.Printf("ensureD4SThumbnail error for uuid=%s path=%s: %v\n", book.FileUUID, file.Path, err)
		c.String(http.StatusInternalServerError, "failed to create thumbnail")
		return
	}

	data, err := os.ReadFile(thumbPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to read thumbnail")
		return
	}

	c.Data(http.StatusOK, "image/webp", data)
}
