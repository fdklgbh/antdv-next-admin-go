package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type route000005 struct {
	Method         string `gorm:"size:10;primaryKey"`
	Route          string `gorm:"size:255;primaryKey"`
	PermissionCode string `gorm:"size:128;not null;index"`
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (route000005) TableName() string { return "route_permissions" }

func migration000005ConfigGroups() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "000005_config_groups",
		Migrate: func(tx *gorm.DB) error {
			route := route000005{
				Method:         "GET",
				Route:          "/api/config/groups",
				PermissionCode: "system.config.view",
				Enabled:        true,
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&route).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Where("method = ? AND route = ?", "GET", "/api/config/groups").Delete(&route000005{}).Error
		},
	}
}
