package repository

import (
	"antdv-next-admin-go/internal/modules/auth/model"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 普通权限取直接授权与角色授权；超级管理员身份独立查询角色关联。
func effectiveCodes(db *gorm.DB, userID int64) *gorm.DB {
	return db.Table("user_permissions").Select("permission_code").Where("user_id = ?", userID)
}
func roleCodes(db *gorm.DB, userID int64) *gorm.DB {
	return db.Table("auth_role_permissions AS p").Select("p.permission_code").
		Joins("JOIN auth_user_roles AS ur ON ur.role_id = p.role_id").
		Joins("JOIN auth_roles AS r ON r.id = ur.role_id").Where("ur.user_id = ? AND r.code <> ?", userID, "super_admin")
}
func affectedUsers(db *gorm.DB, codes []string) *gorm.DB {
	direct := db.Table("user_permissions").Select("user_id").Where("permission_code IN ?", codes)
	inherited := db.Table("auth_user_roles AS ur").Select("ur.user_id").
		Joins("JOIN auth_role_permissions AS p ON p.role_id = ur.role_id").Where("p.permission_code IN ?", codes)
	return db.Table("auth_users").Select("id").Where("id IN (?) OR id IN (?)", direct, inherited)
}
func (r *Repository) Roles(ctx context.Context, userID int64) ([]model.Role, error) {
	roles := []model.Role{}
	q := r.db.WithContext(ctx).Model(&model.Role{}).
		Where("id IN (?)", r.db.Model(&model.UserRole{}).Select("role_id").Where("user_id = ?", userID))
	if err := q.Order("id").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	return roles, nil
}
func (r *Repository) RolePermissionCodes(ctx context.Context, roleID int64) ([]string, error) {
	codes := []string{}
	if err := r.db.WithContext(ctx).Model(&model.RolePermission{}).Where("role_id = ?", roleID).Order("permission_code").Pluck("permission_code", &codes).Error; err != nil {
		return nil, fmt.Errorf("query role permissions: %w", err)
	}
	return codes, nil
}
func (r *Repository) PermissionDefinitions(ctx context.Context) ([]model.Permission, error) {
	rows := []model.Permission{}
	if err := r.db.WithContext(ctx).Order("code").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query permission definitions: %w", err)
	}
	return rows, nil
}

// 固定行锁串行化多个实例的首次创建；只有真正插入账号的实例生成并显示密码。
func (r *Repository) EnsureSuperAdmin(ctx context.Context, username, email string, create func() (string, error)) (bool, error) {
	created := false
	err := r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		var state model.BootstrapState
		stateErr := tx.First(&state, "id = ?", 1).Error
		if stateErr == nil {
			return nil
		}
		if !errors.Is(stateErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("read bootstrap state (run task migrate:up first): %w", stateErr)
		}
		var role model.Role
		roleErr := tx.Where("code = ?", "super_admin").First(&role).Error
		if errors.Is(roleErr, gorm.ErrRecordNotFound) {
			role = model.Role{Code: "super_admin", Name: "超级管理员", Description: "首次初始化创建，拥有全部权限"}
			if err := tx.Create(&role).Error; err != nil {
				return fmt.Errorf("create super admin role: %w", err)
			}
		} else if roleErr != nil {
			return fmt.Errorf("load super admin role (run task migrate:up first): %w", roleErr)
		}
		var user model.User
		err := tx.Where("username = ?", username).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			hash, err := create()
			if err != nil {
				return err
			}
			user = model.User{Username: username, Email: email, DisplayName: "超级管理员", PasswordHash: hash, Enabled: true}
			if err := tx.Create(&user).Error; err != nil {
				return fmt.Errorf("create super admin: %w", err)
			}
			created = true
		} else if err != nil {
			return fmt.Errorf("find super admin: %w", err)
		}
		if user.Username != username {
			return fmt.Errorf("super admin username differs in case from existing account")
		}
		if !user.Enabled {
			return fmt.Errorf("configured super admin account is disabled")
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
			return fmt.Errorf("assign super admin role: %w", err)
		}
		if err := tx.Create(&model.BootstrapState{ID: 1}).Error; err != nil {
			return fmt.Errorf("save bootstrap state: %w", err)
		}
		return nil
	})
	return created && err == nil, err
}

func (r *Repository) IsSuperAdmin(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("auth_user_roles AS ur").Joins("JOIN auth_roles AS r ON r.id = ur.role_id").Where("ur.user_id = ? AND r.code = ?", userID, "super_admin").Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("query super admin role: %w", err)
	}
	return count > 0, nil
}
