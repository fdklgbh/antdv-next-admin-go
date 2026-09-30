package contextx

import "context"

type key string

const (
	requestIDKey key = "request_id"
	userIDKey    key = "user_id"
	traceIDKey   key = "trace_id"
)

func WithRequestID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, requestIDKey, value)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
func WithUserID(ctx context.Context, value int64) context.Context {
	return context.WithValue(ctx, userIDKey, value)
}
func UserID(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey).(int64)
	return value, ok
}
func WithTraceID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, traceIDKey, value)
}
func TraceID(ctx context.Context) string { value, _ := ctx.Value(traceIDKey).(string); return value }
