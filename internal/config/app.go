package config

type App struct {
	Name    string `mapstructure:"name" validate:"required"`
	Env     string `mapstructure:"env" validate:"required"`
	Version string `mapstructure:"version" validate:"required"`
}
