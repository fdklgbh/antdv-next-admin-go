package service

import (
	"encoding/json"
	"strings"
	"testing"

	"antdv-next-admin-go/internal/modules/auth/dto"
)

func TestPermissionSearchName(t *testing.T) {
	t.Run("JSON 字符串名称", func(t *testing.T) {
		if got := permissionSearchName(json.RawMessage(`"Dashboard"`)); got != "Dashboard" {
			t.Fatalf("permissionSearchName() = %q, want %q", got, "Dashboard")
		}
	})
	t.Run("多语言对象只包含语言值", func(t *testing.T) {
		got := permissionSearchName(json.RawMessage(`{"zh-CN":"用户管理","en-US":"User Management","ja-JP":"ユーザー管理","ko-KR":"사용자 관리"}`))
		for _, value := range []string{"用户管理", "User Management", "ユーザー管理", "사용자 관리"} {
			if !strings.Contains(got, value) {
				t.Fatalf("permissionSearchName() = %q, want contains %q", got, value)
			}
		}
		for _, absent := range []string{"zh-CN", "en-US", "ja-JP", "ko-KR", "{", "}", ":", `"`} {
			if strings.Contains(got, absent) {
				t.Fatalf("permissionSearchName() = %q, want not contains %q", got, absent)
			}
		}
	})
	t.Run("多语言对象跳过空白值", func(t *testing.T) {
		got := permissionSearchName(json.RawMessage(`{"zh-CN":"用户管理","en-US":"   "}`))
		if got != "用户管理" {
			t.Fatalf("permissionSearchName() = %q, want %q", got, "用户管理")
		}
	})
	t.Run("无效 JSON 返回空文本", func(t *testing.T) {
		if got := permissionSearchName(json.RawMessage(`{invalid`)); got != "" {
			t.Fatalf("permissionSearchName() = %q, want empty", got)
		}
	})
}

func TestPermissionMatchesQuery(t *testing.T) {
	localized := dto.PermissionInfo{
		ID:     "11",
		Code:   "system.user.view",
		Name:   json.RawMessage(`{"zh-CN":"用户管理","en-US":"User Management"}`),
		Type:   "menu",
		Status: "active",
	}
	plain := dto.PermissionInfo{
		ID:     "1",
		Code:   "dashboard.view",
		Name:   json.RawMessage(`"Dashboard"`),
		Type:   "menu",
		Status: "active",
	}
	tests := []struct {
		name       string
		permission dto.PermissionInfo
		query      dto.PageQuery
		want       bool
	}{
		{
			name:       "中文 keyword 命中多语言名称",
			permission: localized,
			query:      dto.PageQuery{Keyword: "用户"},
			want:       true,
		},
		{
			name:       "英文 keyword 命中多语言名称",
			permission: localized,
			query:      dto.PageQuery{Keyword: "user management"},
			want:       true,
		},
		{
			name:       "keyword 命中 JSON 字符串名称",
			permission: plain,
			query:      dto.PageQuery{Keyword: "dashboard"},
			want:       true,
		},
		{
			name:       "keyword 忽略大小写命中 code",
			permission: localized,
			query:      dto.PageQuery{Keyword: "SYSTEM.USER"},
			want:       true,
		},
		{
			name:       "keyword 不匹配多语言对象的 JSON key",
			permission: localized,
			query:      dto.PageQuery{Keyword: "zh-CN"},
			want:       false,
		},
		{
			name:       "keyword 无命中",
			permission: localized,
			query:      dto.PageQuery{Keyword: "角色"},
			want:       false,
		},
		{
			name:       "name 过滤命中多语言名称值",
			permission: localized,
			query:      dto.PageQuery{Name: "用户"},
			want:       true,
		},
		{
			name:       "name 过滤不匹配 JSON key",
			permission: localized,
			query:      dto.PageQuery{Name: "zh-CN"},
			want:       false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := permissionMatchesQuery(test.permission, test.query); got != test.want {
				t.Fatalf("permissionMatchesQuery() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPermissionMatchesQueryEmptyKeywordReturnsAll(t *testing.T) {
	permissions := []dto.PermissionInfo{
		{ID: "1", Code: "dashboard.view", Name: json.RawMessage(`"Dashboard"`), Type: "menu", Status: "active"},
		{ID: "11", Code: "system.user.view", Name: json.RawMessage(`{"zh-CN":"用户管理","en-US":"User Management"}`), Type: "menu", Status: "active"},
		{ID: "21", Code: "system.role.create", Name: json.RawMessage(`{"zh-CN":"新增角色","en-US":"Create Role"}`), Type: "button", Status: "inactive"},
	}
	for _, permission := range permissions {
		if !permissionMatchesQuery(permission, dto.PageQuery{}) {
			t.Fatalf("空 keyword 应返回全部权限，permission %q 被过滤", permission.Code)
		}
	}
}
