package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigSetCommand(t *testing.T) {
	store, cleanup := testStore()
	defer cleanup()

	mc := &mockClient{}
	deps := testDeps(mc)
	deps.Store = store

	cmd := NewConfigCommand(deps)
	stdout, _, err := executeCommand(cmd, "set", "api_version", "v22.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if result["key"] != "api_version" {
		t.Errorf("expected key 'api_version', got %q", result["key"])
	}
	if result["value"] != "v22.0" {
		t.Errorf("expected value 'v22.0', got %q", result["value"])
	}

	// Verify persistence.
	got := store.Get("api_version")
	if got != "v22.0" {
		t.Errorf("expected persisted 'v22.0', got %q", got)
	}
}

func TestConfigGetCommand(t *testing.T) {
	store, cleanup := testStore()
	defer cleanup()

	_ = store.Set("access_token", "tok_abc")

	mc := &mockClient{}
	deps := testDeps(mc)
	deps.Store = store

	cmd := NewConfigCommand(deps)
	stdout, _, err := executeCommand(cmd, "get", "access_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if result["key"] != "access_token" {
		t.Errorf("expected key 'access_token', got %q", result["key"])
	}
	if result["value"] != "tok_abc" {
		t.Errorf("expected value 'tok_abc', got %q", result["value"])
	}
}

func TestConfigGetCommand_UnsetKey(t *testing.T) {
	store, cleanup := testStore()
	defer cleanup()

	mc := &mockClient{}
	deps := testDeps(mc)
	deps.Store = store

	cmd := NewConfigCommand(deps)
	stdout, _, err := executeCommand(cmd, "get", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if result["value"] != "" {
		t.Errorf("expected empty value, got %q", result["value"])
	}
}

func TestConfigSetCommand_MissingArgs(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	deps.Store, _ = testStore()

	cmd := NewConfigCommand(deps)
	_, _, err := executeCommand(cmd, "set")
	if err == nil {
		t.Error("expected error for missing args")
	}
}

func TestConfigGetCommand_MissingArgs(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	deps.Store, _ = testStore()

	cmd := NewConfigCommand(deps)
	_, _, err := executeCommand(cmd, "get")
	if err == nil {
		t.Error("expected error for missing args")
	}
	if !strings.Contains(err.Error(), "requires") && !strings.Contains(err.Error(), "accepts") {
		t.Errorf("expected arg count error, got: %v", err)
	}
}
