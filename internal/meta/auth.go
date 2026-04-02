package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// AuthUser represents the authenticated user's identity.
type AuthUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AuthApp represents the application associated with the access token.
type AuthApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AuthStatusResult contains the result of validating an access token.
type AuthStatusResult struct {
	Valid       bool     `json:"valid"`
	User        AuthUser `json:"user"`
	App         AuthApp  `json:"app"`
	Permissions []string `json:"permissions"`
	ExpiresAt   string   `json:"expires_at"`
	Scopes      []string `json:"scopes"`
}

// AuthStatus validates the current access token by calling /me and /me/permissions.
// Returns the authenticated user identity and granted permissions.
// The client's token is used implicitly (injected by the transport layer).
func AuthStatus(ctx context.Context, client Client) (*AuthStatusResult, error) {
	// Step 1: GET /me to validate token and get user identity.
	meParams := url.Values{}
	meParams.Set("fields", "id,name")

	meResp, err := client.Get(ctx, "/me", meParams)
	if err != nil {
		return nil, fmt.Errorf("fetching /me: %w", err)
	}

	if ge := ParseGraphError(meResp.Body); ge != nil {
		return nil, ge
	}

	var user AuthUser
	if err := json.Unmarshal(meResp.Body, &user); err != nil {
		return nil, fmt.Errorf("parsing /me response: %w", err)
	}

	// Step 2: GET /me/permissions to list granted permissions.
	permResp, err := client.Get(ctx, "/me/permissions", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching /me/permissions: %w", err)
	}

	if ge := ParseGraphError(permResp.Body); ge != nil {
		return nil, ge
	}

	var permData struct {
		Data []struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(permResp.Body, &permData); err != nil {
		return nil, fmt.Errorf("parsing /me/permissions response: %w", err)
	}

	// Filter to only granted permissions.
	permissions := []string{}
	for _, p := range permData.Data {
		if p.Status == "granted" {
			permissions = append(permissions, p.Permission)
		}
	}

	return &AuthStatusResult{
		Valid:       true,
		User:        user,
		Permissions: permissions,
	}, nil
}
