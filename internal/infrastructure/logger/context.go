package logger

import (
	"context"
	"github.com/rs/zerolog"
	"go-gin-template/internal/shared/contextx"
)

func contextEvent(ctx context.Context, event *zerolog.Event) *zerolog.Event {
	if requestID := contextx.RequestID(ctx); requestID != "" {
		event.Str("request_id", requestID)
	}
	if userID, ok := contextx.UserID(ctx); ok {
		event.Int64("user_id", userID)
	}
	if traceID := contextx.TraceID(ctx); traceID != "" {
		event.Str("trace_id", traceID)
	}
	return event
}
