package pdf

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"paperlink/db/repo"
	"paperlink/pvf"
)

var unsafeFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]+`)

// Download godoc
// @Summary      Download PDF document
// @Description  Exports the stored document as a downloadable PDF.
// @Tags         pdf
// @Param        id path string true "Document ID"
// @Produce      application/pdf
// @Success      200 {file} file
// @Failure      403 {string} string "forbidden"
// @Failure      404 {string} string "document not found"
// @Failure      500 {string} string "failed to export pdf"
// @Router       /pdf/{id}/download [get]
// @Security     BearerAuth
func Download(c *gin.Context) {
	docUUID := c.Param("id")
	userID := c.GetInt("userId")

	doc := repo.Document.GetByUUIDWithFile(docUUID)
	if doc == nil {
		c.String(http.StatusNotFound, "document not found")
		return
	}

	if doc.UserID != userID && !repo.DocumentUser.HasAccess(doc.ID, userID) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	downloadPath := doc.File.Path
	cleanup := func() {}
	if strings.ToLower(filepath.Ext(downloadPath)) != ".pdf" {
		var err error
		downloadPath, cleanup, err = exportPVFToPDF(doc.File.Path)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to export pdf")
			return
		}
	}
	defer cleanup()

	if _, err := os.Stat(downloadPath); err != nil {
		c.String(http.StatusNotFound, "document file not found")
		return
	}

	c.FileAttachment(downloadPath, downloadFilename(doc.Name))
}

func downloadFilename(name string) string {
	base := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
	if base == "" {
		base = "document"
	}
	base = unsafeFilenameChars.ReplaceAllString(base, "_")
	return base + ".pdf"
}

func exportPVFToPDF(filePath string) (string, func(), error) {
	metadata, err := pvf.ReadMetadata(filePath)
	if err != nil {
		return "", func() {}, err
	}

	tmpDir, err := os.MkdirTemp("", "paperlink_export_*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	pageFiles := make([]string, 0, metadata.PageCount)
	for page := uint64(1); page <= metadata.PageCount; page++ {
		data, err := pvf.ReadPage(filePath, page)
		if err != nil {
			cleanup()
			return "", func() {}, err
		}

		pagePath := filepath.Join(tmpDir, fmt.Sprintf("page_%06d.pdf", page))
		if err := os.WriteFile(pagePath, data, 0600); err != nil {
			cleanup()
			return "", func() {}, err
		}
		pageFiles = append(pageFiles, pagePath)
	}

	outputPath := filepath.Join(tmpDir, "document.pdf")
	if err := api.MergeCreateFile(pageFiles, outputPath, false, nil); err != nil {
		cleanup()
		return "", func() {}, err
	}

	return outputPath, cleanup, nil
}
