package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestAuthCommand_Success(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		switch path {
		case "/me":
			return &meta.Response{
				Body:       []byte(`{"id":"123","name":"Test User"}`),
				StatusCode: 200,
			}, nil
		case "/me/permissions":
			return &meta.Response{
				Body:       []byte(`{"data":[{"permission":"ads_read","status":"granted"}]}`),
				StatusCode: 200,
			}, nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", path)
		}
	}

	deps := testDeps(mc)
	cmd := NewAuthCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.AuthStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid auth status")
	}
	if result.User.ID != "123" {
		t.Errorf("expected user ID '123', got %q", result.User.ID)
	}
	if result.User.Name != "Test User" {
		t.Errorf("expected user name 'Test User', got %q", result.User.Name)
	}
}

func TestAuthCommand_DomainReturnsError(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return nil, &meta.GraphError{
			Message: "Invalid access token",
			Code:    190,
		}
	}

	// Verify domain layer returns proper error (CLI exit is tested via integration).
	_, err := meta.AuthStatus(context.Background(), mc)
	if err == nil {
		t.Fatal("expected error from AuthStatus")
	}

	var ge *meta.GraphError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *meta.GraphError, got %T", err)
	}
	if ge.Code != 190 {
		t.Errorf("expected code 190, got %d", ge.Code)
	}
}
