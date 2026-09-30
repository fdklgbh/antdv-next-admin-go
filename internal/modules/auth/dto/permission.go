package dto

type RoutePermission struct {
	Method         string `json:"method" validate:"required,max=10"`
	Route          string `json:"route" validate:"required,max=255"`
	PermissionCode string `json:"permissionCode" validate:"required,max=128"`
	Enabled        bool   `json:"enabled"`
}
