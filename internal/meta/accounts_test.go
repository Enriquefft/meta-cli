package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestListAccounts_Success(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			if path != "/me/adaccounts" {
				t.Fatalf("unexpected path: %s", path)
			}
			if limit := params.Get("limit"); limit != "25" {
				t.Errorf("expected limit=25, got %q", limit)
			}
			if fields := params.Get("fields"); fields != accountsFields {
				t.Errorf("expected fields=%q, got %q", accountsFields, fields)
			}
			body := `{
				"data": [
					{
						"id": "act_123",
						"account_id": "123",
						"name": "My Store Ads",
						"account_status": 1,
						"currency": "USD",
						"timezone_name": "America/Los_Angeles",
						"amount_spent": "15234",
						"balance": "0"
					}
				]
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result.Data))
	}
	acct := result.Data[0]
	if acct.ID != "act_123" {
		t.Errorf("expected ID 'act_123', got %q", acct.ID)
	}
	if acct.AccountID != "123" {
		t.Errorf("expected AccountID '123', got %q", acct.AccountID)
	}
	if acct.Name != "My Store Ads" {
		t.Errorf("expected Name 'My Store Ads', got %q", acct.Name)
	}
	if acct.AccountStatus != 1 {
		t.Errorf("expected AccountStatus 1, got %d", acct.AccountStatus)
	}
	if acct.Currency != "USD" {
		t.Errorf("expected Currency 'USD', got %q", acct.Currency)
	}
	if acct.TimezoneName != "America/Los_Angeles" {
		t.Errorf("expected TimezoneName 'America/Los_Angeles', got %q", acct.TimezoneName)
	}
	if acct.AmountSpent != "15234" {
		t.Errorf("expected AmountSpent '15234', got %q", acct.AmountSpent)
	}
	if acct.Balance != "0" {
		t.Errorf("expected Balance '0', got %q", acct.Balance)
	}
}

func TestListAccounts_CustomLimit(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, params url.Values) (*Response, error) {
			if limit := params.Get("limit"); limit != "10" {
				t.Errorf("expected limit=10, got %q", limit)
			}
			body := `{"data":[]}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	_, err := ListAccounts(context.Background(), mock, ListAccountsParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAccounts_Paginated(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{
				"data": [{"id": "act_1", "account_id": "1", "name": "A1", "account_status": 1, "currency": "USD", "timezone_name": "UTC", "amount_spent": "0", "balance": "0"}],
				"paging": {
					"cursors": {"after": "cursor_abc"},
					"next": "https://graph.facebook.com/v21.0/me/adaccounts?after=cursor_abc"
				}
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasNext {
		t.Error("expected HasNext=true")
	}
	if result.Cursor != "cursor_abc" {
		t.Errorf("expected cursor 'cursor_abc', got %q", result.Cursor)
	}
}

func TestListAccounts_EmptyList(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{"data":[]}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(result.Data))
	}
	if result.HasNext {
		t.Error("expected HasNext=false for empty response")
	}
}

func TestListAccounts_Error(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	result, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestListAccounts_GraphError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":0,"fbtrace_id":"t1"}}`
			return &Response{Body: []byte(body), StatusCode: http.StatusUnauthorized}, nil
		},
	}

	_, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err == nil {
		t.Fatal("expected error for graph error response")
	}
	ge, ok := err.(*GraphError)
	if !ok {
		t.Fatalf("expected *GraphError, got %T", err)
	}
	if ge.Code != 190 {
		t.Errorf("expected code 190, got %d", ge.Code)
	}
}

func TestListAccounts_MultipleAccounts(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{
				"data": [
					{"id": "act_1", "account_id": "1", "name": "Acc 1", "account_status": 1, "currency": "USD", "timezone_name": "UTC", "amount_spent": "100", "balance": "50"},
					{"id": "act_2", "account_id": "2", "name": "Acc 2", "account_status": 2, "currency": "EUR", "timezone_name": "Europe/Berlin", "amount_spent": "200", "balance": "0"}
				]
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListAccounts(context.Background(), mock, ListAccountsParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(result.Data))
	}
	if result.Data[1].Currency != "EUR" {
		t.Errorf("expected second account currency 'EUR', got %q", result.Data[1].Currency)
	}
}

func TestListAccountsResult_JSONSerialization(t *testing.T) {
	result := &ListAccountsResult{
		Data: []AdAccount{
			{ID: "act_1", AccountID: "1", Name: "Test", AccountStatus: 1, Currency: "USD", TimezoneName: "UTC", AmountSpent: "0", Balance: "0"},
		},
		HasNext: true,
		Cursor:  "abc",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ListAccountsResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(decoded.Data) != 1 {
		t.Fatalf("expected 1 account after round-trip, got %d", len(decoded.Data))
	}
	if !decoded.HasNext {
		t.Error("expected HasNext=true after round-trip")
	}
	if decoded.Cursor != "abc" {
		t.Errorf("expected cursor 'abc' after round-trip, got %q", decoded.Cursor)
	}
}
