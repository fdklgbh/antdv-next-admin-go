package cache

import (
	"context"
	"sync"
	"time"
)

const maxMemoryEntries = 100_000

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: make(map[string]memoryEntry)}
}

func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil, ErrMiss
	}
	if !entry.expiresAt.IsZero() && !time.Now().Before(entry.expiresAt) {
		delete(s.entries, key)
		return nil, ErrMiss
	}
	value := make([]byte, len(entry.value))
	copy(value, entry.value)
	return value, nil
}

func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl < 0 && ttl != KeepTTL {
		return ErrInvalidTTL
	}
	stored := make([]byte, len(value))
	copy(stored, value)
	s.mu.Lock()
	defer s.mu.Unlock()
	var expiresAt time.Time
	if ttl == KeepTTL {
		if existing, exists := s.entries[key]; exists && (existing.expiresAt.IsZero() || time.Now().Before(existing.expiresAt)) {
			expiresAt = existing.expiresAt
		}
	} else if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	if _, exists := s.entries[key]; !exists && len(s.entries) >= maxMemoryEntries {
		s.removeExpired(time.Now())
		if len(s.entries) >= maxMemoryEntries {
			return ErrFull
		}
	}
	s.entries[key] = memoryEntry{value: stored, expiresAt: expiresAt}
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, keys ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.entries, key)
	}
	return nil
}

func (s *MemoryStore) removeExpired(now time.Time) {
	for key, entry := range s.entries {
		if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}

var _ Store = (*MemoryStore)(nil)
