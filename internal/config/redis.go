package config

type Redis struct {
	KeyPrefix string `mapstructure:"keyPrefix" validate:"required"`
	Address   string `mapstructure:"address" validate:"required"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	Database  int    `mapstructure:"database" validate:"gte=0"`
	PoolSize  int    `mapstructure:"poolSize" validate:"gt=0"`
}
