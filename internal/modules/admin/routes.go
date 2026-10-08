package admin

import (
	"antdv-next-admin-go/internal/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterRoutes(router *gin.RouterGroup, authorizer middleware.Authorizer) {
	group := router.Group("/admin", middleware.RequirePermission(authorizer))
	group.GET("/ping", m.Handler.Ping)
}
