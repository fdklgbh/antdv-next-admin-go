package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"antdv-next-admin-go/internal/modules/auth/model"
)

func (r *Repository) FindUser(ctx context.Context, username string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, model.ErrUnauthenticated
	}
	if err != nil {
		return user, fmt.Errorf("find auth user: %w", err)
	}
	return user, nil
}
func (r *Repository) UserByID(ctx context.Context, id int64) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, model.ErrUnauthenticated
	}
	if err != nil {
		return user, fmt.Errorf("find auth user by ID: %w", err)
	}
	return user, nil
}
func (r *Repository) PermissionCodes(ctx context.Context, id int64) ([]string, error) {
	codes := []string{}
	if err := r.db.WithContext(ctx).Raw("SELECT permission_code FROM (?) AS direct_codes UNION SELECT permission_code FROM (?) AS role_codes ORDER BY permission_code", effectiveCodes(r.db, id), roleCodes(r.db, id)).Scan(&codes).Error; err != nil {
		return nil, fmt.Errorf("query user permission codes: %w", err)
	}
	return codes, nil
}
func (r *Repository) CreateUser(ctx context.Context, user *model.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("create auth user: %w", err)
	}
	return nil
}
