package handler

import (
	"antdv-next-admin-go/internal/httpserver/middleware"
	"antdv-next-admin-go/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{}

func New() *Handler { return &Handler{} }
func (h *Handler) Me(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	response.Success(c, principal)
}
