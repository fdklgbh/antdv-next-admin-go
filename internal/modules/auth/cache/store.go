package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	basecache "antdv-next-admin-go/internal/infrastructure/cache"
	"antdv-next-admin-go/internal/modules/auth/model"
)

type StoreCache struct {
	store         basecache.Store
	sessionKey    func(string) string
	permissionKey func(int64) string
}

type permissionPayload struct {
	SchemaVersion int      `json:"schemaVersion"`
	Routes        []string `json:"routes"`
}

func NewStoreCache(store basecache.Store, sessionKey func(string) string, permissionKey func(int64) string) *StoreCache {
	return &StoreCache{store: store, sessionKey: sessionKey, permissionKey: permissionKey}
}

func (c *StoreCache) GetRefresh(ctx context.Context, token string) (model.RefreshSession, error) {
	var session model.RefreshSession
	payload, err := c.store.Get(ctx, c.sessionKey(token))
	if err != nil {
		return session, err
	}
	if err := json.Unmarshal(payload, &session); err != nil {
		return session, fmt.Errorf("decode refresh session: %w", err)
	}
	if session.Principal.UserID <= 0 || session.SessionID == "" || session.ExpiresAt.IsZero() {
		return session, fmt.Errorf("invalid refresh session")
	}
	return session, nil
}

func (c *StoreCache) SetRefresh(ctx context.Context, token string, session model.RefreshSession, ttl time.Duration) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode refresh session: %w", err)
	}
	return c.store.Set(ctx, c.sessionKey(token), payload, ttl)
}

func (c *StoreCache) DeleteRefresh(ctx context.Context, token string) error {
	return c.store.Delete(ctx, c.sessionKey(token))
}

func (c *StoreCache) GetRoutes(ctx context.Context, userID int64) ([]model.Route, error) {
	payload, err := c.store.Get(ctx, c.permissionKey(userID))
	if err != nil {
		return nil, err
	}
	var cached permissionPayload
	if err := json.Unmarshal(payload, &cached); err != nil {
		return nil, fmt.Errorf("decode permission cache: %w", err)
	}
	if cached.SchemaVersion != 1 || cached.Routes == nil {
		return nil, fmt.Errorf("invalid permission cache schema")
	}
	routes := make([]model.Route, 0, len(cached.Routes))
	for _, item := range cached.Routes {
		method, path, ok := strings.Cut(item, " ")
		if !ok || method == "" || path == "" {
			return nil, fmt.Errorf("invalid permission cache route")
		}
		routes = append(routes, model.Route{Method: method, Path: path})
	}
	return routes, nil
}

func (c *StoreCache) SetRoutes(ctx context.Context, userID int64, routes []model.Route, ttl time.Duration) error {
	if routes == nil {
		routes = []model.Route{}
	}
	items := make([]string, 0, len(routes))
	for _, route := range routes {
		items = append(items, route.Method+" "+route.Path)
	}
	payload, err := json.Marshal(permissionPayload{SchemaVersion: 1, Routes: items})
	if err != nil {
		return fmt.Errorf("encode permission cache: %w", err)
	}
	return c.store.Set(ctx, c.permissionKey(userID), payload, ttl)
}

func (c *StoreCache) DeleteUsers(ctx context.Context, userIDs ...int64) error {
	const batchSize = 500
	for start := 0; start < len(userIDs); start += batchSize {
		end := min(start+batchSize, len(userIDs))
		keys := make([]string, 0, end-start)
		for _, id := range userIDs[start:end] {
			keys = append(keys, c.permissionKey(id))
		}
		if err := c.store.Delete(ctx, keys...); err != nil {
			return err
		}
	}
	return nil
}
