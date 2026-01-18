package shared

import "context"

// Context keys for request-scoped data. Keep types unexported to avoid collisions.
type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request-id"
	ctxKeyPrincipal ctxKey = "principal"
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

