package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"go-gin-template/internal/modules/auth/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) PermissionRoutes(ctx context.Context, userID int64) ([]model.Route, error) {
	var routes []model.Route
	err := r.db.WithContext(ctx).Table("route_permissions AS rp").
		Select("rp.method, rp.route AS path").
		Joins("JOIN user_permissions AS up ON up.permission_code = rp.permission_code").
		Where("up.user_id = ? AND rp.enabled = ?", userID, true).Scan(&routes).Error
	if err != nil {
		return nil, fmt.Errorf("query permission routes: %w", err)
	}
	return routes, nil
}

func (r *Repository) Grant(ctx context.Context, userID int64, code string) error {
	return r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		permission := model.UserPermission{UserID: userID, PermissionCode: code}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&permission).Error; err != nil {
			return fmt.Errorf("grant permission: %w", err)
		}
		return nil
	})
}
func (r *Repository) Revoke(ctx context.Context, userID int64, code string) error {
	return r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND permission_code = ?", userID, code).Delete(&model.UserPermission{}).Error; err != nil {
			return fmt.Errorf("revoke permission: %w", err)
		}
		return nil
	})
}
func (r *Repository) UpsertRoute(ctx context.Context, permission model.RoutePermission) (string, error) {
	key := invalidationKey("upsert_route_permission", permission.Method, permission.Route)
	var userIDs []int64
	err := r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		codes := []string{permission.PermissionCode}
		var current model.RoutePermission
		if err := tx.Where("method = ? AND route = ?", permission.Method, permission.Route).First(&current).Error; err == nil {
			codes = append(codes, current.PermissionCode)
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("find route permission: %w", err)
		}
		if err := tx.Model(&model.UserPermission{}).Distinct("user_id").Where("permission_code IN ?", codes).Pluck("user_id", &userIDs).Error; err != nil {
			return fmt.Errorf("query affected users: %w", err)
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "method"}, {Name: "route"}}, DoUpdates: clause.AssignmentColumns([]string{"permission_code", "enabled", "updated_at"})}).Create(&permission).Error; err != nil {
			return fmt.Errorf("upsert route permission: %w", err)
		}
		return enqueueInvalidations(tx, key, userIDs)
	})
	return key, err
}
func (r *Repository) DeletePermissionCode(ctx context.Context, code string) (string, error) {
	key := invalidationKey("delete_permission_code", code)
	var userIDs []int64
	err := r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&model.UserPermission{}).Distinct("user_id").Where("permission_code = ?", code).Pluck("user_id", &userIDs).Error; err != nil {
			return fmt.Errorf("query affected users: %w", err)
		}
		if err := tx.Where("permission_code = ?", code).Delete(&model.RoutePermission{}).Error; err != nil {
			return fmt.Errorf("delete route permissions: %w", err)
		}
		if err := tx.Where("permission_code = ?", code).Delete(&model.UserPermission{}).Error; err != nil {
			return fmt.Errorf("delete user permissions: %w", err)
		}
		return enqueueInvalidations(tx, key, userIDs)
	})
	return key, err
}

func invalidationKey(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}

func enqueueInvalidations(tx *gorm.DB, key string, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	rows := make([]model.PermissionInvalidation, 0, len(userIDs))
	for _, userID := range userIDs {
		rows = append(rows, model.PermissionInvalidation{OperationKey: key, UserID: userID})
	}
	if err := tx.CreateInBatches(&rows, 500).Error; err != nil {
		return fmt.Errorf("save permission cache invalidations: %w", err)
	}
	return nil
}

func (r *Repository) PendingInvalidations(ctx context.Context, key string) ([]model.PermissionInvalidation, error) {
	var rows []model.PermissionInvalidation
	if err := r.db.WithContext(ctx).Where("operation_key = ?", key).Order("id").Limit(500).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query permission cache invalidations: %w", err)
	}
	return rows, nil
}

func (r *Repository) CompleteInvalidations(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	// 仅删除已经处理的记录，避免清理并发事务刚提交的失效任务。
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.PermissionInvalidation{}).Error; err != nil {
		return fmt.Errorf("complete permission cache invalidations: %w", err)
	}
	return nil
}
