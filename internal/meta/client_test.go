package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

func makePageResponse(data []string, afterCursor string) *Response {
	body := map[string]interface{}{
		"data": data,
	}
	if afterCursor != "" {
		body["paging"] = map[string]interface{}{
			"cursors": map[string]string{
				"after": afterCursor,
			},
			"next": "https://graph.facebook.com/v21.0/me/posts?after=" + afterCursor,
		}
	}
	b, _ := json.Marshal(body)
	return &Response{Body: b, StatusCode: 200}
}

func TestPageIterator_SinglePage(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return makePageResponse([]string{"a"}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)

	if !it.Next(context.Background()) {
		t.Fatal("expected first Next() to return true")
	}
	resp, err := it.Page()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if it.Next(context.Background()) {
		t.Error("expected second Next() to return false")
	}
	if it.Err() != nil {
		t.Errorf("expected no error, got %v", it.Err())
	}
}

func TestPageIterator_MultiPage(t *testing.T) {
	callCount := 0
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			callCount++
			if callCount == 1 {
				return makePageResponse([]string{"a"}, "https://graph.facebook.com/v21.0/me/posts?after=cursor1"), nil
			}
			return makePageResponse([]string{"b"}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)

	if !it.Next(context.Background()) {
		t.Fatal("expected first Next() to return true")
	}
	resp, _ := it.Page()
	var page1 struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &page1); err != nil {
		t.Fatalf("failed to parse page1: %v", err)
	}
	if len(page1.Data) != 1 || page1.Data[0] != "a" {
		t.Errorf("expected first page data [a], got %v", page1.Data)
	}

	if !it.Next(context.Background()) {
		t.Fatal("expected second Next() to return true")
	}
	resp, _ = it.Page()
	var page2 struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &page2); err != nil {
		t.Fatalf("failed to parse page2: %v", err)
	}
	if len(page2.Data) != 1 || page2.Data[0] != "b" {
		t.Errorf("expected second page data [b], got %v", page2.Data)
	}

	if it.Next(context.Background()) {
		t.Error("expected third Next() to return false")
	}

	if callCount != 2 {
		t.Errorf("expected 2 Get calls, got %d", callCount)
	}
}

func TestPageIterator_MultiPage_PassesCursor(t *testing.T) {
	callCount := 0
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			callCount++
			if callCount == 1 {
				return makePageResponse([]string{"a"}, "cursor1"), nil
			}
			if params.Get("after") != "cursor1" {
				t.Errorf("expected after=cursor1 on second call, got %q", params.Get("after"))
			}
			return makePageResponse([]string{"b"}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)
	for it.Next(context.Background()) {
	}
	if it.Err() != nil {
		t.Fatalf("unexpected error: %v", it.Err())
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestPageIterator_EmptyResult(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return makePageResponse([]string{}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)

	if !it.Next(context.Background()) {
		t.Fatal("expected first Next() to return true")
	}
	resp, err := it.Page()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	if it.Next(context.Background()) {
		t.Error("expected second Next() to return false for empty result")
	}
}

func TestPageIterator_Error(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return nil, fmt.Errorf("network timeout")
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)

	if it.Next(context.Background()) {
		t.Error("expected Next() to return false on error")
	}
	if it.Err() == nil {
		t.Error("expected error from Err()")
	}
}

func TestPageIterator_PageBeforeNext(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return makePageResponse([]string{"a"}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)
	_, err := it.Page()
	if err == nil {
		t.Error("expected error when Page() called before Next()")
	}
}

func TestPageIterator_NilParams(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return makePageResponse([]string{"a"}, ""), nil
		},
	}

	it := NewPageIterator(mock, "/me/posts", nil)
	if !it.Next(context.Background()) {
		t.Fatal("expected Next() to succeed with nil params")
	}
}
