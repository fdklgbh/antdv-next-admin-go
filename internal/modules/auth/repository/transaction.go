package repository

import (
	"context"
	"database/sql"
	"fmt"

	"antdv-next-admin-go/internal/modules/auth/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) permissionTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先锁定迁移创建的固定记录，覆盖尚不存在的路由和用户授权关系。
		var lock model.PermissionWriteLock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, "id = ?", 1).Error; err != nil {
			return fmt.Errorf("lock permission writes: %w", err)
		}
		return fn(tx)
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
