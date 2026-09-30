package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go-gin-template/internal/config"
	"go-gin-template/internal/httpserver"
	basecache "go-gin-template/internal/infrastructure/cache"
	"go-gin-template/internal/infrastructure/database"
	"go-gin-template/internal/infrastructure/logger"
	redisinfra "go-gin-template/internal/infrastructure/redis"
	"go-gin-template/internal/modules/account"
	"go-gin-template/internal/modules/admin"
	"go-gin-template/internal/modules/auth"
	"go-gin-template/internal/modules/health"
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
	authModule := auth.NewModule(db, store, log, cfg.App.Name, cfg.Auth)
	return dependencies{routerDeps: httpserver.Dependencies{Logger: log, Auth: authModule, Account: account.NewModule(), Admin: admin.NewModule(), Health: health.NewModule(sqlDB, redisPinger{client: redisClient}, log)}, closers: closers}, nil
}
