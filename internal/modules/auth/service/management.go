package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/modules/auth/repository"

	"golang.org/x/crypto/bcrypt"
)

var managementCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func (s *Service) ManagedUser(ctx context.Context, id int64) (dto.UserInfo, error) {
	u, err := s.repo.ManagedUser(ctx, id)
	if err != nil {
		return dto.UserInfo{}, err
	}
	authorization, err := s.repo.LoadUserAuthorizationData(ctx, []int64{id})
	if err != nil {
		return dto.UserInfo{}, err
	}
	permissionRows, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return dto.UserInfo{}, err
	}
	definitions, permissionsByCode, err := decodePermissionDefinitions(permissionRows)
	if err != nil {
		return dto.UserInfo{}, err
	}
	rolesByID := make(map[int64]model.Role, len(authorization.Roles))
	for _, role := range authorization.Roles {
		rolesByID[role.ID] = role
	}
	roles := make([]model.Role, 0, len(authorization.UserRoles))
	for _, userRole := range authorization.UserRoles {
		if role, ok := rolesByID[userRole.RoleID]; ok {
			roles = append(roles, role)
		}
	}
	roleCodesByID := make(map[int64][]string)
	for _, assignment := range authorization.RolePermissions {
		roleCodesByID[assignment.RoleID] = append(roleCodesByID[assignment.RoleID], assignment.PermissionCode)
	}
	directCodes := make([]string, 0, len(authorization.UserPermissions))
	for _, assignment := range authorization.UserPermissions {
		directCodes = append(directCodes, assignment.PermissionCode)
	}
	return managedUserInfo(u, roles, roleCodesByID, directCodes, definitions, permissionsByCode), nil
}
func (s *Service) ListUsers(ctx context.Context, q dto.PageQuery) (dto.Page[dto.UserInfo], error) {
	rows, total, err := s.repo.ListUsers(ctx, q)
	out := dto.Page[dto.UserInfo]{List: []dto.UserInfo{}, Total: total, Current: q.Current, PageSize: q.PageSize}
	if err != nil {
		return out, err
	}
	if len(rows) == 0 {
		return out, nil
	}
	userIDs := make([]int64, 0, len(rows))
	for _, user := range rows {
		userIDs = append(userIDs, user.ID)
	}
	authorization, err := s.repo.LoadUserAuthorizationData(ctx, userIDs)
	if err != nil {
		return out, err
	}
	permissionRows, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return out, err
	}
	definitions, permissionsByCode, err := decodePermissionDefinitions(permissionRows)
	if err != nil {
		return out, err
	}
	rolesByID := make(map[int64]model.Role, len(authorization.Roles))
	for _, role := range authorization.Roles {
		rolesByID[role.ID] = role
	}
	rolesByUserID := make(map[int64][]model.Role, len(rows))
	for _, userRole := range authorization.UserRoles {
		if role, ok := rolesByID[userRole.RoleID]; ok {
			rolesByUserID[userRole.UserID] = append(rolesByUserID[userRole.UserID], role)
		}
	}
	roleCodesByID := make(map[int64][]string)
	for _, assignment := range authorization.RolePermissions {
		roleCodesByID[assignment.RoleID] = append(roleCodesByID[assignment.RoleID], assignment.PermissionCode)
	}
	directCodesByUserID := make(map[int64][]string, len(rows))
	for _, assignment := range authorization.UserPermissions {
		directCodesByUserID[assignment.UserID] = append(directCodesByUserID[assignment.UserID], assignment.PermissionCode)
	}
	for _, user := range rows {
		out.List = append(out.List, managedUserInfo(
			user, rolesByUserID[user.ID], roleCodesByID, directCodesByUserID[user.ID], definitions, permissionsByCode,
		))
	}
	return out, nil
}

