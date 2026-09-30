package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-gin-template/internal/infrastructure/logger"
	commoncode "go-gin-template/internal/shared/errcode"
	apperrors "go-gin-template/internal/shared/errors"
	"go-gin-template/internal/shared/response"
)

type RedisPinger interface{ Ping(context.Context) error }
type Handler struct {
	db     *sql.DB
	redis  RedisPinger
	logger logger.Logger
}

func New(db *sql.DB, redis RedisPinger, log logger.Logger) *Handler {
	return &Handler{db: db, redis: redis, logger: log}
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
	h.logger.Error(c.Request.Context(), "readiness_check_failed", logger.String("dependency", dependency), logger.Err(err))
	response.Error(c, apperrors.New(commoncode.ServiceUnavailable, http.StatusServiceUnavailable, "service unavailable", fmt.Errorf("%s: %w", dependency, err)))
}
