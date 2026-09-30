package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userPermission000001 struct {
	UserID         int64  `gorm:"primaryKey"`
	PermissionCode string `gorm:"size:128;primaryKey;index"`
	CreatedAt      time.Time
}

func (userPermission000001) TableName() string { return "user_permissions" }

type routePermission000001 struct {
	Method         string `gorm:"size:10;primaryKey"`
	Route          string `gorm:"size:255;primaryKey"`
	PermissionCode string `gorm:"size:128;not null;index"`
	Enabled        bool   `gorm:"not null;default:true"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (routePermission000001) TableName() string { return "route_permissions" }

type permissionInvalidation000001 struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	OperationKey string `gorm:"size:64;not null;index"`
	UserID       int64  `gorm:"not null"`
}

func (permissionInvalidation000001) TableName() string { return "permission_cache_invalidations" }

type permissionWriteLock000001 struct {
	ID int64 `gorm:"primaryKey;autoIncrement:false"`
}

func (permissionWriteLock000001) TableName() string { return "permission_write_locks" }

func migration000001InitSchema() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "000001_init_schema",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(
				&userPermission000001{},
				&routePermission000001{},
				&permissionInvalidation000001{},
				&permissionWriteLock000001{},
			); err != nil {
				return err
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&permissionWriteLock000001{ID: 1}).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&permissionWriteLock000001{},
				&permissionInvalidation000001{},
				&routePermission000001{},
				&userPermission000001{},
			)
		},
	}
}
