package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestAuthStatus_Success(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			switch path {
			case "/me":
				body := `{"id":"123","name":"Test User"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				body := `{"data":[{"permission":"ads_management","status":"granted"},{"permission":"ads_read","status":"granted"},{"permission":"email","status":"declined"}]}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/debug_token":
				body := `{"data":{"app_id":"app_1","application":"TestApp","expires_at":1713139200,"scopes":"ads_management,ads_read"}}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	result, err := AuthStatus(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.User.ID != "123" {
		t.Errorf("expected user ID '123', got %q", result.User.ID)
	}
	if result.User.Name != "Test User" {
		t.Errorf("expected user name 'Test User', got %q", result.User.Name)
	}
	if len(result.Permissions) != 2 {
		t.Fatalf("expected 2 granted permissions, got %d", len(result.Permissions))
	}
	if result.Permissions[0] != "ads_management" {
		t.Errorf("expected first permission 'ads_management', got %q", result.Permissions[0])
	}
	if result.Permissions[1] != "ads_read" {
		t.Errorf("expected second permission 'ads_read', got %q", result.Permissions[1])
	}
	if result.App.ID != "app_1" {
		t.Errorf("expected app ID 'app_1', got %q", result.App.ID)
	}
	if result.App.Name != "TestApp" {
		t.Errorf("expected app name 'TestApp', got %q", result.App.Name)
	}
	if result.ExpiresAt != "1713139200" {
		t.Errorf("expected expires_at '1713139200', got %q", result.ExpiresAt)
	}
	if len(result.Scopes) != 2 || result.Scopes[0] != "ads_management" || result.Scopes[1] != "ads_read" {
		t.Errorf("expected scopes [ads_management, ads_read], got %v", result.Scopes)
	}
}

func TestAuthStatus_MeError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			if path == "/me" {
				return nil, fmt.Errorf("network error")
			}
			t.Fatalf("unexpected path: %s", path)
			return nil, nil
		},
	}

	result, err := AuthStatus(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestAuthStatus_MeGraphError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			if path == "/me" {
				body := `{"error":{"message":"Invalid OAuth access token.","type":"OAuthException","code":190,"error_subcode":467,"fbtrace_id":"trace1"}}`
				return &Response{Body: []byte(body), StatusCode: http.StatusUnauthorized}, nil
			}
			t.Fatalf("unexpected path: %s", path)
			return nil, nil
		},
	}

	_, err := AuthStatus(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	ge, ok := err.(*GraphError)
	if !ok {
		t.Fatalf("expected *GraphError, got %T", err)
	}
	if ge.Code != 190 {
		t.Errorf("expected error code 190, got %d", ge.Code)
	}
}

func TestAuthStatus_PermissionsError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			switch path {
			case "/me":
				body := `{"id":"123","name":"Test User"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				return nil, fmt.Errorf("permissions fetch failed")
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	_, err := AuthStatus(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error when permissions fetch fails")
	}
}

func TestAuthStatus_PermissionsGraphError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			switch path {
			case "/me":
				body := `{"id":"123","name":"Test User"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				body := `{"error":{"message":"Requires extended permission","type":"OAuthException","code":200,"error_subcode":0,"fbtrace_id":"trace2"}}`
				return &Response{Body: []byte(body), StatusCode: http.StatusForbidden}, nil
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	_, err := AuthStatus(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error for permissions graph error")
	}
	ge, ok := err.(*GraphError)
	if !ok {
		t.Fatalf("expected *GraphError, got %T", err)
	}
	if ge.Code != 200 {
		t.Errorf("expected error code 200, got %d", ge.Code)
	}
}

func TestAuthStatus_MixedPermissions(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			switch path {
			case "/me":
				body := `{"id":"1","name":"U"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				body := `{"data":[{"permission":"ads_management","status":"granted"},{"permission":"email","status":"declined"},{"permission":"ads_read","status":"granted"},{"permission":"pages_read","status":"expired"}]}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/debug_token":
				return nil, fmt.Errorf("debug_token unavailable")
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	result, err := AuthStatus(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Permissions) != 2 {
		t.Fatalf("expected 2 granted permissions, got %d: %v", len(result.Permissions), result.Permissions)
	}
	expected := []string{"ads_management", "ads_read"}
	for i, perm := range expected {
		if result.Permissions[i] != perm {
			t.Errorf("permission[%d]: expected %q, got %q", i, perm, result.Permissions[i])
		}
	}
}

func TestAuthStatus_EmptyPermissions(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, _ url.Values) (*Response, error) {
			switch path {
			case "/me":
				body := `{"id":"1","name":"U"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				body := `{"data":[]}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/debug_token":
				return nil, fmt.Errorf("debug_token unavailable")
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	result, err := AuthStatus(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Permissions == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result.Permissions) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(result.Permissions))
	}
}

func TestAuthStatus_MeFieldsParsed(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			switch path {
			case "/me":
				if fields := params.Get("fields"); fields != "id,name" {
					t.Errorf("expected fields=id,name, got %q", fields)
				}
				body := `{"id":"99","name":"Full Name"}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/me/permissions":
				body := `{"data":[]}`
				return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
			case "/debug_token":
				return nil, fmt.Errorf("debug_token unavailable")
			default:
				t.Fatalf("unexpected path: %s", path)
				return nil, nil
			}
		},
	}

	result, err := AuthStatus(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.ID != "99" || result.User.Name != "Full Name" {
		t.Errorf("user fields not parsed correctly: %+v", result.User)
	}
}

func TestAuthStatusResult_JSONSerialization(t *testing.T) {
	result := &AuthStatusResult{
		Valid:       true,
		User:        AuthUser{ID: "1", Name: "Test"},
		Permissions: []string{"ads_management"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AuthStatusResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Valid != true {
		t.Error("expected Valid=true after round-trip")
	}
	if decoded.User.ID != "1" {
		t.Errorf("expected user ID '1' after round-trip, got %q", decoded.User.ID)
	}
}
