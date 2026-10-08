package auth

import (
	"antdv-next-admin-go/internal/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

func (m *Module) RegisterManagementRoutes(private *gin.RouterGroup) {
	private.POST("/users/change-password", m.handler.ChangePassword)
	private.GET("/permissions/user", m.handler.UserPermissions)
	group := private.Group("", middleware.RequirePermission(m.Service))
	group.GET("/users", m.handler.Users)
	group.GET("/users/:id", m.handler.User)
	group.POST("/users", m.handler.SaveUser)
	group.PUT("/users/:id", m.handler.SaveUser)
	group.DELETE("/users/:id", m.handler.DeleteUser)
	group.GET("/roles/options", m.handler.Roles)
	group.GET("/roles", m.handler.Roles)
	group.GET("/roles/:id", m.handler.Role)
	group.POST("/roles", m.handler.SaveRole)
	group.PUT("/roles/:id", m.handler.SaveRole)
	group.DELETE("/roles/:id", m.handler.DeleteRole)
	group.GET("/permissions", m.handler.Permissions)
	group.GET("/permissions/tree", m.handler.Permissions)
	group.GET("/permissions/:id", m.handler.Permission)
	group.POST("/permissions", m.handler.SavePermission)
	group.PUT("/permissions/:id", m.handler.SavePermission)
	group.DELETE("/permissions/:id", m.handler.DeletePermission)
}
