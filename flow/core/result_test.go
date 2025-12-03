package core

import (
	"strings"
	"testing"
)

func TestErrPanic_Error(t *testing.T) {
	tests := []struct {
		name     string
		panic    ErrPanic
		contains []string
	}{
		{
			name:     "without stack",
			panic:    ErrPanic{Value: "test panic"},
			contains: []string{"panic: test panic"},
		},
		{
			name:     "with stack",
			panic:    ErrPanic{Value: "test panic", Stack: "some/function\n\tfile.go:42"},
			contains: []string{"panic: test panic", "some/function", "file.go:42"},
		},
		{
			name:     "integer value",
			panic:    ErrPanic{Value: 42},
			contains: []string{"panic: 42"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.panic.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(msg, substr) {
					t.Errorf("Error() = %q, want it to contain %q", msg, substr)
				}
			}
		})
	}
}

func TestNewPanicError(t *testing.T) {
	// Create a panic error from inside a function to test stack capture
	var err ErrPanic
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = NewPanicError(r)
			}
		}()
		panic("test panic value")
	}()

	// Check the value was captured
	if err.Value != "test panic value" {
		t.Errorf("Value = %v, want %q", err.Value, "test panic value")
	}

	// Check error message contains the panic value
	errMsg := err.Error()
	if !strings.Contains(errMsg, "panic: test panic value") {
		t.Errorf("Error() = %q, want it to contain 'panic: test panic value'", errMsg)
	}

	// Check that internal min-flow frames are NOT in the stack
	if strings.Contains(err.Stack, "github.com/lguimbarda/min-flow/flow/") {
		t.Errorf("Stack should not contain internal min-flow frames:\n%s", err.Stack)
	}
}

func TestCleanStack(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
		shouldExclude []string
	}{
		{
			name: "removes min-flow frames",
			input: `user/code/main.go
	/path/to/user/code/main.go:10
github.com/lguimbarda/min-flow/flow/core.Map
	/path/to/min-flow/flow/core/map.go:50
testing.tRunner
	/usr/local/go/src/testing/testing.go:1595`,
			shouldContain: []string{"user/code/main.go", "testing.tRunner"},
			shouldExclude: []string{"min-flow/flow/core.Map"},
		},
		{
			name:          "preserves user code",
			input:         "myapp/handler.Process\n\t/home/user/myapp/handler.go:25",
			shouldContain: []string{"myapp/handler.Process", "handler.go:25"},
			shouldExclude: []string{},
		},
		{
			name:          "handles empty input",
			input:         "",
			shouldContain: []string{},
			shouldExclude: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanStack(tt.input)

			for _, s := range tt.shouldContain {
				if !strings.Contains(result, s) {
					t.Errorf("cleanStack() should contain %q, got:\n%s", s, result)
				}
			}

			for _, s := range tt.shouldExclude {
				if strings.Contains(result, s) {
					t.Errorf("cleanStack() should NOT contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}
