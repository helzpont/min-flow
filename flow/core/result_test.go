package core

import (
	"errors"
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

func TestResult_Ok(t *testing.T) {
	r := Ok(42)

	if !r.IsValue() {
		t.Error("Ok() should return IsValue() = true")
	}
	if r.IsError() {
		t.Error("Ok() should return IsError() = false")
	}
	if r.IsSentinel() {
		t.Error("Ok() should return IsSentinel() = false")
	}
	if r.Value() != 42 {
		t.Errorf("Ok(42).Value() = %d, want 42", r.Value())
	}
	if r.Error() != nil {
		t.Errorf("Ok().Error() = %v, want nil", r.Error())
	}
}

func TestResult_Err(t *testing.T) {
	testErr := errors.New("test error")
	r := Err[int](testErr)

	if r.IsValue() {
		t.Error("Err() should return IsValue() = false")
	}
	if !r.IsError() {
		t.Error("Err() should return IsError() = true")
	}
	if r.IsSentinel() {
		t.Error("Err() should return IsSentinel() = false")
	}
	if r.Error() != testErr {
		t.Errorf("Err().Error() = %v, want %v", r.Error(), testErr)
	}
}

func TestResult_Sentinel(t *testing.T) {
	marker := errors.New("batch marker")
	r := Sentinel[int](marker)

	if r.IsValue() {
		t.Error("Sentinel() should return IsValue() = false")
	}
	if r.IsError() {
		t.Error("Sentinel() should return IsError() = false")
	}
	if !r.IsSentinel() {
		t.Error("Sentinel() should return IsSentinel() = true")
	}
	if r.Error() != nil {
		t.Error("Sentinel().Error() should return nil (error is stored as sentinel)")
	}
	if r.Sentinel() != marker {
		t.Errorf("Sentinel().Sentinel() = %v, want %v", r.Sentinel(), marker)
	}
}

func TestResult_EndOfStream(t *testing.T) {
	r := EndOfStream[int]()

	if r.IsValue() {
		t.Error("EndOfStream() should return IsValue() = false")
	}
	if r.IsError() {
		t.Error("EndOfStream() should return IsError() = false")
	}
	if !r.IsSentinel() {
		t.Error("EndOfStream() should return IsSentinel() = true")
	}
	if r.Sentinel() != ErrEndOfStream {
		t.Errorf("EndOfStream().Sentinel() = %v, want ErrEndOfStream", r.Sentinel())
	}
}

func TestResult_NewResult(t *testing.T) {
	tests := []struct {
		name       string
		value      int
		err        error
		isSentinel bool
		wantValue  bool
		wantError  bool
		wantSent   bool
	}{
		{
			name:       "value result",
			value:      42,
			err:        nil,
			isSentinel: false,
			wantValue:  true,
			wantError:  false,
			wantSent:   false,
		},
		{
			name:       "error result",
			value:      0,
			err:        errors.New("error"),
			isSentinel: false,
			wantValue:  false,
			wantError:  true,
			wantSent:   false,
		},
		{
			name:       "sentinel result",
			value:      0,
			err:        errors.New("marker"),
			isSentinel: true,
			wantValue:  false,
			wantError:  false,
			wantSent:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResult(tt.value, tt.err, tt.isSentinel)

			if r.IsValue() != tt.wantValue {
				t.Errorf("IsValue() = %v, want %v", r.IsValue(), tt.wantValue)
			}
			if r.IsError() != tt.wantError {
				t.Errorf("IsError() = %v, want %v", r.IsError(), tt.wantError)
			}
			if r.IsSentinel() != tt.wantSent {
				t.Errorf("IsSentinel() = %v, want %v", r.IsSentinel(), tt.wantSent)
			}
		})
	}
}

func TestResult_Unwrap(t *testing.T) {
	t.Run("value result", func(t *testing.T) {
		r := Ok(42)
		v, err := r.Unwrap()
		if v != 42 || err != nil {
			t.Errorf("Unwrap() = (%d, %v), want (42, nil)", v, err)
		}
	})

	t.Run("error result", func(t *testing.T) {
		testErr := errors.New("test")
		r := Err[int](testErr)
		v, err := r.Unwrap()
		if v != 0 || err != testErr {
			t.Errorf("Unwrap() = (%d, %v), want (0, %v)", v, err, testErr)
		}
	})

	t.Run("sentinel result", func(t *testing.T) {
		marker := errors.New("marker")
		r := Sentinel[int](marker)
		v, err := r.Unwrap()
		if v != 0 || err != marker {
			t.Errorf("Unwrap() = (%d, %v), want (0, %v)", v, err, marker)
		}
	})
}

func TestResult_Sentinel_NonSentinelReturnsNil(t *testing.T) {
	// Test that Sentinel() returns nil for non-sentinel results
	r := Ok(42)
	if r.Sentinel() != nil {
		t.Errorf("Ok().Sentinel() = %v, want nil", r.Sentinel())
	}

	r2 := Err[int](errors.New("test"))
	if r2.Sentinel() != nil {
		t.Errorf("Err().Sentinel() = %v, want nil", r2.Sentinel())
	}
}
