package system

import (
	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
)

func (m *Module) RegisterAudit(router *gin.Engine) { router.Use(m.handler.Audit()) }
func (m *Module) RegisterRoutes(private *gin.RouterGroup, authorizer middleware.Authorizer) {
	moduleRoutes := private.Group("", middleware.RequestLogger(m.logger))
	g := moduleRoutes.Group("", middleware.RequirePermission(authorizer))
	g.GET("/config/list", m.handler.List("config", false))
	g.GET("/config/key/:key", m.handler.ConfigByKey)
	g.POST("/config", m.handler.Save("config"))
	g.PUT("/config/:id", m.handler.Save("config"))
	g.DELETE("/config/:id", m.handler.Delete("config"))
	g.GET("/dict/types", m.handler.List("dict_type", true))
	g.GET("/dict/type/list", m.handler.List("dict_type", false))
	g.POST("/dict/type", m.handler.Save("dict_type"))
	g.PUT("/dict/type/:id", m.handler.Save("dict_type"))
	g.DELETE("/dict/type/:id", m.handler.Delete("dict_type"))
	moduleRoutes.GET("/dict/data/all", m.handler.EnabledDictionaries)
	g.GET("/dict/data/list", m.handler.List("dict_data", false))
	g.GET("/dict/data/:typeCode", m.handler.List("dict_data", true))
	g.POST("/dict/data", m.handler.Save("dict_data"))
	g.PUT("/dict/data/:id", m.handler.Save("dict_data"))
	g.DELETE("/dict/data/:id", m.handler.Delete("dict_data"))
	g.GET("/file/list", m.handler.List("file", false))
	g.POST("/file/upload", m.handler.Upload)
	g.GET("/file/:id/download", m.handler.Download)
	g.DELETE("/file/:id", m.handler.Delete("file"))
	g.GET("/log/operation/list", m.handler.List("operation", false))
	g.GET("/log/login/list", m.handler.List("login", false))
	g.DELETE("/log/operation/clear", m.handler.Clear("operation"))
	g.DELETE("/log/login/clear", m.handler.Clear("login"))
}
