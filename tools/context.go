package tools

import (
	"context"

	"github.com/hamstah/gomcp/types"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// loggerContextKey is the key used to store the logger in the context
var loggerKey = contextKey("logger")

func makeContextWithLogger(ctx context.Context, logger types.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func GetLogger(ctx context.Context) types.Logger {
	return ctx.Value(loggerKey).(types.Logger)
}
