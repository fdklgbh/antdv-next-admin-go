package handler

import (
	"antdv-next-admin-go/internal/config"
	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/auth/dto"
	authcode "antdv-next-admin-go/internal/modules/auth/errcode"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/modules/auth/service"
	"antdv-next-admin-go/internal/shared/contextx"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

const refreshCookie = "refresh_token"

type Handler struct {
	service *service.Service
	config  config.Auth
}

func New(s *service.Service, cfg config.Auth) *Handler {
	return &Handler{service: s, config: cfg}
}
func (h *Handler) allowed(c *gin.Context) bool {
	origin := c.GetHeader("Origin")
	if origin == "" && c.GetHeader("Sec-Fetch-Site") != "cross-site" {
		return true
	}
	for _, allowed := range h.config.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	response.Error(c, apperrors.New(authcode.OriginDenied, http.StatusForbidden, "请求来源不被允许", nil))
	return false
}
func cookie(c *gin.Context) string {
	value, _ := c.Cookie(refreshCookie)
	if len(value) > 512 {
		return ""
	}
	return value
}
func (h *Handler) fail(c *gin.Context, err error) {
	if errors.Is(err, model.ErrUnauthenticated) {
		response.Error(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "登录状态已失效，请重新登录", nil))
		return
	}
	logger.FromContext(c.Request.Context()).Error("auth_request_failed", logger.Err(err))
	response.Error(c, apperrors.New(authcode.ServiceUnavailable, http.StatusServiceUnavailable, "认证或权限服务暂不可用，请稍后重试", nil))
}
func (h *Handler) Login(c *gin.Context) {
	if !h.allowed(c) {
		return
	}
	var input dto.LoginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, nil)
		return
	}
	c.Set("audit.username", input.Username)
	result, refresh, expires, err := h.service.Login(c.Request.Context(), input, cookie(c))
	if errors.Is(err, model.ErrUnauthenticated) {
		response.Error(c, apperrors.New(authcode.InvalidCredentials, http.StatusUnauthorized, "用户名或密码错误，或账号不可用", nil))
		return
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	value := &http.Cookie{Name: refreshCookie, Value: refresh, Path: "/api/auth", HttpOnly: true, Secure: *h.config.CookieSecure, SameSite: http.SameSiteLaxMode}
	if input.Remember {
		value.MaxAge = int(time.Until(expires).Seconds())
		value.Expires = expires
	}
	http.SetCookie(c.Writer, value)
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}
func (h *Handler) Refresh(c *gin.Context) {
	if !h.allowed(c) {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), cookie(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}
func (h *Handler) Logout(c *gin.Context) {
	if !h.allowed(c) {
		return
	}
	if err := h.service.Logout(c.Request.Context(), cookie(c)); err != nil {
		h.fail(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: refreshCookie, Path: "/api/auth", HttpOnly: true, Secure: *h.config.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	response.Success(c, nil)
}
func (h *Handler) Info(c *gin.Context) {
	id, _ := contextx.UserID(c.Request.Context())
	info, err := h.service.UserInfo(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, info)
}
