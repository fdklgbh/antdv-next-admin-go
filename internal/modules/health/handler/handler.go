package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"antdv-next-admin-go/internal/infrastructure/logger"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type RedisPinger interface{ Ping(context.Context) error }
type Handler struct {
	db    *sql.DB
	redis RedisPinger
}

func New(db *sql.DB, redis RedisPinger) *Handler {
	return &Handler{db: db, redis: redis}
}
func (h *Handler) Live(c *gin.Context) { response.Success(c, gin.H{"status": "ok"}) }
func (h *Handler) Ready(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.db.PingContext(ctx); err != nil {
		h.unavailable(c, "database", err)
		return
	}
	if err := h.redis.Ping(ctx); err != nil {
		h.unavailable(c, "redis", err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
func (h *Handler) unavailable(c *gin.Context, dependency string, err error) {
	logger.FromContext(c.Request.Context()).Error("readiness_check_failed", logger.String("dependency", dependency), logger.Err(err))
	response.Error(c, apperrors.New(commoncode.ServiceUnavailable, http.StatusServiceUnavailable, "服务暂不可用，请稍后重试", fmt.Errorf("%s: %w", dependency, err)))
}
