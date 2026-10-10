package auth

import (
	"gorm.io/gorm"

	"antdv-next-admin-go/internal/config"
	basecache "antdv-next-admin-go/internal/infrastructure/cache"
	"antdv-next-admin-go/internal/infrastructure/logger"
	authcache "antdv-next-admin-go/internal/modules/auth/cache"
	"antdv-next-admin-go/internal/modules/auth/handler"
	"antdv-next-admin-go/internal/modules/auth/repository"
	"antdv-next-admin-go/internal/modules/auth/service"
)

type Module struct {
	Service *service.Service
	handler *handler.Handler
	logger  logger.Logger
}

func NewModule(db *gorm.DB, store basecache.Store, log logger.Logger, redisKeyPrefix string, cfg config.Auth) *Module {
	repo := repository.New(db)
	cache := authcache.NewStoreCache(store, func(token string) string { return sessionKey(redisKeyPrefix, token) }, func(userID int64) string { return permissionRoutesKey(redisKeyPrefix, userID) })
	authLog := log.With(logger.String("module", "auth"))
	svc := service.New(repo, cache, cache, cfg)
	return &Module{Service: svc, handler: handler.New(svc, cfg), logger: authLog}
}

func (m *Module) Logger() logger.Logger { return m.logger }
