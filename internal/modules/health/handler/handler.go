package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/infrastructure/logger"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
)

type CachePinger interface{ Ping(context.Context) error }
type Handler struct {
	db          *sql.DB
	cachePinger CachePinger
}

func New(db *sql.DB, cachePinger CachePinger) *Handler {
	return &Handler{db: db, cachePinger: cachePinger}
}
func (h *Handler) Live(c *gin.Context) { response.Success(c, gin.H{"status": "ok"}) }
func (h *Handler) Ready(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.db.PingContext(ctx); err != nil {
		h.unavailable(c, "database", err)
		return
	}
	if h.cachePinger != nil {
		if err := h.cachePinger.Ping(ctx); err != nil {
			h.unavailable(c, "cache", err)
			return
		}
	}
	response.Success(c, gin.H{"status": "ok"})
}
func (h *Handler) unavailable(c *gin.Context, dependency string, err error) {
	logger.FromContext(c.Request.Context()).Error("readiness_check_failed", logger.String("dependency", dependency), logger.Err(err))
	response.Error(c, apperrors.New(commoncode.ServiceUnavailable, http.StatusServiceUnavailable, "服务暂不可用，请稍后重试", fmt.Errorf("%s: %w", dependency, err)))
}
