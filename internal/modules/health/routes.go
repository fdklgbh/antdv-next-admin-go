package health

import "github.com/gin-gonic/gin"

func (m *Module) RegisterRoutes(router gin.IRoutes) {
	router.GET("/health/live", m.Handler.Live)
	router.GET("/health/ready", m.Handler.Ready)
}
