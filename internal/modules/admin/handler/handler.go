package handler

import (
	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/shared/response"
)

type Handler struct{}

func New() *Handler                    { return &Handler{} }
func (h *Handler) Ping(c *gin.Context) { response.Success(c, gin.H{"message": "pong"}) }
