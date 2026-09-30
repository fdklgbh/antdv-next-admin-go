package config

type Log struct {
	Level   string     `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Console LogConsole `mapstructure:"console" validate:"required_without=File.Enabled"`
	File    LogFile    `mapstructure:"file"`
}

type LogConsole struct {
	Enabled bool `mapstructure:"enabled"`
}

type LogFile struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path" validate:"required_if=Enabled true"`
	MaxSizeMB  int    `mapstructure:"maxSizeMB" validate:"required_if=Enabled true,gte=0,lte=10240"`
	MaxBackups int    `mapstructure:"maxBackups" validate:"required_if=Enabled true,gte=0,lte=1000"`
	MaxAgeDays int    `mapstructure:"maxAgeDays" validate:"required_if=Enabled true,gte=0,lte=3650"`
	Compress   bool   `mapstructure:"compress"`
}
