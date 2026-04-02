package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestAccountsCommand_DefaultLimit(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if path != "/me/adaccounts" {
			t.Errorf("expected path /me/adaccounts, got %s", path)
		}
		if params.Get("limit") != "25" {
			t.Errorf("expected limit 25, got %s", params.Get("limit"))
		}
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"act_111","name":"My Account","account_id":"111"}]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAccountsCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListAccountsResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 account, got %d", len(result.Data))
	}
	if result.Data[0].AccountID != "111" {
		t.Errorf("expected account_id '111', got %q", result.Data[0].AccountID)
	}
}

func TestAccountsCommand_CustomLimit(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if params.Get("limit") != "5" {
			t.Errorf("expected limit 5, got %s", params.Get("limit"))
		}
		return &meta.Response{
			Body:       []byte(`{"data":[]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAccountsCommand(deps)
	stdout, _, err := executeCommand(cmd, "--limit", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListAccountsResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(result.Data))
	}
}

func TestAccountsCommand_TableFormat(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"act_111","name":"Test","account_id":"111","currency":"USD"}]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	deps.Format = "table"
	cmd := NewAccountsCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected table output")
	}
}

func TestAccountsCommand_AllFlag(t *testing.T) {
	callCount := 0
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		callCount++
		switch callCount {
		case 1:
			return &meta.Response{
				Body:       []byte(`{"data":[{"id":"act_111","name":"Account 1","account_id":"111"}],"paging":{"cursors":{"after":"cursor_1"},"next":"/me/adaccounts?after=cursor_1"}}`),
				StatusCode: 200,
			}, nil
		case 2:
			return &meta.Response{
				Body:       []byte(`{"data":[{"id":"act_222","name":"Account 2","account_id":"222"}],"paging":{"cursors":{"after":"cursor_2"}}}`),
				StatusCode: 200,
			}, nil
		default:
			return nil, fmt.Errorf("unexpected call %d", callCount)
		}
	}
	mc.pagFn = func(ctx context.Context, path string, params url.Values) *meta.PageIterator {
		return meta.NewPageIterator(mc, path, params)
	}

	deps := testDeps(mc)
	cmd := NewAccountsCommand(deps)
	stdout, _, err := executeCommand(cmd, "--all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListAccountsResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(result.Data))
	}
	if result.Data[0].AccountID != "111" {
		t.Errorf("expected first account_id '111', got %q", result.Data[0].AccountID)
	}
	if result.Data[1].AccountID != "222" {
		t.Errorf("expected second account_id '222', got %q", result.Data[1].AccountID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}
