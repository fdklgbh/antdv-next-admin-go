package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	App      App      `mapstructure:"app" validate:"required"`
	Server   Server   `mapstructure:"server" validate:"required"`
	Database Database `mapstructure:"database" validate:"required"`
	Redis    Redis    `mapstructure:"redis" validate:"required"`
	Auth     Auth     `mapstructure:"auth" validate:"required"`
	Log      Log      `mapstructure:"log" validate:"required"`
}

func Load(path string) (Config, error) {
	loader := viper.New()
	loader.SetConfigFile(path)
	if err := loader.BindEnv("auth.jwtSecret", "AUTH_JWT_SECRET"); err != nil {
		return Config{}, fmt.Errorf("bind JWT secret: %w", err)
	}

	if err := loader.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := loader.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	configValidator := validator.New(validator.WithRequiredStructEnabled())
	if err := configValidator.Struct(&cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}
