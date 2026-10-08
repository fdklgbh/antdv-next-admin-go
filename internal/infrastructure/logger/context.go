package logger

import (
	"context"

	"antdv-next-admin-go/internal/shared/contextx"
)

type loggerContextKey struct{}

func WithLogger(ctx context.Context, log Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, log)
}

func FromContext(ctx context.Context) Logger {
	log, ok := ctx.Value(loggerContextKey{}).(Logger)
	if !ok || log == nil {
		panic("logger: no logger in context")
	}
	fields := make([]Field, 0, 3)
	if requestID := contextx.RequestID(ctx); requestID != "" {
		fields = append(fields, String("request_id", requestID))
	}
	if userID, ok := contextx.UserID(ctx); ok {
		fields = append(fields, Int64("user_id", userID))
	}
	if traceID := contextx.TraceID(ctx); traceID != "" {
		fields = append(fields, String("trace_id", traceID))
	}
	if len(fields) == 0 {
		return log
	}
	return log.With(fields...)
}
