package health

import (
	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
)

func (m *Module) RegisterRoutes(router *gin.Engine) {
	group := router.Group("", middleware.RequestLogger(m.logger))
	group.GET("/health/live", m.Handler.Live)
	group.GET("/health/ready", m.Handler.Ready)
}
