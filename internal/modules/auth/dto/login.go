package dto

import (
	"encoding/json"
	"time"
)

type LoginRequest struct {
	Username     string `json:"username" binding:"required,min=1,max=64"`
	Password     string `json:"password" binding:"required,max=72"`
	Remember     bool   `json:"remember"`
	RememberDays int    `json:"rememberDays" binding:"required_if=Remember true,excluded_if=Remember false,omitempty,oneof=7 15 30"`
}
type LoginResult struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expiresIn"`
	SessionID string `json:"sessionId"`
	Remember  bool   `json:"remember"`
}
type PermissionInfo struct {
	Children    []PermissionInfo `json:"children,omitempty"`
	ID          string           `json:"id"`
	Name        json.RawMessage  `json:"name"`
	Code        string           `json:"code"`
	Description string           `json:"description"`
	Resource    string           `json:"resource"`
	Action      string           `json:"action"`
	Type        string           `json:"type"`
	ParentID    string           `json:"parentId,omitempty"`
	Path        string           `json:"path,omitempty"`
	Component   string           `json:"component,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Sort        int              `json:"sort,omitempty"`
	Status      string           `json:"status,omitempty"`
	Visible     *bool            `json:"visible,omitempty"`
}
type UserInfo struct {
	Gender      string           `json:"gender,omitempty"`
	Bio         string           `json:"bio"`
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	DisplayName string           `json:"displayName"`
	Email       string           `json:"email"`
	Avatar      string           `json:"avatar"`
	Phone       string           `json:"phone"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Roles       []RoleInfo       `json:"roles"`
	Permissions []PermissionInfo `json:"permissions"`
}

type RoleInfo struct {
	ID          string           `json:"id"`
	Code        string           `json:"code"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Permissions []PermissionInfo `json:"permissions"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}
