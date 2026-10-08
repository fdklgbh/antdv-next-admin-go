package service

import (
	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strconv"
)

// prepareDisplay 在写入账号前确认存在可显示密码的终端，不向普通日志输出密码。
func (s *Service) Bootstrap(ctx context.Context, prepareDisplay func() error) (string, error) {
	password := ""
	created, err := s.repo.EnsureSuperAdmin(ctx, s.authConfig.SuperAdminUsername, s.authConfig.SuperAdminEmail, func() (string, error) {
		if err := prepareDisplay(); err != nil {
			return "", err
		}
		value, err := randomToken()
		if err != nil {
			return "", err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
		if err != nil {
			return "", fmt.Errorf("hash super admin password: %w", err)
		}
		password = value
		return string(hash), nil
	})
	if err != nil {
		return "", err
	}
	if !created {
		return "", nil
	}
	return password, nil
}

func (s *Service) userAuthorization(ctx context.Context, user model.User, codes []string) ([]dto.RoleInfo, []dto.PermissionInfo, error) {
	super, err := s.repo.IsSuperAdmin(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	roles, err := s.repo.Roles(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	definitions, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return nil, nil, err
	}
	byCode := make(map[string]dto.PermissionInfo, len(definitions))
	for _, row := range definitions {
		var item dto.PermissionInfo
		if err := json.Unmarshal([]byte(row.Definition), &item); err != nil {
			return nil, nil, fmt.Errorf("decode permission definition: %w", err)
		}
		byCode[row.Code] = item
	}
	resolve := func(codes []string) []dto.PermissionInfo {
		result := make([]dto.PermissionInfo, 0, len(codes))
		for _, code := range codes {
			// 星号仅由超级管理员角色注入，普通角色或直接授权不能取得此特权。
			if code == "*" {
				continue
			}
			item, ok := byCode[code]
			if !ok {
				name, _ := json.Marshal(code)
				item = dto.PermissionInfo{ID: code, Code: code, Name: name, Type: "api"}
			}
			result = append(result, item)
		}
		return result
	}
	all := dto.PermissionInfo{ID: "*", Code: "*", Name: json.RawMessage(`"全部权限"`), Description: "超级管理员全部权限", Type: "api", Resource: "*", Action: "*"}
	result := make([]dto.RoleInfo, 0, len(roles))
	for _, role := range roles {
		roleCodes, err := s.repo.RolePermissionCodes(ctx, role.ID)
		if err != nil {
			return nil, nil, err
		}
		permissions := resolve(roleCodes)
		if role.Code == "super_admin" {
			permissions = []dto.PermissionInfo{all}
		}
		result = append(result, dto.RoleInfo{ID: strconv.FormatInt(role.ID, 10), Code: role.Code, Name: role.Name, Description: role.Description, Permissions: permissions, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt})
	}
	permissions := resolve(codes)
	if super {
		permissions = []dto.PermissionInfo{all}
	}
	return result, permissions, nil
}
