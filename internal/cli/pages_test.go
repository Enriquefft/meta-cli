package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestPagesCommand_Success(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if path != "/me/accounts" {
			t.Errorf("expected path /me/accounts, got %q", path)
		}
		if params.Get("limit") != "25" {
			t.Errorf("expected default limit 25, got %q", params.Get("limit"))
		}
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"pg_1","name":"Test Page","category":"BUSINESS","access_token":"patok"}],"paging":{"next":"next","cursors":{"after":"cursor_1"}}}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 page, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Test Page" {
		t.Errorf("expected page name Test Page, got %q", result.Data[0].Name)
	}
	if !result.HasNext {
		t.Error("expected HasNext=true")
	}
	if result.Cursor != "cursor_1" {
		t.Errorf("expected cursor cursor_1, got %q", result.Cursor)
	}
}

func TestPagesCommand_EmptyResult(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"data":[]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 pages, got %d", len(result.Data))
	}
}

func TestPagesCommand_CustomLimit(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if params.Get("limit") != "5" {
			t.Errorf("expected limit 5, got %q", params.Get("limit"))
		}
		return &meta.Response{
			Body:       []byte(`{"data":[]}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd, "--limit", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 pages, got %d", len(result.Data))
	}
}

func TestPagesCommand_AllFlag(t *testing.T) {
	callCount := 0
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		callCount++
		switch callCount {
		case 1:
			return &meta.Response{
				Body:       []byte(`{"data":[{"id":"pg_1","name":"Page 1","category":"BUSINESS","access_token":"tok1"}],"paging":{"cursors":{"after":"cursor_1"},"next":"/me/accounts?after=cursor_1"}}`),
				StatusCode: 200,
			}, nil
		case 2:
			return &meta.Response{
				Body:       []byte(`{"data":[{"id":"pg_2","name":"Page 2","category":"BUSINESS","access_token":"tok2"}],"paging":{"cursors":{"after":"cursor_2"}}}`),
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
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd, "--all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Page 1" {
		t.Errorf("expected first page name 'Page 1', got %q", result.Data[0].Name)
	}
	if result.Data[1].Name != "Page 2" {
		t.Errorf("expected second page name 'Page 2', got %q", result.Data[1].Name)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}
