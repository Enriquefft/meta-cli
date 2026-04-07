package meta

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// fetchAfterCreateTarget is a minimal struct used by the helper tests as a
// decode target. It matches the shape of a generic Meta resource record.
type fetchAfterCreateTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func TestFetchAfterCreate_HappyPath(t *testing.T) {
	const wantFields = "id,name,status"

	mock := &MockClient{
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			if path != "/res_42" {
				t.Errorf("expected GET path /res_42, got %s", path)
			}
			if params.Get("fields") != wantFields {
				t.Errorf("expected fields %q, got %q", wantFields, params.Get("fields"))
			}
			body := []byte(`{"id":"res_42","name":"Enriched","status":"ACTIVE"}`)
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		mock,
		[]byte(`{"id":"res_42"}`),
		"resource",
		wantFields,
		&dst,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != "res_42" {
		t.Errorf("expected ID res_42, got %q", dst.ID)
	}
	if dst.Name != "Enriched" {
		t.Errorf("expected Name Enriched, got %q", dst.Name)
	}
	if dst.Status != "ACTIVE" {
		t.Errorf("expected Status ACTIVE, got %q", dst.Status)
	}
}

func TestFetchAfterCreate_MissingID(t *testing.T) {
	// GET must not be called when the id is missing — the helper must fail
	// fast with a defensive error.
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			t.Fatal("GetFn should not be invoked when post body lacks id")
			return nil, nil
		},
	}

	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		mock,
		[]byte(`{"other":"value"}`),
		"campaign",
		"id,name",
		&dst,
	)
	if err == nil {
		t.Fatal("expected error when post body lacks id")
	}
	if !strings.Contains(err.Error(), "campaign id") {
		t.Errorf("expected error to mention missing campaign id, got: %v", err)
	}
}

func TestFetchAfterCreate_MissingID_EmptyString(t *testing.T) {
	// An explicit empty-string id is also invalid (Meta should never return
	// this, but guard against propagation of a malformed reference).
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			t.Fatal("GetFn should not be invoked when post body has empty id")
			return nil, nil
		},
	}

	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		mock,
		[]byte(`{"id":""}`),
		"ad",
		"id,name",
		&dst,
	)
	if err == nil {
		t.Fatal("expected error when post body has empty id")
	}
	if !strings.Contains(err.Error(), "ad id") {
		t.Errorf("expected error to mention missing ad id, got: %v", err)
	}
}

func TestFetchAfterCreate_InvalidPostBody(t *testing.T) {
	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		&MockClient{},
		[]byte(`not json`),
		"resource",
		"id",
		&dst,
	)
	if err == nil {
		t.Fatal("expected error when post body is not valid JSON")
	}
	if !strings.Contains(err.Error(), "parsing create response") {
		t.Errorf("expected error to mention parsing create response, got: %v", err)
	}
}

func TestFetchAfterCreate_GetError(t *testing.T) {
	transportErr := fmt.Errorf("transport: EOF")
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			return nil, transportErr
		},
	}

	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		mock,
		[]byte(`{"id":"res_1"}`),
		"resource",
		"id,name",
		&dst,
	)
	if err == nil {
		t.Fatal("expected error when GET fails at transport level")
	}
	if !errors.Is(err, transportErr) {
		// Helper returns the error verbatim so identity should hold.
		if err.Error() != transportErr.Error() {
			t.Errorf("expected transport error to be returned, got: %v", err)
		}
	}
}

func TestFetchAfterCreate_DecodeError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(_ context.Context, _ string, _ url.Values) (*Response, error) {
			// Return a payload where "id" is a number — incompatible with
			// the string ID field on fetchAfterCreateTarget.
			return &Response{Body: []byte(`{"id": 123}`), StatusCode: 200}, nil
		},
	}

	var dst fetchAfterCreateTarget
	err := fetchAfterCreate(
		context.Background(),
		mock,
		[]byte(`{"id":"res_1"}`),
		"creative",
		"id,name",
		&dst,
	)
	if err == nil {
		t.Fatal("expected decode error from GET response")
	}
	if !strings.Contains(err.Error(), "parsing creative response") {
		t.Errorf("expected error to mention parsing creative response, got: %v", err)
	}
}
