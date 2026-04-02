package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ListPagesParams configures the pages listing request.
type ListPagesParams struct {
	Limit int
}

// Page represents a Facebook Page managed by the authenticated user.
type Page struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	AccessToken string `json:"access_token"`
}

// ListPagesResult contains pages and pagination state.
type ListPagesResult struct {
	Data    []Page `json:"data"`
	HasNext bool   `json:"has_next"`
	Cursor  string `json:"cursor"`
}

// ListPages retrieves Facebook Pages the authenticated user manages.
// GET /me/accounts?fields=id,name,category,access_token&limit=N
func ListPages(ctx context.Context, client Client, params ListPagesParams) (*ListPagesResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	qp := url.Values{}
	qp.Set("fields", "id,name,category,access_token")
	qp.Set("limit", strconv.Itoa(limit))

	resp, err := client.Get(ctx, "/me/accounts", qp)
	if err != nil {
		return nil, fmt.Errorf("fetching /me/accounts: %w", err)
	}

	if ge := ParseGraphError(resp.Body); ge != nil {
		return nil, ge
	}

	var raw struct {
		Data   []Page `json:"data"`
		Paging struct {
			Cursors struct {
				After string `json:"after"`
			} `json:"cursors"`
			Next string `json:"next"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("parsing /me/accounts response: %w", err)
	}

	data := raw.Data
	if data == nil {
		data = []Page{}
	}

	return &ListPagesResult{
		Data:    data,
		HasNext: raw.Paging.Next != "",
		Cursor:  raw.Paging.Cursors.After,
	}, nil
}
