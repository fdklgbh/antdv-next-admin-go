package auth

import (
	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
)

func (m *Module) RegisterPublicRoutes(group *gin.RouterGroup) {
	group = group.Group("", middleware.RequestLogger(m.logger))
	group.POST("/auth/login", m.handler.Login)
	group.POST("/auth/refresh", m.handler.Refresh)
	group.POST("/auth/logout", m.handler.Logout)
}
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/auth/info", m.handler.Info)
}
