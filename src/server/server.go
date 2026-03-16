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
	"paperlink/server/routes/structure"
	"paperlink/server/routes/task"
	"paperlink/util"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var log = util.GroupLog("SERVER")

func isBrotliCompressedAsset(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".css", ".js":
		return true
	default:
		return false
	}
}

func Start() {
	r := gin.New()

	r.GET("/assets/*filepath", func(c *gin.Context) {
		path := "./dist/assets" + c.Param("filepath")

		if isBrotliCompressedAsset(path) && strings.Contains(c.GetHeader("Accept-Encoding"), "br") {
			if _, err := os.Stat(path); err == nil {
				c.Header("Content-Encoding", "br")
				c.Header("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
				c.File(path)
				return
			}
		}

		c.File(path)
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}

		if strings.Contains(c.GetHeader("Accept-Encoding"), "br") {
			if _, err := os.Stat("./dist/index.html"); err == nil {
				c.Header("Content-Encoding", "br")
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.File("./dist/index.html")
				return
			}
		}

		c.File("./dist/index.html")
	})

	auth.InitAuthRouter(r)
	admin.InitAdminRouter(r)
	pdf.InitPDFRouter(r)
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
