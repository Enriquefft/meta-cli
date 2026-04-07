package meta

import (
	"fmt"
	"math"
	"strings"
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

func TestParseGraphError_UserTitleAndMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid parameter","type":"OAuthException","code":100,"error_subcode":1443226,"error_user_title":"Your ad needs a video thumbnail","error_user_msg":"Please specify one of image_hash or image_url in the video_data field of object_story_spec.","fbtrace_id":"A_4_hqeC6Ihn23dP1sxHqfg"}}`)

	ge := ParseGraphError(body)
	if ge == nil {
		t.Fatal("expected GraphError, got nil")
	}
	if ge.Message != "Invalid parameter" {
		t.Errorf("expected message 'Invalid parameter', got %q", ge.Message)
	}
	if ge.Code != 100 {
		t.Errorf("expected code 100, got %d", ge.Code)
	}
	if ge.Subcode != 1443226 {
		t.Errorf("expected subcode 1443226, got %d", ge.Subcode)
	}
	if ge.UserTitle != "Your ad needs a video thumbnail" {
		t.Errorf("expected UserTitle populated, got %q", ge.UserTitle)
	}
	const wantMsg = "Please specify one of image_hash or image_url in the video_data field of object_story_spec."
	if ge.UserMessage != wantMsg {
		t.Errorf("expected UserMessage %q, got %q", wantMsg, ge.UserMessage)
	}
	if ge.TraceID != "A_4_hqeC6Ihn23dP1sxHqfg" {
		t.Errorf("expected trace id preserved, got %q", ge.TraceID)
	}
}

func TestParseGraphError_NoUserFields(t *testing.T) {
	body := []byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":467,"fbtrace_id":"abc"}}`)
	ge := ParseGraphError(body)
	if ge == nil {
		t.Fatal("expected GraphError, got nil")
	}
	if ge.UserTitle != "" {
		t.Errorf("expected empty UserTitle, got %q", ge.UserTitle)
	}
	if ge.UserMessage != "" {
		t.Errorf("expected empty UserMessage, got %q", ge.UserMessage)
	}
}

func TestGraphError_ErrorString_IncludesUserFields(t *testing.T) {
	ge := &GraphError{
		Message:     "Invalid parameter",
		Code:        100,
		Subcode:     1443226,
		TraceID:     "trace-xyz",
		UserTitle:   "Your ad needs a video thumbnail",
		UserMessage: "Please specify one of image_hash or image_url.",
	}
	s := ge.Error()

	if !strings.Contains(s, "Your ad needs a video thumbnail") {
		t.Errorf("Error() should include UserTitle; got %q", s)
	}
	if !strings.Contains(s, "Please specify one of image_hash or image_url.") {
		t.Errorf("Error() should include UserMessage; got %q", s)
	}
	if !strings.Contains(s, "code=100") {
		t.Errorf("Error() should include code; got %q", s)
	}
	if !strings.Contains(s, "subcode=1443226") {
		t.Errorf("Error() should include subcode; got %q", s)
	}
	if !strings.Contains(s, "trace=trace-xyz") {
		t.Errorf("Error() should include trace id; got %q", s)
	}
}

func TestGraphError_ErrorString_OmitsSeparatorWhenNoUserFields(t *testing.T) {
	ge := &GraphError{Message: "Invalid token", Code: 190, Subcode: 467}
	s := ge.Error()

	if strings.Contains(s, " — ") {
		t.Errorf("Error() should not include the em-dash separator when user fields are empty; got %q", s)
	}
	if !strings.Contains(s, "Invalid token") {
		t.Errorf("Error() should include message; got %q", s)
	}
	if !strings.Contains(s, "code=190") {
		t.Errorf("Error() should include code; got %q", s)
	}
}

func TestGraphError_ErrorString_UserTitleOnly(t *testing.T) {
	ge := &GraphError{Message: "Invalid parameter", Code: 100, UserTitle: "Bad thumbnail"}
	s := ge.Error()
	if !strings.Contains(s, "Bad thumbnail") {
		t.Errorf("Error() should include UserTitle alone; got %q", s)
	}
}

func TestGraphError_ErrorString_UserMessageOnly(t *testing.T) {
	ge := &GraphError{Message: "Invalid parameter", Code: 100, UserMessage: "Specify image_hash."}
	s := ge.Error()
	if !strings.Contains(s, "Specify image_hash.") {
		t.Errorf("Error() should include UserMessage alone; got %q", s)
	}
}
