package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisStore struct{ client *redis.Client }

func NewRedisStore(client *redis.Client) *RedisStore { return &RedisStore{client: client} }
func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	if err != nil {
		return nil, fmt.Errorf("get redis cache: %w", err)
	}
	return value, nil
}
func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set redis cache: %w", err)
	}
	return nil
}
func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete redis cache: %w", err)
	}
	return nil
}
