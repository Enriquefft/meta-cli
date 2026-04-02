package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestListPages_Success(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			if path != "/me/accounts" {
				t.Fatalf("unexpected path: %s", path)
			}
			if limit := params.Get("limit"); limit != "25" {
				t.Errorf("expected limit=25, got %q", limit)
			}
			expectedFields := "id,name,category,access_token"
			if fields := params.Get("fields"); fields != expectedFields {
				t.Errorf("expected fields=%q, got %q", expectedFields, fields)
			}
			body := `{
				"data": [
					{
						"id": "111",
						"name": "My Store",
						"category": "E-commerce website",
						"access_token": "EAApage123"
					}
				]
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListPages(context.Background(), mock, ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 page, got %d", len(result.Data))
	}
	page := result.Data[0]
	if page.ID != "111" {
		t.Errorf("expected ID '111', got %q", page.ID)
	}
	if page.Name != "My Store" {
		t.Errorf("expected Name 'My Store', got %q", page.Name)
	}
	if page.Category != "E-commerce website" {
		t.Errorf("expected Category 'E-commerce website', got %q", page.Category)
	}
	if page.AccessToken != "EAApage123" {
		t.Errorf("expected AccessToken 'EAApage123', got %q", page.AccessToken)
	}
}

func TestListPages_CustomLimit(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, params url.Values) (*Response, error) {
			if limit := params.Get("limit"); limit != "5" {
				t.Errorf("expected limit=5, got %q", limit)
			}
			body := `{"data":[]}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	_, err := ListPages(context.Background(), mock, ListPagesParams{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPages_EmptyList(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{"data":[]}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListPages(context.Background(), mock, ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 pages, got %d", len(result.Data))
	}
	if result.HasNext {
		t.Error("expected HasNext=false for empty response")
	}
}

func TestListPages_Error(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	result, err := ListPages(context.Background(), mock, ListPagesParams{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

func TestListPages_GraphError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":0,"fbtrace_id":"t1"}}`
			return &Response{Body: []byte(body), StatusCode: http.StatusUnauthorized}, nil
		},
	}

	_, err := ListPages(context.Background(), mock, ListPagesParams{})
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

func TestListPages_Paginated(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{
				"data": [{"id": "1", "name": "Page 1", "category": "Business", "access_token": "tok1"}],
				"paging": {
					"cursors": {"after": "page_cursor"},
					"next": "https://graph.facebook.com/v21.0/me/accounts?after=page_cursor"
				}
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListPages(context.Background(), mock, ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasNext {
		t.Error("expected HasNext=true")
	}
	if result.Cursor != "page_cursor" {
		t.Errorf("expected cursor 'page_cursor', got %q", result.Cursor)
	}
}

func TestListPages_MultiplePages(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			body := `{
				"data": [
					{"id": "1", "name": "Page 1", "category": "Business", "access_token": "tok1"},
					{"id": "2", "name": "Page 2", "category": "E-commerce", "access_token": "tok2"}
				]
			}`
			return &Response{Body: []byte(body), StatusCode: http.StatusOK}, nil
		},
	}

	result, err := ListPages(context.Background(), mock, ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(result.Data))
	}
	if result.Data[1].Name != "Page 2" {
		t.Errorf("expected second page name 'Page 2', got %q", result.Data[1].Name)
	}
}

func TestListPagesResult_JSONSerialization(t *testing.T) {
	result := &ListPagesResult{
		Data: []Page{
			{ID: "1", Name: "P1", Category: "Biz", AccessToken: "tok"},
		},
		HasNext: false,
		Cursor:  "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ListPagesResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(decoded.Data) != 1 {
		t.Fatalf("expected 1 page after round-trip, got %d", len(decoded.Data))
	}
	if decoded.Data[0].AccessToken != "tok" {
		t.Errorf("expected access_token 'tok' after round-trip, got %q", decoded.Data[0].AccessToken)
	}
}
