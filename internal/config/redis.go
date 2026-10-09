package config

type Redis struct {
	Enabled   bool   `mapstructure:"enabled"`
	KeyPrefix string `mapstructure:"keyPrefix" validate:"required_if=Enabled true"`
	Address   string `mapstructure:"address" validate:"required_if=Enabled true"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Database  int    `mapstructure:"database" validate:"omitempty,gte=0"`
	PoolSize  int    `mapstructure:"poolSize" validate:"required_if=Enabled true,omitempty,gt=0"`
}
