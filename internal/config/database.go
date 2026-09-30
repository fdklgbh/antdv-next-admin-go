package config

import "time"

type Database struct {
	Driver                string        `mapstructure:"driver" validate:"required,oneof=mysql postgres"`
	DSN                   string        `mapstructure:"dsn" validate:"required"`
	MaxOpenConnections    int           `mapstructure:"maxOpenConnections" validate:"gt=0"`
	MaxIdleConnections    int           `mapstructure:"maxIdleConnections" validate:"gte=0,ltefield=MaxOpenConnections"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connectionMaxLifetime" validate:"gt=0"`
}
