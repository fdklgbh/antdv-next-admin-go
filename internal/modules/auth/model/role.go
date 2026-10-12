package model

import "time"

type Role struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Code        string `gorm:"size:64;not null;uniqueIndex"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"size:255;not null"`
	Level       int    `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Role) TableName() string { return "auth_roles" }

type Permission struct {
	Code       string `gorm:"size:128;primaryKey"`
	Definition string `gorm:"type:text;not null"`
}

func (Permission) TableName() string { return "auth_permissions" }

type UserRole struct {
	UserID int64 `gorm:"primaryKey"`
	RoleID int64 `gorm:"primaryKey;index"`
}

func (UserRole) TableName() string { return "auth_user_roles" }

type RolePermission struct {
	RoleID         int64  `gorm:"primaryKey"`
	PermissionCode string `gorm:"size:128;primaryKey;index"`
}

func (RolePermission) TableName() string { return "auth_role_permissions" }

type BootstrapState struct {
	ID        int64 `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt time.Time
}

func (BootstrapState) TableName() string { return "auth_bootstrap" }
