package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"antdv-next-admin-go/internal/infrastructure/logger"
	authcode "antdv-next-admin-go/internal/modules/auth/errcode"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/shared/contextx"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const principalKey = "auth.principal"
const maxBearerTokenLength = 4096

type Authenticator interface {
	Authenticate(context.Context, string) (model.Principal, error)
}
type Authorizer interface {
	Authorize(context.Context, int64, string, string) (bool, error)
}

func RequireLogin(service Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "登录状态已失效，请重新登录", nil))
			return
		}
		principal, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, model.ErrUnauthenticated) {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "登录状态已失效，请重新登录", err))
			return
		}
		if err != nil {
			logger.FromContext(c.Request.Context()).Error("authentication_failed", logger.Err(err))
			response.Abort(c, apperrors.New(authcode.ServiceUnavailable, http.StatusServiceUnavailable, "认证或权限服务暂不可用，请稍后重试", err))
			return
		}
		c.Set(principalKey, principal)
		c.Request = c.Request.WithContext(contextx.WithUserID(c.Request.Context(), principal.UserID))
		c.Next()
	}
}
func RequirePermission(service Authorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentPrincipal(c)
		if !ok {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "登录状态已失效，请重新登录", nil))
			return
		}
		allowed, err := service.Authorize(c.Request.Context(), principal.UserID, c.Request.Method, c.FullPath())
		if err != nil {
			logger.FromContext(c.Request.Context()).Error("authorization_failed", logger.String("method", c.Request.Method), logger.String("route", c.FullPath()), logger.Err(err))
			response.Abort(c, apperrors.New(authcode.ServiceUnavailable, http.StatusServiceUnavailable, "认证或权限服务暂不可用，请稍后重试", err))
			return
		}
		if !allowed {
			response.Abort(c, apperrors.New(authcode.PermissionDenied, http.StatusForbidden, "没有访问权限", nil))
			return
		}
		c.Next()
	}
}
func CurrentPrincipal(c *gin.Context) (model.Principal, bool) {
	value, exists := c.Get(principalKey)
	if !exists {
		return model.Principal{}, false
	}
	principal, ok := value.(model.Principal)
	return principal, ok
}
func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" || len(parts[1]) > maxBearerTokenLength {
		return "", false
	}
	return parts[1], true
}
