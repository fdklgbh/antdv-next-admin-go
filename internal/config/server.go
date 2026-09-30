package config

import "time"

type Server struct {
	Mode            string        `mapstructure:"mode" validate:"required,oneof=debug release test"`
	Address         string        `mapstructure:"address" validate:"required"`
	StartupTimeout  time.Duration `mapstructure:"startupTimeout" validate:"gt=0"`
	ReadTimeout     time.Duration `mapstructure:"readTimeout" validate:"gt=0"`
	WriteTimeout    time.Duration `mapstructure:"writeTimeout" validate:"gt=0"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout" validate:"gt=0"`
	MaxHeaderBytes  int           `mapstructure:"maxHeaderBytes" validate:"gte=1024,lte=1048576"`
}
