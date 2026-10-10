package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type authUser000002 struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string `gorm:"size:255;not null"`
	Email        string `gorm:"size:254;not null;default:''"`
	DisplayName  string `gorm:"size:128;not null"`
	Enabled      bool   `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (authUser000002) TableName() string { return "auth_users" }
func migration000002AuthUsers() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID:       "000002_auth_users",
		Migrate:  func(tx *gorm.DB) error { return tx.AutoMigrate(&authUser000002{}) },
		Rollback: func(tx *gorm.DB) error { return tx.Migrator().DropTable(&authUser000002{}) },
	}
}
