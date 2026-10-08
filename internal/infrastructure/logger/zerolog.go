package logger

import (
	"fmt"

	"antdv-next-admin-go/internal/config"

	"github.com/rs/zerolog"
)

type zerologAdapter struct{ logger zerolog.Logger }

func New(cfg config.Log, service, environment, version string) (Logger, func() error, error) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("parse log level: %w", err)
	}
	writer, closeWriter, err := buildWriter(cfg)
	if err != nil {
		return nil, nil, err
	}
	base := zerolog.New(writer).Level(level).With().Timestamp().Str("service", service).Str("environment", environment).Str("version", version).Logger()
	return &zerologAdapter{logger: base}, closeWriter, nil
}
func (l *zerologAdapter) Debug(message string, fields ...Field) {
	write(l.logger.Debug(), message, fields)
}
func (l *zerologAdapter) Info(message string, fields ...Field) {
	write(l.logger.Info(), message, fields)
}
func (l *zerologAdapter) Warn(message string, fields ...Field) {
	write(l.logger.Warn(), message, fields)
}
func (l *zerologAdapter) Error(message string, fields ...Field) {
	write(l.logger.Error(), message, fields)
}
func (l *zerologAdapter) With(fields ...Field) Logger {
	ctx := l.logger.With()
	for _, field := range fields {
		ctx = ctx.Interface(field.Key, field.Value)
	}
	return &zerologAdapter{logger: ctx.Logger()}
}
func write(event *zerolog.Event, message string, fields []Field) {
	for _, field := range fields {
		if err, ok := field.Value.(error); ok {
			event.Err(err)
			if field.Key != "error" {
				event.Str("error_field", field.Key)
			}
			continue
		}
		event.Interface(field.Key, field.Value)
	}
	event.Msg(message)
}
