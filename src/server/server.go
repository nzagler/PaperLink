package server

import (
	"mime"
	"os"
	"paperlink/server/routes/admin"
	"paperlink/server/routes/auth"
	"paperlink/server/routes/d4s"
	"paperlink/server/routes/directory"
	"paperlink/server/routes/document"
	"paperlink/server/routes/invite"
	"paperlink/server/routes/pdf"
	"paperlink/server/routes/pdfws"
	"paperlink/server/routes/structure"
	"paperlink/server/routes/task"
	"paperlink/util"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var log = util.GroupLog("SERVER")

func isCompressibleAsset(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".css", ".js":
		return true
	default:
		return false
	}
}

func frontendFilePath(requestPath string) string {
	return filepath.Join("./dist", strings.TrimPrefix(filepath.Clean("/"+requestPath), "/"))
}

func clientAcceptsBrotli(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept-Encoding"), "br")
}

func clientAcceptsGzip(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept-Encoding"), "gzip")
}

func serveFile(c *gin.Context, path string, allowCompressed bool) {
	c.Header("Vary", "Accept-Encoding")

	if allowCompressed {
		if clientAcceptsBrotli(c) {
			if _, err := os.Stat(path + ".br"); err == nil {
				c.Header("Content-Encoding", "br")
				c.Header("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
				c.File(path + ".br")
				return
			}
		}
		if clientAcceptsGzip(c) {
			if _, err := os.Stat(path + ".gz"); err == nil {
				c.Header("Content-Encoding", "gzip")
				c.Header("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
				c.File(path + ".gz")
				return
			}
		}
	}

	c.Writer.Header().Del("Content-Encoding")
	c.File(path)
}

func Start() {
	r := gin.New()

	r.GET("/assets/*filepath", func(c *gin.Context) {
		path := "./dist/assets" + c.Param("filepath")

		serveFile(c, path, isCompressibleAsset(path))
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		requestPath := c.Request.URL.Path
		if filepath.Ext(requestPath) != "" {
			path := frontendFilePath(requestPath)
			if _, err := os.Stat(path); err == nil {
				serveFile(c, path, isCompressibleAsset(path))
				return
			}

			c.Status(404)
			return
		}

		serveFile(c, "./dist/index.html", true)
	})

	auth.InitAuthRouter(r)
	admin.InitAdminRouter(r)
	pdf.InitPDFRouter(r)
	pdfws.InitPDFWSRouter(r)
	document.InitDocumentRouter(r)
	invite.InitInviteRouter(r)
	directory.InitDirectoryRouter(r)
	structure.InitStructureRoutes(r)
	d4s.InitDigi4SchoolRouter(r)
	task.InitTasksTasks(r)
	log.Info("starting server at port 8080")
	err := r.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
