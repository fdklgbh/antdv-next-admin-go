package cache

import (
	"context"
	"time"
)

type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, ...string) error
}
