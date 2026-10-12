package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
)

func managementError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return model.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("management repository: %w", err)
	}
	return nil
}
func (r *Repository) ListUsers(ctx context.Context, q dto.PageQuery) ([]model.User, int64, error) {
	rows := []model.User{}
	var total int64
	db := r.db.WithContext(ctx).Model(&model.User{})
	if q.Email != "" {
		db = db.Where("email LIKE ?", "%"+q.Email+"%")
	}
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.Status != "" {
		db = db.Where("enabled = ?", q.Status == "active")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, managementError(err)
	}
	err := db.Order("id").Offset((q.Current - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, managementError(err)
}
func (r *Repository) ListRoles(ctx context.Context, q dto.PageQuery) ([]model.Role, int64, error) {
	rows := []model.Role{}
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Role{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code LIKE ?", "%"+q.Code+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, managementError(err)
	}
	err := db.Order("id").Offset((q.Current - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, total, managementError(err)
}
func (r *Repository) RoleByID(ctx context.Context, id int64) (model.Role, error) {
	var role model.Role
	err := r.db.WithContext(ctx).First(&role, id).Error
	return role, managementError(err)
}
func (r *Repository) ManagedUser(ctx context.Context, id int64) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	return user, managementError(err)
}

type UserAuthorizationData struct {
	UserRoles       []model.UserRole
	Roles           []model.Role
	RolePermissions []model.RolePermission
	UserPermissions []model.UserPermission
}

func (r *Repository) LoadUserAuthorizationData(ctx context.Context, userIDs []int64) (UserAuthorizationData, error) {
	data := UserAuthorizationData{}
	if len(userIDs) == 0 {
		return data, nil
	}
	if err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Order("user_id, role_id").Find(&data.UserRoles).Error; err != nil {
		return data, managementError(err)
	}
	roleIDs := make([]int64, 0, len(data.UserRoles))
	seenRoleIDs := make(map[int64]bool, len(data.UserRoles))
	for _, userRole := range data.UserRoles {
		if !seenRoleIDs[userRole.RoleID] {
			seenRoleIDs[userRole.RoleID] = true
			roleIDs = append(roleIDs, userRole.RoleID)
		}
	}
	if len(roleIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("id IN ?", roleIDs).Order("id").Find(&data.Roles).Error; err != nil {
			return data, managementError(err)
		}
		if err := r.db.WithContext(ctx).Where("role_id IN ?", roleIDs).Order("role_id, permission_code").Find(&data.RolePermissions).Error; err != nil {
			return data, managementError(err)
		}
	}
	if err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Order("user_id, permission_code").Find(&data.UserPermissions).Error; err != nil {
		return data, managementError(err)
	}
	return data, nil
}

func isSuper(tx *gorm.DB, id int64) (bool, error) {
	var count int64
	err := tx.Table("auth_user_roles AS ur").Joins("JOIN auth_roles AS r ON r.id = ur.role_id").Where("ur.user_id = ? AND r.code = ?", id, "super_admin").Count(&count).Error
	return count > 0, err
}

// superAdminCode 超级管理员角色码，独占最高优先级，任何人（含其本人）都不能修改或删除。
const superAdminCode = "super_admin"

// builtinRoleCodes 内置角色码，不允许通过接口新建或改名占用。
var builtinRoleCodes = map[string]bool{
	"super_admin": true,
	"admin":       true,
	"manager":     true,
	"user":        true,
	"guest":       true,
}

// actorRoleLevel 返回操作者的最高角色优先级；超级管理员视为最高 +1，保证能管理除 super_admin 角色外的一切。
func actorRoleLevel(tx *gorm.DB, actor int64) (int, error) {
	super, err := isSuper(tx, actor)
	if err != nil {
		return 0, err
	}
	if super {
		return 101, nil
	}
	var level int
	if err := tx.Table("auth_roles AS r").
		Joins("JOIN auth_user_roles AS ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND r.code <> ?", actor, superAdminCode).
		Select("COALESCE(MAX(r.level), 0)").Scan(&level).Error; err != nil {
		return 0, err
	}
	return level, nil
}

// requireManageableRole 校验操作者能否管理目标角色：不能管理同级或更高优先级的角色，
// 且 super_admin 角色对所有人（含超级管理员本人）都不可管理。
func requireManageableRole(tx *gorm.DB, actor int64, targetCode string, targetLevel int) error {
	if targetCode == superAdminCode {
		return model.ErrProtected
	}
	level, err := actorRoleLevel(tx, actor)
	if err != nil {
		return err
	}
	if targetLevel >= level {
		return model.ErrProtected
	}
	return nil
}

// 所有管理写操作共用权限写锁；提交前持久化受影响用户，提交后由 Service 清理缓存。
func (r *Repository) manage(ctx context.Context, key string, fn func(*gorm.DB) error) error {
	return managementError(r.permissionTransaction(ctx, func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.Model(&model.User{}).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			return err
		}
		return enqueueInvalidations(tx, key, ids)
	}))
}
func (r *Repository) SaveUser(ctx context.Context, actor int64, user *model.User, roleIDs []int64, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		roles := []model.Role{}
		if len(roleIDs) > 0 {
			if err := tx.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
				return err
			}
		}
		if len(roles) != len(roleIDs) {
			return model.ErrInvalid
		}
		actorSuper, err := isSuper(tx, actor)
		if err != nil {
			return err
		}
		var requestedCodes []string
		if len(roleIDs) > 0 {
			if err := tx.Model(&model.RolePermission{}).Where("role_id IN ?", roleIDs).Pluck("permission_code", &requestedCodes).Error; err != nil {
				return err
			}
		}
		if err := grantable(tx, actor, requestedCodes); err != nil {
			return err
		}
		assigningSuper := false
		for _, role := range roles {
			if role.Code == superAdminCode {
				assigningSuper = true
			}
		}
		if assigningSuper && !actorSuper {
			return model.ErrProtected
		}
		// 分配角色同样受优先级约束：不能分配同级或更高优先级的角色。
		// 编辑自己时跳过：下方 self-check 保证自己的角色集合不会被修改。
		if user.ID != actor {
			for _, role := range roles {
				if err := requireManageableRole(tx, actor, role.Code, role.Level); err != nil {
					return err
				}
			}
		}
		if user.ID == 0 {
			if err := tx.Create(user).Error; err != nil {
				return err
			}
		} else {
			var old model.User
			if err := tx.First(&old, user.ID).Error; err != nil {
				return err
			}
			protected, err := isSuper(tx, user.ID)
			if err != nil {
				return err
			}
			if protected && (!actorSuper || !assigningSuper || !user.Enabled) {
				return model.ErrProtected
			}
			// 目标用户当前已有的角色也必须在可管理范围内，避免越级修改他人角色。
			// 编辑自己的资料时跳过该校验：下方 self-check 已保证自己的角色集合不可变更。
			if user.ID != actor {
				var currentRoles []model.Role
				if err := tx.Table("auth_roles AS r").
					Joins("JOIN auth_user_roles AS ur ON ur.role_id = r.id").
					Where("ur.user_id = ?", user.ID).Find(&currentRoles).Error; err != nil {
					return err
				}
				for _, role := range currentRoles {
					if err := requireManageableRole(tx, actor, role.Code, role.Level); err != nil {
						return err
					}
				}
			}
			if user.ID == actor && !actorSuper {
				var currentRoleIDs []int64
				if err := tx.Model(&model.UserRole{}).Where("user_id = ?", actor).Pluck("role_id", &currentRoleIDs).Error; err != nil {
					return err
				}
				requestedRoleIDs := make(map[int64]struct{}, len(roleIDs))
				for _, roleID := range roleIDs {
					requestedRoleIDs[roleID] = struct{}{}
				}
				if len(currentRoleIDs) != len(requestedRoleIDs) {
					return model.ErrProtected
				}
				for _, roleID := range currentRoleIDs {
					if _, ok := requestedRoleIDs[roleID]; !ok {
						return model.ErrProtected
					}
				}
			}
			if user.Username != old.Username {
				return model.ErrInvalid
			}
			if user.ID == actor && !user.Enabled {
				return model.ErrProtected
			}
			if err := tx.Model(&old).Updates(map[string]any{"display_name": user.DisplayName, "email": user.Email, "phone": user.Phone, "gender": user.Gender, "bio": user.Bio, "enabled": user.Enabled}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		for _, id := range roleIDs {
			if err := tx.Create(&model.UserRole{UserID: user.ID, RoleID: id}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *Repository) DeleteUser(ctx context.Context, actor, id int64, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		protected, err := isSuper(tx, id)
		if err != nil {
			return err
		}
		if protected || actor == id {
			return model.ErrProtected
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.UserPermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	})
}
func (r *Repository) SaveRole(ctx context.Context, actor int64, role *model.Role, codes []string, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		if err := grantable(tx, actor, codes); err != nil {
			return err
		}
		if role.ID != 0 {
			var current []string
			if err := tx.Model(&model.RolePermission{}).Where("role_id = ?", role.ID).Pluck("permission_code", &current).Error; err != nil {
				return err
			}
			if err := grantable(tx, actor, current); err != nil {
				return err
			}
		}
		if role.Code == superAdminCode {
			return model.ErrProtected
		}
		var count int64
		if len(codes) > 0 {
			if err := tx.Model(&model.Permission{}).Where("code IN ?", codes).Count(&count).Error; err != nil {
				return err
			}
		}
		if int(count) != len(codes) {
			return model.ErrInvalid
		}
		if role.ID == 0 {
			// 新建角色：不允许占用内置角色码；优先级必须低于操作者。
			if builtinRoleCodes[role.Code] {
				return model.ErrProtected
			}
			if err := requireManageableRole(tx, actor, role.Code, role.Level); err != nil {
				return err
			}
			if err := tx.Create(role).Error; err != nil {
				return err
			}
		} else {
			var old model.Role
			if err := tx.First(&old, role.ID).Error; err != nil {
				return err
			}
			// 编辑既有角色：不能管理同级及更高优先级角色，super_admin 角色不可修改。
			if err := requireManageableRole(tx, actor, old.Code, old.Level); err != nil {
				return err
			}
			if old.Code != role.Code {
				return model.ErrInvalid
			}
			// 新优先级同样必须低于操作者，且不能借编辑提权到原等级及以上。
			if err := requireManageableRole(tx, actor, role.Code, role.Level); err != nil {
				return err
			}
			if err := tx.Model(&old).Updates(map[string]any{"name": role.Name, "description": role.Description, "level": role.Level}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("role_id = ?", role.ID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		for _, code := range codes {
			if code == "*" {
				return model.ErrProtected
			}
			if err := tx.Create(&model.RolePermission{RoleID: role.ID, PermissionCode: code}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *Repository) DeleteRole(ctx context.Context, actor, id int64, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, id).Error; err != nil {
			return err
		}
		if err := requireManageableRole(tx, actor, role.Code, role.Level); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.UserRole{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return model.ErrConflict
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&role).Error
	})
}
func permissionItems(tx *gorm.DB) ([]dto.PermissionInfo, error) {
	var rows []model.Permission
	if err := tx.Order("code").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]dto.PermissionInfo, 0, len(rows))
	for _, row := range rows {
		var p dto.PermissionInfo
		if err := json.Unmarshal([]byte(row.Definition), &p); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, nil
}
func (r *Repository) SavePermission(ctx context.Context, id string, p *dto.PermissionInfo, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		items, err := permissionItems(tx)
		if err != nil {
			return err
		}
		byID := map[string]dto.PermissionInfo{}
		for _, item := range items {
			byID[item.ID] = item
		}
		if id != "" {
			old, ok := byID[id]
			if !ok {
				return model.ErrNotFound
			}
			if old.Code != p.Code {
				return model.ErrInvalid
			}
			p.ID = id
			if p.ParentID == nil {
				p.ParentID = old.ParentID
			}
		}
		parent := ""
		if p.ParentID != nil {
			parent = *p.ParentID
		}
		seen := map[string]bool{p.ID: true}
		for parent != "" {
			if seen[parent] {
				return model.ErrInvalid
			}
			seen[parent] = true
			item, ok := byID[parent]
			if !ok || item.Type != "menu" {
				return model.ErrInvalid
			}
			parent = ""
			if item.ParentID != nil {
				parent = *item.ParentID
			}
		}
		if p.Type != "menu" {
			for _, item := range items {
				if item.ParentID != nil && *item.ParentID == p.ID {
					return model.ErrConflict
				}
			}
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		row := model.Permission{Code: p.Code, Definition: string(raw)}
		if id == "" {
			return tx.Create(&row).Error
		}
		return tx.Model(&model.Permission{}).Where("code = ?", p.Code).Update("definition", row.Definition).Error
	})
}
func (r *Repository) DeletePermission(ctx context.Context, id, key string) error {
	return r.manage(ctx, key, func(tx *gorm.DB) error {
		items, err := permissionItems(tx)
		if err != nil {
			return err
		}
		code := ""
		for _, p := range items {
			if p.ParentID != nil && *p.ParentID == id {
				return model.ErrConflict
			}
			if p.ID == id {
				code = p.Code
			}
		}
		if code == "" {
			return model.ErrNotFound
		}
		if err := tx.Where("permission_code = ?", code).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("permission_code = ?", code).Delete(&model.UserPermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("permission_code = ?", code).Delete(&model.RoutePermission{}).Error; err != nil {
			return err
		}
		return tx.Where("code = ?", code).Delete(&model.Permission{}).Error
	})
}
func (r *Repository) ChangePassword(ctx context.Context, id int64, oldHash, newHash string) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND password_hash = ? AND enabled = ?", id, oldHash, true).Update("password_hash", newHash)
	if result.Error != nil {
		return managementError(result.Error)
	}
	if result.RowsAffected != 1 {
		return model.ErrUnauthenticated
	}
	return nil
}
func ParseIDs(roles []dto.RoleInfo) ([]int64, error) {
	ids := []int64{}
	seen := map[int64]bool{}
	for _, role := range roles {
		id, err := strconv.ParseInt(role.ID, 10, 64)
		if err != nil || id <= 0 || seen[id] {
			return nil, model.ErrInvalid
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func grantable(tx *gorm.DB, actor int64, codes []string) error {
	super, err := isSuper(tx, actor)
	if err != nil {
		return err
	}
	if super {
		return nil
	}
	var owned []string
	if err := tx.Raw("SELECT permission_code FROM (?) AS direct_codes UNION SELECT permission_code FROM (?) AS role_codes", effectiveCodes(tx, actor), roleCodes(tx, actor)).Scan(&owned).Error; err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, code := range owned {
		allowed[code] = true
	}
	for _, code := range codes {
		if code == "*" || !allowed[code] {
			return model.ErrProtected
		}
	}
	return nil
}
