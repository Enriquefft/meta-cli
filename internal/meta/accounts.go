package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

const defaultLimit = 25

// ListAccountsParams configures the ad accounts listing request.
type ListAccountsParams struct {
	Limit int
}

// AdAccount represents a Meta ad account.
type AdAccount struct {
	ID            string `json:"id"`
	AccountID     string `json:"account_id"`
	Name          string `json:"name"`
	AccountStatus int    `json:"account_status"`
	Currency      string `json:"currency"`
	TimezoneName  string `json:"timezone_name"`
	AmountSpent   string `json:"amount_spent"`
	Balance       string `json:"balance"`
}

// ListAccountsResult contains ad accounts and pagination state.
type ListAccountsResult struct {
	Data    []AdAccount `json:"data"`
	HasNext bool        `json:"has_next"`
	Cursor  string      `json:"cursor"`
}

// ListAccounts retrieves ad accounts accessible by the authenticated user.
// GET /me/adaccounts?fields=account_id,name,account_status,currency,timezone_name,amount_spent,balance&limit=N
func ListAccounts(ctx context.Context, client Client, params ListAccountsParams) (*ListAccountsResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	qp := url.Values{}
	qp.Set("fields", "account_id,name,account_status,currency,timezone_name,amount_spent,balance")
	qp.Set("limit", strconv.Itoa(limit))

	resp, err := client.Get(ctx, "/me/adaccounts", qp)
	if err != nil {
		return nil, fmt.Errorf("fetching /me/adaccounts: %w", err)
	}

	if ge := ParseGraphError(resp.Body); ge != nil {
		return nil, ge
	}

	var raw struct {
		Data   []AdAccount `json:"data"`
		Paging struct {
			Cursors struct {
				After string `json:"after"`
			} `json:"cursors"`
			Next string `json:"next"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("parsing /me/adaccounts response: %w", err)
	}

	data := raw.Data
	if data == nil {
		data = []AdAccount{}
	}

	return &ListAccountsResult{
		Data:    data,
		HasNext: raw.Paging.Next != "",
		Cursor:  raw.Paging.Cursors.After,
	}, nil
}
