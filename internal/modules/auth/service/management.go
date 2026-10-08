package service

import (
	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/modules/auth/repository"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"sort"
	"strconv"
	"strings"
)

func (s *Service) ManagedUser(ctx context.Context, id int64) (dto.UserInfo, error) {
	u, err := s.repo.ManagedUser(ctx, id)
	if err != nil {
		return dto.UserInfo{}, err
	}
	codes, err := s.repo.PermissionCodes(ctx, id)
	if err != nil {
		return dto.UserInfo{}, err
	}
	roles, permissions, err := s.userAuthorization(ctx, u, codes)
	if err != nil {
		return dto.UserInfo{}, err
	}
	status := "inactive"
	if u.Enabled {
		status = "active"
	}
	return dto.UserInfo{ID: strconv.FormatInt(u.ID, 10), Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Phone: u.Phone, Gender: u.Gender, Bio: u.Bio, Status: status, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, Roles: roles, Permissions: permissions}, nil
}
func (s *Service) ListUsers(ctx context.Context, q dto.PageQuery) (dto.Page[dto.UserInfo], error) {
	rows, total, err := s.repo.ListUsers(ctx, q)
	out := dto.Page[dto.UserInfo]{List: []dto.UserInfo{}, Total: total, Current: q.Current, PageSize: q.PageSize}
	if err != nil {
		return out, err
	}
	for _, u := range rows {
		info, err := s.ManagedUser(ctx, u.ID)
		if err != nil {
			return out, err
		}
		out.List = append(out.List, info)
	}
	return out, nil
}
func (s *Service) roleInfo(ctx context.Context, role model.Role) (dto.RoleInfo, error) {
	codes, err := s.repo.RolePermissionCodes(ctx, role.ID)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	definitions, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	selected := map[string]bool{}
	for _, code := range codes {
		selected[code] = true
	}
	permissions := []dto.PermissionInfo{}
	for _, row := range definitions {
		if selected[row.Code] {
			var item dto.PermissionInfo
			if err := json.Unmarshal([]byte(row.Definition), &item); err != nil {
				return dto.RoleInfo{}, err
			}
			permissions = append(permissions, item)
		}
	}
	if role.Code == "super_admin" {
		permissions = []dto.PermissionInfo{{ID: "*", Code: "*", Name: json.RawMessage(`"全部权限"`), Type: "api", Resource: "*", Action: "*"}}
	}
	return dto.RoleInfo{ID: strconv.FormatInt(role.ID, 10), Name: role.Name, Code: role.Code, Description: role.Description, Permissions: permissions, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}, nil
}
func (s *Service) ListRoles(ctx context.Context, q dto.PageQuery) (dto.Page[dto.RoleInfo], error) {
	rows, total, err := s.repo.ListRoles(ctx, q)
	out := dto.Page[dto.RoleInfo]{List: []dto.RoleInfo{}, Total: total, Current: q.Current, PageSize: q.PageSize}
	if err != nil {
		return out, err
	}
	for _, role := range rows {
		info, err := s.roleInfo(ctx, role)
		if err != nil {
			return out, err
		}
		out.List = append(out.List, info)
	}
	return out, nil
}
func (s *Service) Role(ctx context.Context, id int64) (dto.RoleInfo, error) {
	role, err := s.repo.RoleByID(ctx, id)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	return s.roleInfo(ctx, role)
}
func (s *Service) SaveUser(ctx context.Context, actor, id int64, input dto.UserInput) (dto.UserInfo, error) {
	if strings.TrimSpace(input.Username) != input.Username || strings.TrimSpace(input.DisplayName) == "" {
		return dto.UserInfo{}, model.ErrInvalid
	}
	user := model.User{ID: id, Username: input.Username, DisplayName: input.DisplayName, Email: input.Email, Phone: input.Phone, Gender: input.Gender, Bio: input.Bio, Enabled: input.Status == "active"}
	if id == 0 {
		if len(input.Password) < 8 || len(input.Password) > 72 {
			return dto.UserInfo{}, model.ErrInvalid
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return dto.UserInfo{}, err
		}
		user.PasswordHash = string(hash)
	} else if input.Password != "" {
		return dto.UserInfo{}, model.ErrInvalid
	}
	ids, err := repository.ParseIDs(input.Roles)
	if err != nil {
		return dto.UserInfo{}, err
	}
	key := fmt.Sprintf("manage_user_%d_%s", id, input.Username)
	if err := s.repo.SaveUser(ctx, actor, &user, ids, key); err != nil {
		return dto.UserInfo{}, err
	}
	if err := s.invalidatePending(ctx, "save_user", key); err != nil {
		return dto.UserInfo{}, err
	}
	return s.ManagedUser(ctx, user.ID)
}
func (s *Service) DeleteUser(ctx context.Context, actor, id int64) error {
	key := fmt.Sprintf("delete_user_%d", id)
	if err := s.repo.DeleteUser(ctx, actor, id, key); err != nil {
		return err
	}
	return s.invalidatePending(ctx, "delete_user", key)
}
func (s *Service) SaveRole(ctx context.Context, actor, id int64, input dto.RoleInput) (dto.RoleInfo, error) {
	if strings.TrimSpace(input.Code) != input.Code || strings.TrimSpace(input.Name) == "" {
		return dto.RoleInfo{}, model.ErrInvalid
	}
	role := model.Role{ID: id, Name: input.Name, Code: input.Code, Description: input.Description}
	codes := []string{}
	seen := map[string]bool{}
	for _, p := range input.Permissions {
		if p.Code == "" || p.Code == "*" || seen[p.Code] {
			return dto.RoleInfo{}, model.ErrInvalid
		}
		seen[p.Code] = true
		codes = append(codes, p.Code)
	}
	key := fmt.Sprintf("manage_role_%d_%s", id, input.Code)
	if err := s.repo.SaveRole(ctx, actor, &role, codes, key); err != nil {
		return dto.RoleInfo{}, err
	}
	if err := s.invalidatePending(ctx, "save_role", key); err != nil {
		return dto.RoleInfo{}, err
	}
	return s.Role(ctx, role.ID)
}
func (s *Service) DeleteRole(ctx context.Context, id int64) error {
	key := fmt.Sprintf("delete_role_%d", id)
	if err := s.repo.DeleteRole(ctx, id, key); err != nil {
		return err
	}
	return s.invalidatePending(ctx, "delete_role", key)
}
func (s *Service) Permissions(ctx context.Context, q dto.PageQuery, tree bool) ([]dto.PermissionInfo, error) {
	rows, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	all := []dto.PermissionInfo{}
	for _, row := range rows {
		var p dto.PermissionInfo
		if err := json.Unmarshal([]byte(row.Definition), &p); err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Sort < all[j].Sort })
	if !tree {
		return all, nil
	}
	var build func(string) []dto.PermissionInfo
	build = func(parent string) []dto.PermissionInfo {
		out := []dto.PermissionInfo{}
		for _, p := range all {
			if p.ParentID != parent {
				continue
			}
			p.Children = build(p.ID)
			match := (q.Keyword == "" || strings.Contains(strings.ToLower(p.Code), strings.ToLower(q.Keyword)) || strings.Contains(strings.ToLower(string(p.Name)), strings.ToLower(q.Keyword))) && (q.Code == "" || strings.Contains(p.Code, q.Code)) && (q.Name == "" || strings.Contains(string(p.Name), q.Name)) && (q.Status == "" || p.Status == q.Status) && (q.Type == "" || p.Type == q.Type)
			if match || len(p.Children) > 0 {
				out = append(out, p)
			}
		}
		return out
	}
	return build(""), nil
}
func (s *Service) Permission(ctx context.Context, id string) (dto.PermissionInfo, error) {
	rows, err := s.Permissions(ctx, dto.PageQuery{}, false)
	if err != nil {
		return dto.PermissionInfo{}, err
	}
	for _, p := range rows {
		if p.ID == id {
			return p, nil
		}
	}
	return dto.PermissionInfo{}, model.ErrNotFound
}
func (s *Service) SavePermission(ctx context.Context, id string, p dto.PermissionInfo) (dto.PermissionInfo, error) {
	if p.Code == "" || p.Code == "*" || len(p.Code) > 128 || len(p.Description) > 1000 || len(p.Resource) > 255 || len(p.Action) > 64 || len(p.Path) > 255 || len(p.Component) > 255 || len(p.Icon) > 128 || len(p.ParentID) > 128 {
		return p, model.ErrInvalid
	}
	if p.Type != "menu" && p.Type != "button" && p.Type != "api" {
		return p, model.ErrInvalid
	}
	if p.Status != "active" && p.Status != "inactive" {
		return p, model.ErrInvalid
	}
	var name string
	var localized map[string]string
	if json.Unmarshal(p.Name, &name) != nil {
		if json.Unmarshal(p.Name, &localized) != nil || len(localized) == 0 {
			return p, model.ErrInvalid
		}
		for _, v := range localized {
			if strings.TrimSpace(v) == "" {
				return p, model.ErrInvalid
			}
		}
	} else if strings.TrimSpace(name) == "" {
		return p, model.ErrInvalid
	}
	if len(p.Name) > 2000 || p.Sort < 0 {
		return p, model.ErrInvalid
	}
	if p.Type == "menu" && (!strings.HasPrefix(p.Path, "/") || strings.HasPrefix(p.Path, "//")) {
		return p, model.ErrInvalid
	}
	if id == "" {
		v, err := randomToken()
		if err != nil {
			return p, err
		}
		p.ID = v
	}
	p.Children = nil
	key := "manage_permission_" + p.Code
	if err := s.repo.SavePermission(ctx, id, &p, key); err != nil {
		return p, err
	}
	return p, s.invalidatePending(ctx, "save_permission", key)
}
func (s *Service) DeletePermission(ctx context.Context, id string) error {
	key := "delete_permission_" + id
	if err := s.repo.DeletePermission(ctx, id, key); err != nil {
		return err
	}
	return s.invalidatePending(ctx, "delete_permission", key)
}
func (s *Service) ChangePassword(ctx context.Context, id int64, input dto.PasswordInput) error {
	user, err := s.repo.UserByID(ctx, id)
	if err != nil {
		return err
	}
	if !user.Enabled {
		return model.ErrUnauthenticated
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword)) != nil {
		return model.ErrInvalid
	}
	if len(input.NewPassword) < 8 || len(input.NewPassword) > 72 {
		return model.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, id, user.PasswordHash, string(hash))
}
