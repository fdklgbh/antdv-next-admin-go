package config

import "time"

type Auth struct {
	PermissionCacheTTL time.Duration `mapstructure:"permissionCacheTTL" validate:"gt=0,lte=86400000000000"`
}
