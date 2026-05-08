package document

import (
	"paperlink/server/middleware"
	"paperlink/util"

	"github.com/gin-gonic/gin"
)

var log = util.GroupLog("DOCUMENT")

func InitDocumentRouter(r *gin.Engine) {
	group := r.Group("/api/v1/document")
	group.Use(middleware.Auth)
	group.GET("/filter", Filter)
	group.POST("/update", Update)
	group.POST("/create", Create)
	group.POST("/upload", Upload)
	group.GET("/invites", ListInvites)
	group.GET("/share/users", SearchUsers)
	group.GET("/get/:id", Get)
	group.DELETE("/delete/:id", Delete)
	group.GET("/:id/shares", ListShares)
	group.POST("/:id/share", Share)
	group.POST("/:id/invite/accept", AcceptInvite)
	group.POST("/:id/invite/decline", DeclineInvite)
	group.DELETE("/:id/share/me", RemoveOwnShare)
	group.DELETE("/:id/share/:userId", Unshare)
}
