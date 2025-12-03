package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

func TestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()
	ctx := context.Background()
	stream := Get(server.URL)
	var results []Response
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 response, got %d", len(results))
	}
	resp := results[0]
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "Hello, World!" {
		t.Errorf("expected body 'Hello, World!', got %q", string(resp.Body))
	}
}

func TestGetLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("line1\nline2\nline3\n"))
	}))
	defer server.Close()
	ctx := context.Background()
	stream := GetLines(server.URL)
	var lines []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		lines = append(lines, res.Value())
	}
	expected := []string{"line1", "line2", "line3"}
	if len(lines) != len(expected) {
		t.Errorf("expected %d lines, got %d", len(expected), len(lines))
	}
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestGetBytes(t *testing.T) {
	content := "Hello, World! This is a test."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer server.Close()
	ctx := context.Background()
	stream := GetBytes(server.URL, 5)
	var result []byte
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		result = append(result, res.Value()...)
	}
	if string(result) != content {
		t.Errorf("expected %q, got %q", content, string(result))
	}
}

func TestGetEach(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response1"))
	}))
	defer server1.Close()
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response2"))
	}))
	defer server2.Close()
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 2)
		out <- core.Ok(server1.URL)
		out <- core.Ok(server2.URL)
		close(out)
		return out
	})
	output := GetEach(http.DefaultClient).Apply(ctx, input)
	var responses []Response
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		responses = append(responses, res.Value())
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
}

func TestGet_InvalidURL(t *testing.T) {
	ctx := context.Background()
	stream := Get("http://invalid.invalid.invalid")
	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError() {
		t.Error("expected error for invalid URL")
	}
}

func TestGet_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := Get(server.URL)
	results := stream.Collect(ctx)
	// When context is already cancelled, the stream may emit an error or nothing
	// depending on timing - both are acceptable behaviors
	if len(results) > 0 && !results[0].IsError() {
		t.Error("if a result is emitted, it should be an error for cancelled context")
	}
}

func TestRequest_WithBody(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Write([]byte("OK"))
	}))
	defer server.Close()
	ctx := context.Background()
	body := strings.NewReader("test body")
	stream := Request(http.DefaultClient, http.MethodPost, server.URL, body)
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
	}
	if receivedBody != "test body" {
		t.Errorf("expected 'test body', got %q", receivedBody)
	}
}
