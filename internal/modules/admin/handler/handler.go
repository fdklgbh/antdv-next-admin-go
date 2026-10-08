package handler

import (
	"antdv-next-admin-go/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{}

func New() *Handler                    { return &Handler{} }
func (h *Handler) Ping(c *gin.Context) { response.Success(c, gin.H{"message": "pong"}) }
