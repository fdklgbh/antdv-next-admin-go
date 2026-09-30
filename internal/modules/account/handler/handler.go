package handler

import (
	"github.com/gin-gonic/gin"
	"go-gin-template/internal/httpserver/middleware"
	"go-gin-template/internal/shared/response"
)

type Handler struct{}

func New() *Handler { return &Handler{} }
func (h *Handler) Me(c *gin.Context) {
	principal, _ := middleware.CurrentPrincipal(c)
	response.Success(c, principal)
}
