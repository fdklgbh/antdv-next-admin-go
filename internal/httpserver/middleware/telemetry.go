package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/shared/contextx"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(requestID) {
			var err error
			requestID, err = newRequestID()
			if err != nil {
				response.Abort(c, apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "服务器内部错误，请稍后重试", err))
				return
			}
		}
		c.Set(response.RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		ctx := contextx.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(logger.WithLogger(ctx, log))
		c.Next()
	}
}

func RequestLogger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := logger.WithLogger(c.Request.Context(), log)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		status := c.Writer.Status()
		fields := []logger.Field{logger.String("method", c.Request.Method), logger.String("route", c.FullPath()), logger.Int("status", status), logger.Int64("latency_ms", time.Since(startedAt).Milliseconds()), logger.String("client_ip", c.ClientIP()), logger.String("user_agent", c.Request.UserAgent())}
		if status >= 500 {
			logger.FromContext(c.Request.Context()).Error("http_request", fields...)
		} else if status >= 400 {
			logger.FromContext(c.Request.Context()).Warn("http_request", fields...)
		} else {
			logger.FromContext(c.Request.Context()).Info("http_request", fields...)
		}
	}
}
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.FromContext(c.Request.Context()).Error("http_panic", logger.Any("panic", recovered), logger.String("stack", string(debug.Stack())))
				response.Abort(c, apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "服务器内部错误，请稍后重试", nil))
			}
		}()
		c.Next()
	}
}
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
func newRequestID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
