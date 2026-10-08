package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"antdv-next-admin-go/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

func buildWriter(cfg config.Log) (io.Writer, func() error, error) {
	writers := make([]io.Writer, 0, 2)
	if cfg.Console.Enabled {
		writers = append(writers, os.Stdout)
	}
	if !cfg.File.Enabled {
		return io.MultiWriter(writers...), func() error { return nil }, nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.File.Path), 0o750); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(cfg.File.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, nil, fmt.Errorf("validate log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, nil, fmt.Errorf("close log validation file: %w", err)
	}
	rotator := &lumberjack.Logger{Filename: cfg.File.Path, MaxSize: cfg.File.MaxSizeMB, MaxBackups: cfg.File.MaxBackups, MaxAge: cfg.File.MaxAgeDays, Compress: cfg.File.Compress}
	writers = append(writers, rotator)
	return io.MultiWriter(writers...), rotator.Close, nil
}
