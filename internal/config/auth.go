package config

import "time"

type Auth struct {
	SuperAdminEmail    string        `mapstructure:"superAdminEmail" validate:"required,email,max=254"`
	SuperAdminUsername string        `mapstructure:"superAdminUsername" validate:"required,min=1,max=64,excludesall= \t\r\n"`
	PermissionCacheTTL time.Duration `mapstructure:"permissionCacheTTL" validate:"gt=0,lte=86400000000000"`
	JWTSecret          string        `mapstructure:"jwtSecret" validate:"required,min=32"`
	JWTIssuer          string        `mapstructure:"jwtIssuer" validate:"required"`
	JWTAudience        string        `mapstructure:"jwtAudience" validate:"required"`
	CookieSecure       *bool         `mapstructure:"cookieSecure" validate:"required"`
	AllowedOrigins     []string      `mapstructure:"allowedOrigins" validate:"required,min=1,dive,http_url"`
}
