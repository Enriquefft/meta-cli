package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestTargetingCommand_Success(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if path != "/search" {
			t.Errorf("expected path /search, got %q", path)
		}
		if params.Get("type") != "interests" {
			t.Errorf("expected type interests, got %q", params.Get("type"))
		}
		if params.Get("q") != "shopping" {
			t.Errorf("expected query shopping, got %q", params.Get("q"))
		}
		if params.Get("limit") != "5" {
			t.Errorf("expected limit 5, got %q", params.Get("limit"))
		}
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"tsug_1","name":"Shopping","type":"interests","audience_size_lower_bound":1000,"audience_size_upper_bound":50000,"path":["Interests","Shopping"]}]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewTargetingCommand(deps)
	stdout, _, err := executeCommand(cmd, "--type", "interests", "--query", "shopping", "--limit", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.SearchTargetingResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Shopping" {
		t.Errorf("expected result name Shopping, got %q", result.Data[0].Name)
	}
}

func TestTargetingCommand_MissingType(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewTargetingCommand(deps)

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	_, stderr, _ := executeCommand(cmd, "--query", "shopping")
	if stderr == "" {
		t.Fatal("expected validation error output for missing --type")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestTargetingCommand_MissingQuery(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewTargetingCommand(deps)

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	_, stderr, _ := executeCommand(cmd, "--type", "interests")
	if stderr == "" {
		t.Fatal("expected validation error output for missing --query")
	}
	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}
