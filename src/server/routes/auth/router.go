package auth

import (
	"paperlink/server/middleware"
	"paperlink/util"

	"github.com/gin-gonic/gin"
)

var log = util.GroupLog("AUTH")

func InitAuthRouter(r *gin.Engine) {
	group := r.Group("/api/v1/auth")
	group.POST("/register", Register)
	group.POST("/login", Login)
	group.POST("/refresh", Refresh)
	group.POST("/logout", Logout)
	group.GET("/me", middleware.Auth, Me)
	group.GET("/hasAdmin", middleware.Auth, middleware.Admin, HasAdmin)
	group.GET("/oidc/status", OIDCStatus)
	group.GET("/oidc/config", middleware.Auth, GetOIDCConfig)
	group.PUT("/oidc/config", middleware.Auth, SaveOIDCConfig)
	group.DELETE("/oidc/identity", middleware.Auth, DisconnectOIDC)
	group.GET("/oidc/start", OIDCStart)
	group.GET("/oidc/callback", OIDCCallback)
}
