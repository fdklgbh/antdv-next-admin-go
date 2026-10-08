package dto

type PageQuery struct {
	Email    string `form:"email" binding:"max=254"`
	Keyword  string `form:"keyword" binding:"max=128"`
	Current  int    `form:"current" binding:"min=1,max=1000000"`
	PageSize int    `form:"pageSize" binding:"min=1,max=10000"`
	Username string `form:"username" binding:"max=64"`
	Name     string `form:"name" binding:"max=128"`
	Code     string `form:"code" binding:"max=128"`
	Status   string `form:"status" binding:"omitempty,oneof=active inactive"`
	Type     string `form:"type" binding:"omitempty,oneof=menu button api"`
}
type Page[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Current  int   `json:"current"`
	PageSize int   `json:"pageSize"`
}
type UserInput struct {
	Username    string     `json:"username" binding:"required,max=64"`
	DisplayName string     `json:"displayName" binding:"required,max=128"`
	Email       string     `json:"email" binding:"omitempty,email,max=254"`
	Phone       string     `json:"phone" binding:"max=32"`
	Gender      string     `json:"gender" binding:"omitempty,oneof=male female"`
	Bio         string     `json:"bio" binding:"max=200"`
	Status      string     `json:"status" binding:"required,oneof=active inactive"`
	Password    string     `json:"password" binding:"omitempty,min=8,max=72"`
	Roles       []RoleInfo `json:"roles" binding:"required"`
}
type RoleInput struct {
	Name        string           `json:"name" binding:"required,max=128"`
	Code        string           `json:"code" binding:"required,max=64"`
	Description string           `json:"description" binding:"max=255"`
	Permissions []PermissionInfo `json:"permissions" binding:"required"`
}
type PasswordInput struct {
	OldPassword string `json:"oldPassword" binding:"required,max=72"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=72"`
}
