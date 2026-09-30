package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go-gin-template/internal/infrastructure/logger"
	authcode "go-gin-template/internal/modules/auth/errcode"
	"go-gin-template/internal/modules/auth/model"
	"go-gin-template/internal/shared/contextx"
	apperrors "go-gin-template/internal/shared/errors"
	"go-gin-template/internal/shared/response"
)

const principalKey = "auth.principal"
const maxBearerTokenLength = 4096

type Authenticator interface {
	Authenticate(context.Context, string) (model.Principal, error)
}
type Authorizer interface {
	Authorize(context.Context, int64, string, string) (bool, error)
}

func RequireLogin(service Authenticator, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "authentication required", nil))
			return
		}
		principal, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, model.ErrUnauthenticated) {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "authentication required", err))
			return
		}
		if err != nil {
			log.Error(c.Request.Context(), "authentication_failed", logger.Err(err))
			response.Abort(c, apperrors.New(authcode.ServiceUnavailable, http.StatusServiceUnavailable, "authentication service unavailable", err))
			return
		}
		c.Set(principalKey, principal)
		c.Request = c.Request.WithContext(contextx.WithUserID(c.Request.Context(), principal.UserID))
		c.Next()
	}
}
func RequirePermission(service Authorizer, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentPrincipal(c)
		if !ok {
			response.Abort(c, apperrors.New(authcode.Unauthenticated, http.StatusUnauthorized, "authentication required", nil))
			return
		}
		allowed, err := service.Authorize(c.Request.Context(), principal.UserID, c.Request.Method, c.FullPath())
		if err != nil {
			log.Error(c.Request.Context(), "authorization_failed", logger.String("method", c.Request.Method), logger.String("route", c.FullPath()), logger.Err(err))
			response.Abort(c, apperrors.New(authcode.ServiceUnavailable, http.StatusServiceUnavailable, "authorization service unavailable", err))
			return
		}
		if !allowed {
			response.Abort(c, apperrors.New(authcode.PermissionDenied, http.StatusForbidden, "permission denied", nil))
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
