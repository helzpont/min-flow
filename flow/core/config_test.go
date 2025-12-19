package core

import (
	"context"
	"testing"
)

// Test config types

type testConfig struct {
	Value int
}

type otherConfig struct {
	Name string
}

func TestWithConfig(t *testing.T) {
	t.Run("stores config in context", func(t *testing.T) {
		ctx := context.Background()
		cfg := &testConfig{Value: 42}

		newCtx := WithConfig(ctx, cfg)

		if newCtx == ctx {
			t.Error("WithConfig() should return new context")
		}

		got, ok := GetConfig[*testConfig](newCtx)
		if !ok {
			t.Error("GetConfig() returned false, want true")
		}
		if got != cfg {
			t.Errorf("GetConfig() = %v, want %v", got, cfg)
		}
	})

	t.Run("overwrites same type", func(t *testing.T) {
		ctx := context.Background()
		cfg1 := &testConfig{Value: 1}
		cfg2 := &testConfig{Value: 2}

		ctx = WithConfig(ctx, cfg1)
		ctx = WithConfig(ctx, cfg2)

		got, ok := GetConfig[*testConfig](ctx)
		if !ok {
			t.Error("GetConfig() returned false, want true")
		}
		if got.Value != 2 {
			t.Errorf("GetConfig().Value = %d, want 2", got.Value)
		}
	})

	t.Run("different types are independent", func(t *testing.T) {
		ctx := context.Background()
		cfg1 := &testConfig{Value: 42}
		cfg2 := &otherConfig{Name: "test"}

		ctx = WithConfig(ctx, cfg1)
		ctx = WithConfig(ctx, cfg2)

		got1, ok1 := GetConfig[*testConfig](ctx)
		got2, ok2 := GetConfig[*otherConfig](ctx)

		if !ok1 || got1.Value != 42 {
			t.Errorf("testConfig not found or wrong value")
		}
		if !ok2 || got2.Name != "test" {
			t.Errorf("otherConfig not found or wrong value")
		}
	})
}

func TestGetConfig(t *testing.T) {
	t.Run("returns false when not set", func(t *testing.T) {
		ctx := context.Background()
		_, ok := GetConfig[*testConfig](ctx)
		if ok {
			t.Error("GetConfig() returned true for missing config")
		}
	})

	t.Run("returns config when set", func(t *testing.T) {
		ctx := WithConfig(context.Background(), &testConfig{Value: 100})

		got, ok := GetConfig[*testConfig](ctx)
		if !ok {
			t.Error("GetConfig() returned false")
		}
		if got.Value != 100 {
			t.Errorf("got.Value = %d, want 100", got.Value)
		}
	})
}
