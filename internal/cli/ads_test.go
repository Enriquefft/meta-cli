package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestAdsCommand_Success(t *testing.T) {
	mc := &mockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"ad_123","name":"Test Ad","adset_id":"adset_789","creative":{"id":"crtv_111"},"status":"PAUSED"}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAdsCommand(deps)
	stdout, _, err := executeCommand(cmd, "--name", "Test Ad", "--adset", "adset_789", "--creative", "crtv_111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.Ad
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.ID != "ad_123" {
		t.Errorf("expected ad ID 'ad_123', got %q", result.ID)
	}
	if result.AdSetID != "adset_789" {
		t.Errorf("expected adset ID 'adset_789', got %q", result.AdSetID)
	}
}

func TestAdsCommand_MissingName(t *testing.T) {
	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAdsCommand(deps)
	_, stderr, _ := executeCommand(cmd, "--adset", "adset_789", "--creative", "crtv_111")
	if stderr == "" {
		t.Error("expected error output for missing --name")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestAdsCommand_MissingCreative(t *testing.T) {
	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAdsCommand(deps)
	_, stderr, _ := executeCommand(cmd, "--name", "Test Ad", "--adset", "adset_789")
	if stderr == "" {
		t.Error("expected error output for missing --creative")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestAdsCommand_APIError(t *testing.T) {
	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		return nil, &meta.GraphError{
			Message: "Invalid parameter",
			Type:    "OAuthException",
			Code:    100,
			Subcode: 1885024,
		}
	}

	deps := testDeps(mc)
	cmd := NewAdsCommand(deps)
	_, stderr, _ := executeCommand(cmd, "--name", "Test", "--adset", "a", "--creative", "c")
	if stderr == "" {
		t.Error("expected error output for API error")
	}
	if exitCode == 0 {
		t.Errorf("expected non-zero exit code for API error, got %d", exitCode)
	}
}
