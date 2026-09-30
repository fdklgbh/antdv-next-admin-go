package cache

import (
	"context"
	"go-gin-template/internal/modules/auth/model"
	"time"
)

type SessionCache interface {
	GetPrincipal(context.Context, string) (model.Principal, error)
}
type PermissionCache interface {
	GetRoutes(context.Context, int64) ([]model.Route, error)
	SetRoutes(context.Context, int64, []model.Route, time.Duration) error
	DeleteUsers(context.Context, ...int64) error
}
