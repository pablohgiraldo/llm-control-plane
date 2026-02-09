package observability

import (
	"context"

	"go.uber.org/zap"
)

// Logger provides structured logging with context awareness.
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
}

// Field represents a structured log field.
type Field = zap.Field

// TODO: Implement logger with request ID extraction from context
