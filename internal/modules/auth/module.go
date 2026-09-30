package auth

import (
	"go-gin-template/internal/config"
	basecache "go-gin-template/internal/infrastructure/cache"
	"go-gin-template/internal/infrastructure/logger"
	authcache "go-gin-template/internal/modules/auth/cache"
	"go-gin-template/internal/modules/auth/repository"
	"go-gin-template/internal/modules/auth/service"
	"gorm.io/gorm"
)

type Module struct{ Service *service.Service }

func NewModule(db *gorm.DB, store basecache.Store, log logger.Logger, appName string, cfg config.Auth) *Module {
	repo := repository.New(db)
	cache := authcache.NewRedis(store, func(token string) string { return sessionKey(appName, token) }, func(userID int64) string { return permissionRoutesKey(appName, userID) })
	return &Module{Service: service.New(repo, cache, cache, cfg.PermissionCacheTTL, log.With(logger.String("module", "auth")))}
}
