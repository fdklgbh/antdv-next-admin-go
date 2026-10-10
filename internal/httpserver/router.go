package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/account"
	"antdv-next-admin-go/internal/modules/admin"
	"antdv-next-admin-go/internal/modules/auth"
	"antdv-next-admin-go/internal/modules/health"
	"antdv-next-admin-go/internal/modules/system"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
)

type Dependencies struct {
	System  *system.Module
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
	router.Use(middleware.RequestID(deps.Logger), middleware.AccessLog(), middleware.Recovery(), middleware.SecurityHeaders())
	router.NoRoute(func(c *gin.Context) {
		response.Error(c, apperrors.New(commoncode.NotFound, http.StatusNotFound, "资源不存在", nil))
	})
	router.NoMethod(func(c *gin.Context) {
		response.Error(c, apperrors.New(commoncode.MethodNotAllowed, http.StatusMethodNotAllowed, "不支持的请求方法", nil))
	})
	deps.System.RegisterAudit(router)
	deps.Health.RegisterRoutes(router)
	deps.Auth.RegisterPublicRoutes(router.Group("/api"))
	private := router.Group("/api", middleware.RequestLogger(deps.Auth.Logger()), middleware.RequireLogin(deps.Auth.Service))
	deps.Auth.RegisterRoutes(private)
	deps.Auth.RegisterManagementRoutes(private)
	deps.System.RegisterRoutes(private, deps.Auth.Service)
	deps.Account.RegisterRoutes(private)
	deps.Admin.RegisterRoutes(private, deps.Auth.Service)
	return router, nil
}
