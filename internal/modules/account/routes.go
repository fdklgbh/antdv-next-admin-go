package account

import "github.com/gin-gonic/gin"

func (m *Module) RegisterRoutes(router *gin.RouterGroup) { router.GET("/me", m.Handler.Me) }
