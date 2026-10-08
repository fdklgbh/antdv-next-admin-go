package app

import (
	"context"
	"errors"
	"fmt"

	"antdv-next-admin-go/internal/config"
	"antdv-next-admin-go/internal/httpserver"
	basecache "antdv-next-admin-go/internal/infrastructure/cache"
	"antdv-next-admin-go/internal/infrastructure/database"
	"antdv-next-admin-go/internal/infrastructure/logger"
	redisinfra "antdv-next-admin-go/internal/infrastructure/redis"
	"antdv-next-admin-go/internal/modules/account"
	"antdv-next-admin-go/internal/modules/admin"
	"antdv-next-admin-go/internal/modules/auth"
	"antdv-next-admin-go/internal/modules/health"
	"antdv-next-admin-go/internal/modules/system"
	"github.com/redis/go-redis/v9"
)

type redisPinger struct{ client *redis.Client }

func (p redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }

type dependencies struct {
	routerDeps httpserver.Dependencies
	closers    []func() error
}

func buildDependencies(ctx context.Context, cfg config.Config) (_ dependencies, err error) {
	log, closeLog, err := logger.New(cfg.Log, cfg.App.Name, cfg.App.Env, cfg.App.Version)
	if err != nil {
		return dependencies{}, err
	}
	closers := []func() error{closeLog}
	defer func() {
		if err != nil {
			for index := len(closers) - 1; index >= 0; index-- {
				err = errors.Join(err, closers[index]())
			}
		}
	}()
	db, closeDB, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return dependencies{}, err
	}
	closers = append(closers, closeDB)
	redisClient, closeRedis, err := redisinfra.Open(ctx, cfg.Redis)
	if err != nil {
		return dependencies{}, err
	}
	closers = append(closers, closeRedis)
	sqlDB, err := db.DB()
	if err != nil {
		return dependencies{}, fmt.Errorf("get database connection pool: %w", err)
	}
	store := basecache.NewRedisStore(redisClient)
	authModule := auth.NewModule(db, store, log, cfg.Redis.KeyPrefix, cfg.Auth)
	if err := initializeSuperAdmin(logger.WithLogger(ctx, authModule.Logger()), authModule, cfg.Auth.SuperAdminUsername); err != nil {
		return dependencies{}, err
	}
	return dependencies{routerDeps: httpserver.Dependencies{System: system.NewModule(db, log), Logger: log, Auth: authModule, Account: account.NewModule(), Admin: admin.NewModule(), Health: health.NewModule(sqlDB, redisPinger{client: redisClient}, log)}, closers: closers}, nil
}
