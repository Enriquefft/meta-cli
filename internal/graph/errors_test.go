package graph

import (
	"testing"

	meta "github.com/enriquefft/meta-cli/internal/meta"
)

func TestCheckResponse_2xxClean(t *testing.T) {
	resp := &meta.Response{
		StatusCode: 200,
		Body:       []byte(`{"id":"123"}`),
	}
	if err := CheckResponse(resp); err != nil {
		t.Errorf("expected no error for clean 2xx, got: %v", err)
	}
}

func TestCheckResponse_2xxWithGraphError(t *testing.T) {
	resp := &meta.Response{
		StatusCode: 200,
		Body:       []byte(`{"error":{"message":"Temp blocked","type":"OAuthException","code":17}}`),
	}
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected error for 2xx with graph error in body")
	}
	ge, ok := err.(*meta.GraphError)
	if !ok {
		t.Fatalf("expected *meta.GraphError, got %T", err)
	}
	if ge.Code != 17 {
		t.Errorf("expected code 17, got %d", ge.Code)
	}
}

func TestCheckResponse_NonRetryableHTTPWithGraphError(t *testing.T) {
	resp := &meta.Response{
		StatusCode: 400,
		Body:       []byte(`{"error":{"message":"Invalid param","type":"OAuthException","code":100}}`),
	}
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	ge, ok := err.(*meta.GraphError)
	if !ok {
		t.Fatalf("expected *meta.GraphError, got %T", err)
	}
	if ge.Code != 100 {
		t.Errorf("expected code 100, got %d", ge.Code)
	}
}

func TestCheckResponse_NonRetryableHTTPWithoutGraphError(t *testing.T) {
	resp := &meta.Response{
		StatusCode: 502,
		Body:       []byte(`Bad Gateway`),
	}
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected error for 502")
	}
	if _, ok := err.(*meta.GraphError); ok {
		t.Error("expected generic error, not GraphError, for non-JSON body")
	}
}

func TestCheckResponse_401AuthError(t *testing.T) {
	resp := &meta.Response{
		StatusCode: 401,
		Body:       []byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":467}}`),
	}
	err := CheckResponse(resp)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	ge, ok := err.(*meta.GraphError)
	if !ok {
		t.Fatalf("expected *meta.GraphError, got %T", err)
	}
	if ge.Code != 190 {
		t.Errorf("expected code 190, got %d", ge.Code)
	}
	if ge.Subcode != 467 {
		t.Errorf("expected subcode 467, got %d", ge.Subcode)
	}
}

func TestIsRetryableHTTP(t *testing.T) {
	retryable := []int{429, 500, 502, 503}
	for _, code := range retryable {
		if !IsRetryableHTTP(code) {
			t.Errorf("expected %d to be retryable", code)
		}
	}

	notRetryable := []int{200, 201, 400, 401, 403, 404, 409, 422}
	for _, code := range notRetryable {
		if IsRetryableHTTP(code) {
			t.Errorf("expected %d to NOT be retryable", code)
		}
	}
}
