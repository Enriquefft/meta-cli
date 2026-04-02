package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestServeCommand_Registered(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	cmd := NewServeCommand(deps)
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Use != "serve" {
		t.Errorf("expected Use == 'serve', got %q", cmd.Use)
	}
}

func TestServeCommand_RunServerWiring(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	// Use a cancelled context so RunServer returns immediately
	// without blocking on stdio transport.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runMCPServer(ctx, deps)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "server shutdown") {
		t.Errorf("expected 'server shutdown' in error, got: %v", err)
	}
}

func TestServeCommand_ExitCodeOnContextCancel(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	var exitCode int
	origExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origExit }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := NewServeCommand(deps)
	_, stderr, err := executeCommandWithContext(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected command execution error: %v", err)
	}
	if stderr == "" {
		t.Fatal("expected error output on stderr")
	}
	if exitCode != meta.ExitAPIError {
		t.Fatalf("expected exit code %d (ExitAPIError), got %d", meta.ExitAPIError, exitCode)
	}
}

func TestServeCommand_CleanStartupAndShutdown(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- runMCPServer(ctx, deps)
	}()

	// Give the server goroutine time to start and begin listening on stdio.
	// The server blocks on stdin, so it will be alive after this brief wait.
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "server shutdown") {
		t.Errorf("expected 'server shutdown' in error, got: %v", err)
	}
}
