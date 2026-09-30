package redis

import (
	"context"
	"fmt"
	"go-gin-template/internal/config"

	redisclient "github.com/redis/go-redis/v9"
)

func Open(ctx context.Context, cfg config.Redis) (*redisclient.Client, func() error, error) {
	client := redisclient.NewClient(&redisclient.Options{Addr: cfg.Address, Username: cfg.Username, Password: cfg.Password, DB: cfg.Database, PoolSize: cfg.PoolSize})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, client.Close, nil
}
