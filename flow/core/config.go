package core

import (
	"context"
)

// Event represents a named occurrence in the stream processing lifecycle.
type Event string

// Stream lifecycle events
const (
	StreamStart Event = "stream:start"
	StreamEnd   Event = "stream:end"
)

// configKey is a typed context key for config injection.
// Each config type gets its own unique key.
type configKey[C any] struct{}

// WithConfig attaches a configuration value to the context.
// The config is keyed by its type, so only one instance of each config type
// can be stored. Later calls with the same type will override earlier ones.
//
// Example:
//
//	ctx := core.WithConfig(ctx, &RetryConfig{MaxRetries: 5})
func WithConfig[C any](ctx context.Context, cfg C) context.Context {
	return context.WithValue(ctx, configKey[C]{}, cfg)
}

// GetConfig retrieves a configuration of type C from the context.
// Returns the config and true if found, or zero value and false if not present.
//
// Example:
//
//	if cfg, ok := core.GetConfig[*RetryConfig](ctx); ok {
//	    maxRetries = cfg.MaxRetries
//	}
func GetConfig[C any](ctx context.Context) (C, bool) {
	if cfg, ok := ctx.Value(configKey[C]{}).(C); ok {
		return cfg, true
	}
	return *new(C), false
}
