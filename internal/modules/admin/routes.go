package admin

import (
	"github.com/gin-gonic/gin"
	"go-gin-template/internal/httpserver/middleware"
	"go-gin-template/internal/infrastructure/logger"
)

func (m *Module) RegisterRoutes(router *gin.RouterGroup, authorizer middleware.Authorizer, log logger.Logger) {
	group := router.Group("/admin", middleware.RequirePermission(authorizer, log))
	group.GET("/ping", m.Handler.Ping)
}
