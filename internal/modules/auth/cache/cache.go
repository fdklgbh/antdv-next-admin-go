package cache

import (
	"antdv-next-admin-go/internal/modules/auth/model"
	"context"
	"time"
)

type SessionCache interface {
	GetRefresh(context.Context, string) (model.RefreshSession, error)
	SetRefresh(context.Context, string, model.RefreshSession, time.Duration) error
	DeleteRefresh(context.Context, string) error
}
type PermissionCache interface {
	GetRoutes(context.Context, int64) ([]model.Route, error)
	SetRoutes(context.Context, int64, []model.Route, time.Duration) error
	DeleteUsers(context.Context, ...int64) error
}
