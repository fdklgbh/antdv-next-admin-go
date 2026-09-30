package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go-gin-template/internal/infrastructure/logger"
	"go-gin-template/internal/shared/contextx"
	commoncode "go-gin-template/internal/shared/errcode"
	apperrors "go-gin-template/internal/shared/errors"
	"go-gin-template/internal/shared/response"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(requestID) {
			var err error
			requestID, err = newRequestID()
			if err != nil {
				response.Abort(c, apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "internal server error", err))
				return
			}
		}
		c.Set(response.RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Request = c.Request.WithContext(contextx.WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}
func AccessLog(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		status := c.Writer.Status()
		fields := []logger.Field{logger.String("method", c.Request.Method), logger.String("route", c.FullPath()), logger.Int("status", status), logger.Int64("latency_ms", time.Since(startedAt).Milliseconds()), logger.String("client_ip", c.ClientIP()), logger.String("user_agent", c.Request.UserAgent())}
		if status >= 500 {
			log.Error(c.Request.Context(), "http_request", fields...)
		} else if status >= 400 {
			log.Warn(c.Request.Context(), "http_request", fields...)
		} else {
			log.Info(c.Request.Context(), "http_request", fields...)
		}
	}
}
func Recovery(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(c.Request.Context(), "http_panic", logger.Any("panic", recovered), logger.String("stack", string(debug.Stack())))
				response.Abort(c, apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "internal server error", nil))
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
