package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-gin-template/internal/httpserver/middleware"
	"go-gin-template/internal/infrastructure/logger"
	"go-gin-template/internal/modules/account"
	"go-gin-template/internal/modules/admin"
	"go-gin-template/internal/modules/auth"
	"go-gin-template/internal/modules/health"
	commoncode "go-gin-template/internal/shared/errcode"
	apperrors "go-gin-template/internal/shared/errors"
	"go-gin-template/internal/shared/response"
)

type Dependencies struct {
	Logger  logger.Logger
	Auth    *auth.Module
	Account *account.Module
	Admin   *admin.Module
	Health  *health.Module
}

func NewRouter(mode string, deps Dependencies) (*gin.Engine, error) {
	gin.SetMode(mode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	router.Use(middleware.RequestID(), middleware.AccessLog(deps.Logger), middleware.Recovery(deps.Logger), middleware.SecurityHeaders())
	router.NoRoute(func(c *gin.Context) {
		response.Error(c, apperrors.New(commoncode.NotFound, http.StatusNotFound, "resource not found", nil))
	})
	router.NoMethod(func(c *gin.Context) {
		response.Error(c, apperrors.New(commoncode.MethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil))
	})
	deps.Health.RegisterRoutes(router)
	private := router.Group("/api", middleware.RequireLogin(deps.Auth.Service, deps.Logger))
	deps.Auth.RegisterRoutes(private)
	deps.Account.RegisterRoutes(private)
	deps.Admin.RegisterRoutes(private, deps.Auth.Service, deps.Logger)
	return router, nil
}
