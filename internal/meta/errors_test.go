package meta

import (
	"fmt"
	"math"
	"testing"
)

func TestParseGraphError(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":467,"fbtrace_id":"abc123"}}`)

	ge := ParseGraphError(body)
	if ge == nil {
		t.Fatal("expected GraphError, got nil")
	}
	if ge.Message != "Invalid token" {
		t.Errorf("expected message 'Invalid token', got %s", ge.Message)
	}
	if ge.Code != 190 {
		t.Errorf("expected code 190, got %d", ge.Code)
	}
	if ge.Subcode != 467 {
		t.Errorf("expected subcode 467, got %d", ge.Subcode)
	}
	if ge.IsRetryable {
		t.Error("auth errors should not be retryable")
	}
}

func TestParseGraphError_NoError(t *testing.T) {
	body := []byte(`{"id":"123"}`)
	ge := ParseGraphError(body)
	if ge != nil {
		t.Error("expected nil for non-error response")
	}
}

func TestParseGraphError_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	ge := ParseGraphError(body)
	if ge != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestParseGraphError_Retryable(t *testing.T) {
	body := []byte(`{"error":{"message":"Temp blocked","type":"OAuthException","code":17,"error_subcode":0,"fbtrace_id":"x"}}`)
	ge := ParseGraphError(body)
	if ge == nil {
		t.Fatal("expected GraphError, got nil")
	}
	if !ge.IsRetryable {
		t.Error("code 17 should be retryable")
	}
}

func TestClassifyError_AuthError(t *testing.T) {
	ge := &GraphError{Code: 190}
	code := ClassifyError(ge)
	if code != ExitAuthError {
		t.Errorf("expected ExitAuthError (%d), got %d", ExitAuthError, code)
	}
}

func TestClassifyError_AuthErrorSubcode(t *testing.T) {
	ge := &GraphError{Code: 102, Subcode: 467}
	code := ClassifyError(ge)
	if code != ExitAuthError {
		t.Errorf("expected ExitAuthError (%d) for subcode 467, got %d", ExitAuthError, code)
	}
}

func TestClassifyError_ValidationError(t *testing.T) {
	ge := &GraphError{Code: 100}
	code := ClassifyError(ge)
	if code != ExitValidationError {
		t.Errorf("expected ExitValidationError (%d), got %d", ExitValidationError, code)
	}
}

func TestClassifyError_APIError(t *testing.T) {
	ge := &GraphError{Code: 506}
	code := ClassifyError(ge)
	if code != ExitAPIError {
		t.Errorf("expected ExitAPIError (%d), got %d", ExitAPIError, code)
	}
}

func TestClassifyError_NonGraphError(t *testing.T) {
	code := ClassifyError(errDummy{})
	if code != ExitAPIError {
		t.Errorf("expected ExitAPIError for non-GraphError, got %d", code)
	}
}

func TestClassifyError_WrappedGraphError(t *testing.T) {
	ge := &GraphError{Code: 190}
	wrapped := fmt.Errorf("request failed: %w", ge)
	code := ClassifyError(wrapped)
	if code != ExitAuthError {
		t.Errorf("expected ExitAuthError for wrapped GraphError, got %d", code)
	}
}

func TestClassifyError_NetworkError(t *testing.T) {
	ne := &mockNetError{msg: "connection refused"}
	wrapped := fmt.Errorf("executing request: %w", ne)
	code := ClassifyError(wrapped)
	if code != ExitNetworkError {
		t.Errorf("expected ExitNetworkError (%d), got %d", ExitNetworkError, code)
	}
}

func TestClassifyError_NetworkErrorDirect(t *testing.T) {
	ne := &mockNetError{msg: "timeout"}
	code := ClassifyError(ne)
	if code != ExitNetworkError {
		t.Errorf("expected ExitNetworkError (%d), got %d", ExitNetworkError, code)
	}
}

type errDummy struct{}

func (errDummy) Error() string { return "dummy" }

type mockNetError struct {
	msg string
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return false }
func (e *mockNetError) Temporary() bool { return false }

func TestDollarsToCents(t *testing.T) {
	tests := []struct {
		input  float64
		expect int64
	}{
		{0, 0},
		{50, 5000},
		{1.5, 150},
		{0.01, 1},
		{100.99, 10099},
		{19.99, 1999},
	}
	for _, tt := range tests {
		got, err := DollarsToCents(tt.input)
		if err != nil {
			t.Errorf("DollarsToCents(%f) returned unexpected error: %v", tt.input, err)
		}
		if got != tt.expect {
			t.Errorf("DollarsToCents(%f) = %d, want %d", tt.input, got, tt.expect)
		}
	}
}

func TestDollarsToCents_Negative(t *testing.T) {
	_, err := DollarsToCents(-1)
	if err == nil {
		t.Error("expected error for negative value")
	}
}

func TestDollarsToCents_NaN(t *testing.T) {
	_, err := DollarsToCents(math.NaN())
	if err == nil {
		t.Error("expected error for NaN")
	}
}

func TestDollarsToCents_Inf(t *testing.T) {
	_, err := DollarsToCents(math.Inf(1))
	if err == nil {
		t.Error("expected error for +Inf")
	}
	_, err = DollarsToCents(math.Inf(-1))
	if err == nil {
		t.Error("expected error for -Inf")
	}
}

func TestGraphError_Error(t *testing.T) {
	ge := &GraphError{Message: "something broke", Code: 100, Subcode: 0}
	s := ge.Error()
	if s == "" {
		t.Error("Error() should not be empty")
	}
}
