package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
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
	roles, err := s.repo.Roles(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	roleIDs := make([]int64, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}
	assignments, err := s.repo.RolePermissionAssignments(ctx, roleIDs)
	if err != nil {
		return nil, nil, err
	}
	roleCodesByID := make(map[int64][]string)
	for _, assignment := range assignments {
		roleCodesByID[assignment.RoleID] = append(roleCodesByID[assignment.RoleID], assignment.PermissionCode)
	}
	definitions, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, byCode, err := decodePermissionDefinitions(definitions)
	if err != nil {
		return nil, nil, err
	}
	all := dto.PermissionInfo{ID: "*", Code: "*", Name: json.RawMessage(`"全部权限"`), Description: "超级管理员全部权限", Type: "api", Resource: "*", Action: "*"}
	result := make([]dto.RoleInfo, 0, len(roles))
	super := false
	for _, role := range roles {
		permissions := resolvePermissionCodes(roleCodesByID[role.ID], byCode)
		if role.Code == "super_admin" {
			super = true
			permissions = []dto.PermissionInfo{all}
		}
		result = append(result, dto.RoleInfo{ID: strconv.FormatInt(role.ID, 10), Code: role.Code, Name: role.Name, Description: role.Description, Level: role.Level, Permissions: permissions, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt})
	}
	permissions := resolvePermissionCodes(codes, byCode)
	if super {
		permissions = []dto.PermissionInfo{all}
	}
	return result, permissions, nil
}
