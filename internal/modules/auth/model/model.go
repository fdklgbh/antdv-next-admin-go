package model

import (
	"errors"
	"time"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type Principal struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}
type Route struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type UserPermission struct {
	UserID         int64  `gorm:"primaryKey"`
	PermissionCode string `gorm:"size:128;primaryKey;index"`
	CreatedAt      time.Time
}

func (UserPermission) TableName() string { return "user_permissions" }

type RoutePermission struct {
	Method         string `gorm:"size:10;primaryKey"`
	Route          string `gorm:"size:255;primaryKey"`
	PermissionCode string `gorm:"size:128;not null;index"`
	Enabled        bool   `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (RoutePermission) TableName() string { return "route_permissions" }

type PermissionInvalidation struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	OperationKey string `gorm:"size:64;not null;index"`
	UserID       int64  `gorm:"not null"`
}

func (PermissionInvalidation) TableName() string { return "permission_cache_invalidations" }

type PermissionWriteLock struct {
	ID int64 `gorm:"primaryKey;autoIncrement:false"`
}

func (PermissionWriteLock) TableName() string { return "permission_write_locks" }
