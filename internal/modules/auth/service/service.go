package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	basecache "go-gin-template/internal/infrastructure/cache"
	"go-gin-template/internal/infrastructure/logger"
	authcache "go-gin-template/internal/modules/auth/cache"
	"go-gin-template/internal/modules/auth/model"
	"go-gin-template/internal/modules/auth/repository"
)

type Service struct {
	repo        *repository.Repository
	sessions    authcache.SessionCache
	permissions authcache.PermissionCache
	ttl         time.Duration
	logger      logger.Logger
}

func New(repo *repository.Repository, sessions authcache.SessionCache, permissions authcache.PermissionCache, ttl time.Duration, log logger.Logger) *Service {
	return &Service{repo: repo, sessions: sessions, permissions: permissions, ttl: ttl, logger: log}
}
func (s *Service) Authenticate(ctx context.Context, token string) (model.Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.Principal{}, model.ErrUnauthenticated
	}
	principal, err := s.sessions.GetPrincipal(ctx, token)
	if errors.Is(err, basecache.ErrMiss) {
		return model.Principal{}, model.ErrUnauthenticated
	}
	if err != nil {
		return model.Principal{}, fmt.Errorf("get session: %w", err)
	}
	return principal, nil
}
func (s *Service) Authorize(ctx context.Context, userID int64, method, route string) (bool, error) {
	if userID <= 0 || method == "" || route == "" {
		return false, fmt.Errorf("user ID, method, and route are required")
	}
	routes, err := s.permissions.GetRoutes(ctx, userID)
	if errors.Is(err, basecache.ErrMiss) {
		routes, err = s.repo.PermissionRoutes(ctx, userID)
		if err != nil {
			return false, err
		}
		if err := s.permissions.SetRoutes(ctx, userID, routes, s.ttl); err != nil {
			return false, fmt.Errorf("fill permission cache: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("read permission cache: %w", err)
	}
	for _, item := range routes {
		if item.Method == method && item.Path == route {
			return true, nil
		}
	}
	return false, nil
}
func (s *Service) Grant(ctx context.Context, userID int64, code string) error {
	if err := s.repo.Grant(ctx, userID, code); err != nil {
		return err
	}
	return s.invalidate(ctx, "grant_permission", userID)
}
func (s *Service) Revoke(ctx context.Context, userID int64, code string) error {
	if err := s.repo.Revoke(ctx, userID, code); err != nil {
		return err
	}
	return s.invalidate(ctx, "revoke_permission", userID)
}
func (s *Service) UpsertRoute(ctx context.Context, permission model.RoutePermission) error {
	key, err := s.repo.UpsertRoute(ctx, permission)
	if err != nil {
		return err
	}
	return s.invalidatePending(ctx, "upsert_route_permission", key)
}
func (s *Service) DeletePermissionCode(ctx context.Context, code string) error {
	key, err := s.repo.DeletePermissionCode(ctx, code)
	if err != nil {
		return err
	}
	return s.invalidatePending(ctx, "delete_permission_code", key)
}

func (s *Service) invalidatePending(ctx context.Context, operation, key string) error {
	for {
		rows, err := s.repo.PendingInvalidations(ctx, key)
		if err != nil {
			return fmt.Errorf("database committed but read pending invalidations: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		userIDs := make([]int64, 0, len(rows))
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			userIDs = append(userIDs, row.UserID)
			ids = append(ids, row.ID)
		}
		if err := s.invalidate(ctx, operation, userIDs...); err != nil {
			return err
		}
		if err := s.repo.CompleteInvalidations(ctx, ids); err != nil {
			return fmt.Errorf("database committed but acknowledge invalidations: %w", err)
		}
	}
}
func (s *Service) invalidate(ctx context.Context, operation string, ids ...int64) error {
	if err := s.permissions.DeleteUsers(ctx, ids...); err != nil {
		s.logger.Error(ctx, "permission_cache_invalidation_failed", logger.String("operation", operation), logger.Any("user_ids", ids), logger.Err(err))
		return fmt.Errorf("database committed but invalidate permission cache: %w", err)
	}
	return nil
}
