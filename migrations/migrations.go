package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func All() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		migration000001InitSchema(),
	}
}
func Migrator(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, All())
}
