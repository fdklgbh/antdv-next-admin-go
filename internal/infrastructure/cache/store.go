package cache

import (
	"context"
	"time"
)

// KeepTTL preserves an existing entry's expiration; missing entries do not expire.
const KeepTTL time.Duration = -1

type Store interface {
	Get(context.Context, string) ([]byte, error)
	// Set uses ttl == 0 for a non-expiring entry and KeepTTL to preserve its expiration.
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, ...string) error
}
