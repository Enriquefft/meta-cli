package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestCreativesCommand_Success(t *testing.T) {
	mc := &mockClient{}
	var capturedPath string
	var capturedParams map[string]string
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedPath = path
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"crtv_789","name":"Test Creative"}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewCreativesCommand(deps)
	stdout, _, err := executeCommand(cmd,
		"--name", "Test Creative",
		"--page", "pg_123",
		"--message", "Buy now!",
		"--link", "https://example.com/store",
		"--cta", "SHOP_NOW",
		"--video", "vid_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_123456/adcreatives" {
		t.Errorf("expected path /act_123456/adcreatives, got %q", capturedPath)
	}
	if capturedParams["object_story_spec"] == "" {
		t.Fatal("expected object_story_spec to be set")
	}

	var result meta.Creative
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.ID != "crtv_789" {
		t.Errorf("expected creative ID crtv_789, got %q", result.ID)
	}
}

func TestCreativesCommand_MissingPage(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewCreativesCommand(deps)

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	_, stderr, _ := executeCommand(cmd,
		"--message", "Buy now!",
		"--link", "https://example.com/store",
		"--video", "vid_1")
	if stderr == "" {
		t.Fatal("expected validation error output for missing --page")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestCreativesCommand_MissingLink(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewCreativesCommand(deps)

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	_, stderr, _ := executeCommand(cmd,
		"--page", "pg_123",
		"--message", "Buy now!",
		"--video", "vid_1")
	if stderr == "" {
		t.Fatal("expected validation error output for missing --link")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestCreativesCommand_APIError(t *testing.T) {
	mc := &mockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		return nil, &meta.GraphError{
			Message: "Invalid parameter",
			Type:    "OAuthException",
			Code:    100,
			Subcode: 1885024,
		}
	}

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	deps := testDeps(mc)
	cmd := NewCreativesCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "Test Creative",
		"--page", "pg_123",
		"--message", "Buy now!",
		"--link", "https://example.com/store",
		"--cta", "SHOP_NOW",
		"--video", "vid_456")
	if stderr == "" {
		t.Fatal("expected error output for API error")
	}
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for API error, got %d", exitCode)
	}
}