func managedUserInfo(
	user model.User,
	roles []model.Role,
	roleCodesByID map[int64][]string,
	directCodes []string,
	definitions []dto.PermissionInfo,
	permissionsByCode map[string]dto.PermissionInfo,
) dto.UserInfo {
	roleInfos := make([]dto.RoleInfo, 0, len(roles))
	effectiveCodes := make(map[string]struct{})
	superAdmin := false
	for _, role := range roles {
		codes := roleCodesByID[role.ID]
		info := roleInfoFromPermissions(role, codes, definitions)
		if role.Code == "super_admin" {
			superAdmin = true
			info.Permissions[0].Description = "超级管理员全部权限"
		}
		roleInfos = append(roleInfos, info)
		if role.Code != "super_admin" {
			for _, code := range codes {
				if code != "*" {
					effectiveCodes[code] = struct{}{}
				}
			}
		}
	}
	for _, code := range directCodes {
		if code != "*" {
			effectiveCodes[code] = struct{}{}
		}
	}
	var userPermissions []dto.PermissionInfo
	if superAdmin {
		userPermissions = []dto.PermissionInfo{{ID: "*", Code: "*", Name: json.RawMessage(`"全部权限"`), Description: "超级管理员全部权限", Type: "api", Resource: "*", Action: "*"}}
	} else {
		codes := make([]string, 0, len(effectiveCodes))
		for code := range effectiveCodes {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		userPermissions = resolvePermissionCodes(codes, permissionsByCode)
	}
	status := "inactive"
	if user.Enabled {
		status = "active"
	}
	return dto.UserInfo{
		ID: strconv.FormatInt(user.ID, 10), Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Phone: user.Phone, Gender: user.Gender, Bio: user.Bio, Status: status,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Roles: roleInfos, Permissions: userPermissions,
	}
}

func decodePermissionDefinitions(rows []model.Permission) ([]dto.PermissionInfo, map[string]dto.PermissionInfo, error) {
	definitions := make([]dto.PermissionInfo, 0, len(rows))
	byCode := make(map[string]dto.PermissionInfo, len(rows))
	for _, row := range rows {
		var item dto.PermissionInfo
		if err := json.Unmarshal([]byte(row.Definition), &item); err != nil {
			return nil, nil, fmt.Errorf("decode permission definition %q: %w", row.Code, err)
		}
		definitions = append(definitions, item)
		byCode[row.Code] = item
	}
	return definitions, byCode, nil
}

func decodeSelectedPermissionDefinitions(rows []model.Permission, codes []string) ([]dto.PermissionInfo, error) {
	selected := make(map[string]bool, len(codes))
	for _, code := range codes {
		selected[code] = true
	}
	definitions := make([]dto.PermissionInfo, 0, len(selected))
	for _, row := range rows {
		if !selected[row.Code] {
			continue
		}
		var item dto.PermissionInfo
		if err := json.Unmarshal([]byte(row.Definition), &item); err != nil {
			return nil, fmt.Errorf("decode permission definition %q: %w", row.Code, err)
		}
		definitions = append(definitions, item)
	}
	return definitions, nil
}

func roleInfoFromPermissions(role model.Role, codes []string, definitions []dto.PermissionInfo) dto.RoleInfo {
	selected := make(map[string]bool, len(codes))
	for _, code := range codes {
		selected[code] = true
	}
	permissions := []dto.PermissionInfo{}
	if role.Code == "super_admin" {
		permissions = []dto.PermissionInfo{{ID: "*", Code: "*", Name: json.RawMessage(`"全部权限"`), Type: "api", Resource: "*", Action: "*"}}
	} else {
		for _, item := range definitions {
			if selected[item.Code] {
				permissions = append(permissions, item)
			}
		}
	}
	return dto.RoleInfo{ID: strconv.FormatInt(role.ID, 10), Name: role.Name, Code: role.Code, Description: role.Description, Level: role.Level, Permissions: permissions, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func resolvePermissionCodes(codes []string, byCode map[string]dto.PermissionInfo) []dto.PermissionInfo {
	permissions := make([]dto.PermissionInfo, 0, len(codes))
	for _, code := range codes {
		if code == "*" {
			continue
		}
		item, ok := byCode[code]
		if !ok {
			name, _ := json.Marshal(code)
			item = dto.PermissionInfo{ID: code, Code: code, Name: name, Type: "api"}
		}
		permissions = append(permissions, item)
	}
	return permissions
}

func (s *Service) roleInfo(ctx context.Context, role model.Role) (dto.RoleInfo, error) {
	if role.Code == "super_admin" {
		return roleInfoFromPermissions(role, nil, nil), nil
	}
	codes, err := s.repo.RolePermissionCodes(ctx, role.ID)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	definitions, err := s.repo.PermissionDefinitions(ctx)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	items, err := decodeSelectedPermissionDefinitions(definitions, codes)
	if err != nil {
		return dto.RoleInfo{}, err
	}
	return roleInfoFromPermissions(role, codes, items), nil
}
func (s *Service) ListRoles(ctx context.Context, q dto.PageQuery) (dto.Page[dto.RoleInfo], error) {
	rows, total, err := s.repo.ListRoles(ctx, q)
	out := dto.Page[dto.RoleInfo]{List: []dto.RoleInfo{}, Total: total, Current: q.Current, PageSize: q.PageSize}
	if err != nil {
		return out, err
	}
	if len(rows) == 0 {
		return out, nil
	}
	roleIDs := make([]int64, 0, len(rows))
	needsDefinitions := false
	for _, role := range rows {
		roleIDs = append(roleIDs, role.ID)
		needsDefinitions = needsDefinitions || role.Code != "super_admin"
	}
	assignments, err := s.repo.RolePermissionAssignments(ctx, roleIDs)
	if err != nil {
		return out, err
	}
	codesByRoleID := make(map[int64][]string)
	for _, assignment := range assignments {
		codesByRoleID[assignment.RoleID] = append(codesByRoleID[assignment.RoleID], assignment.PermissionCode)
	}
	var definitions []dto.PermissionInfo
	if needsDefinitions {
		selectedCodes := make(map[string]struct{})
		for _, codes := range codesByRoleID {
			for _, code := range codes {
				selectedCodes[code] = struct{}{}
			}
		}
		codes := make([]string, 0, len(selectedCodes))
		for code := range selectedCodes {
			codes = append(codes, code)
		}
		permissionRows, err := s.repo.PermissionDefinitions(ctx)
		if err != nil {
			return out, err
		}
		definitions, err = decodeSelectedPermissionDefinitions(permissionRows, codes)
		if err != nil {
			return out, err
		}
	}
	for _, role := range rows {
		out.List = append(out.List, roleInfoFromPermissions(role, codesByRoleID[role.ID], definitions))
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
	if strings.TrimSpace(input.Code) != input.Code ||
		len(input.Code) > 64 ||
		!managementCodePattern.MatchString(input.Code) ||
		strings.TrimSpace(input.Name) == "" {
		return dto.RoleInfo{}, model.ErrInvalid
	}
	role := model.Role{ID: id, Name: input.Name, Code: input.Code, Description: input.Description, Level: input.Level}
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
func (s *Service) DeleteRole(ctx context.Context, actor, id int64) error {
	key := fmt.Sprintf("delete_role_%d", id)
	if err := s.repo.DeleteRole(ctx, actor, id, key); err != nil {
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
			parentID := ""
			if p.ParentID != nil {
				parentID = *p.ParentID
			}
			if parentID != parent {
				continue
			}
			p.Children = build(p.ID)
			if permissionMatchesQuery(p, q) || len(p.Children) > 0 {
				out = append(out, p)
			}
		}
		return out
	}
	return build(""), nil
}

// permissionSearchName 提取权限名称的可搜索文本，多语言对象只拼接语言值，不包含 JSON key 与标点。
func permissionSearchName(name json.RawMessage) string {
	var plain string
	if err := json.Unmarshal(name, &plain); err == nil {
		return plain
	}
	var localized map[string]string
	if err := json.Unmarshal(name, &localized); err != nil {
		return ""
	}
	keys := make([]string, 0, len(localized))
	for key := range localized {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(localized[key]); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, " ")
}

// permissionMatchesQuery 判断权限是否命中列表查询条件，名称匹配使用解析后的可搜索文本。
func permissionMatchesQuery(p dto.PermissionInfo, q dto.PageQuery) bool {
	name := permissionSearchName(p.Name)
	if q.Keyword != "" {
		keyword := strings.ToLower(q.Keyword)
		if !strings.Contains(strings.ToLower(p.Code), keyword) && !strings.Contains(strings.ToLower(name), keyword) {
			return false
		}
	}
	if q.Code != "" && !strings.Contains(p.Code, q.Code) {
		return false
	}
	if q.Name != "" && !strings.Contains(name, q.Name) {
		return false
	}
	if q.Status != "" && p.Status != q.Status {
		return false
	}
	if q.Type != "" && p.Type != q.Type {
		return false
	}
	return true
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
	parentIDTooLong := p.ParentID != nil && len(*p.ParentID) > 128
	if p.Code == "" || p.Code == "*" || len(p.Code) > 128 ||
		!managementCodePattern.MatchString(p.Code) ||
		len(p.Description) > 1000 || len(p.Resource) > 255 || len(p.Action) > 64 ||
		len(p.Path) > 255 || len(p.Component) > 255 || len(p.Icon) > 128 || parentIDTooLong {
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
		if strings.TrimSpace(localized["zh-CN"]) == "" {
			return p, model.ErrInvalid
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
