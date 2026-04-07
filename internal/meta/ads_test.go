package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// adGetFn returns a GetFn suitable for the MockClient that answers the
// follow-up GET issued by CreateAd after the POST returns only {"id": ...}.
// The returned body mirrors the shape the real Graph API sends for the
// adFields field list.
func adGetFn(t *testing.T, ad Ad) func(ctx context.Context, path string, params url.Values) (*Response, error) {
	t.Helper()
	return func(_ context.Context, path string, params url.Values) (*Response, error) {
		expectedPath := "/" + ad.ID
		if path != expectedPath {
			t.Errorf("expected GET path %s, got %s", expectedPath, path)
		}
		if params.Get("fields") != adFields {
			t.Errorf("expected fields %q, got %q", adFields, params.Get("fields"))
		}
		body, _ := json.Marshal(ad)
		return &Response{Body: body, StatusCode: 200}, nil
	}
}

func TestCreateAd_Success(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			return &Response{Body: []byte(`{"id":"ad_123"}`), StatusCode: 200}, nil
		},
		GetFn: adGetFn(t, Ad{
			ID:       "ad_123",
			Name:     "Test Ad",
			AdSetID:  "adset_456",
			Creative: AdCreative{ID: "cr_789"},
			Status:   "PAUSED",
		}),
	}

	params := CreateAdParams{
		AccountID:  "987654",
		Name:       "Test Ad",
		AdSetID:    "adset_456",
		CreativeID: "cr_789",
	}

	ad, err := CreateAd(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/ads" {
		t.Errorf("expected path /act_987654/ads, got %s", capturedPath)
	}
	if capturedParams["name"] != "Test Ad" {
		t.Errorf("expected name 'Test Ad', got %s", capturedParams["name"])
	}
	if capturedParams["adset_id"] != "adset_456" {
		t.Errorf("expected adset_id 'adset_456', got %s", capturedParams["adset_id"])
	}
	if capturedParams["status"] != "PAUSED" {
		t.Errorf("expected default status 'PAUSED', got %s", capturedParams["status"])
	}

	// Verify creative param is JSON with creative_id
	creativeJSON := capturedParams["creative"]
	if creativeJSON == "" {
		t.Fatal("expected creative param to be set")
	}
	var creativeParam map[string]string
	if err := json.Unmarshal([]byte(creativeJSON), &creativeParam); err != nil {
		t.Fatalf("creative param is not valid JSON: %v", err)
	}
	if creativeParam["creative_id"] != "cr_789" {
		t.Errorf("expected creative_id 'cr_789', got %s", creativeParam["creative_id"])
	}

	if ad.ID != "ad_123" {
		t.Errorf("expected ad ID 'ad_123', got %s", ad.ID)
	}
	if ad.Name != "Test Ad" {
		t.Errorf("expected ad Name 'Test Ad', got %s", ad.Name)
	}
	if ad.AdSetID != "adset_456" {
		t.Errorf("expected ad AdSetID 'adset_456', got %s", ad.AdSetID)
	}
	if ad.Creative.ID != "cr_789" {
		t.Errorf("expected ad Creative.ID 'cr_789', got %s", ad.Creative.ID)
	}
	if ad.Status != "PAUSED" {
		t.Errorf("expected ad Status 'PAUSED', got %s", ad.Status)
	}
}

func TestCreateAd_CustomStatus(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adGetFn(t, Ad{ID: "1"}),
	}

	params := CreateAdParams{
		AccountID:  "123",
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
		Status:     "ACTIVE",
	}

	_, err := CreateAd(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["status"] != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got %s", capturedParams["status"])
	}
}

func TestCreateAd_AccountIDNormalization(t *testing.T) {
	var capturedPath string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			capturedPath = path
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adGetFn(t, Ad{ID: "1"}),
	}

	params := CreateAdParams{
		AccountID:  "act_987654",
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	}

	_, err := CreateAd(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/ads" {
		t.Errorf("expected path /act_987654/ads, got %s", capturedPath)
	}
}

func TestCreateAd_MissingAccountID(t *testing.T) {
	mock := &MockClient{}
	params := CreateAdParams{
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	}
	_, err := CreateAd(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing AccountID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAd_MissingName(t *testing.T) {
	mock := &MockClient{}
	params := CreateAdParams{
		AccountID:  "123",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	}
	_, err := CreateAd(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Name")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAd_MissingAdSetID(t *testing.T) {
	mock := &MockClient{}
	params := CreateAdParams{
		AccountID:  "123",
		Name:       "Test",
		CreativeID: "cr_1",
	}
	_, err := CreateAd(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing AdSetID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAd_MissingCreativeID(t *testing.T) {
	mock := &MockClient{}
	params := CreateAdParams{
		AccountID: "123",
		Name:      "Test",
		AdSetID:   "adset_1",
	}
	_, err := CreateAd(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing CreativeID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAd_DefaultStatusIsPaused(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adGetFn(t, Ad{ID: "1"}),
	}

	params := CreateAdParams{
		AccountID:  "123",
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	}

	_, err := CreateAd(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["status"] != "PAUSED" {
		t.Errorf("expected default status 'PAUSED', got %s", capturedParams["status"])
	}
}

// TestCreateAd_FetchesFullRecordAfterCreate is the regression test for the
// empty-fields bug: Meta's POST /ads returns only {"id": ...}, so CreateAd
// must issue a follow-up GET to enrich the returned Ad with name, adset_id,
// creative, and status fields.
func TestCreateAd_FetchesFullRecordAfterCreate(t *testing.T) {
	var postCalled, getCalled bool

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			postCalled = true
			if path != "/act_88/ads" {
				t.Errorf("expected POST path /act_88/ads, got %s", path)
			}
			return &Response{Body: []byte(`{"id":"ad_full"}`), StatusCode: 200}, nil
		},
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			getCalled = true
			if path != "/ad_full" {
				t.Errorf("expected GET path /ad_full, got %s", path)
			}
			if params.Get("fields") != adFields {
				t.Errorf("expected fields %q, got %q", adFields, params.Get("fields"))
			}
			body := `{"id":"ad_full","name":"Enriched Ad","adset_id":"adset_88","status":"PAUSED","creative":{"id":"cr_88"}}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
	}

	result, err := CreateAd(context.Background(), mock, CreateAdParams{
		AccountID:  "88",
		Name:       "Enriched Ad",
		AdSetID:    "adset_88",
		CreativeID: "cr_88",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Fatal("expected CreateAd to POST")
	}
	if !getCalled {
		t.Fatal("expected CreateAd to issue a follow-up GET for full metadata")
	}
	if result.ID != "ad_full" {
		t.Errorf("expected ID ad_full, got %q", result.ID)
	}
	if result.Name != "Enriched Ad" {
		t.Errorf("expected Name Enriched Ad, got %q", result.Name)
	}
	if result.AdSetID != "adset_88" {
		t.Errorf("expected AdSetID adset_88, got %q", result.AdSetID)
	}
	if result.Status != "PAUSED" {
		t.Errorf("expected Status PAUSED, got %q", result.Status)
	}
	if result.Creative.ID != "cr_88" {
		t.Errorf("expected Creative.ID cr_88, got %q", result.Creative.ID)
	}
}

// TestCreateAd_MissingIDInPostResponse asserts a defensive error is returned
// when the POST response lacks an id.
func TestCreateAd_MissingIDInPostResponse(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{}`), StatusCode: 200}, nil
		},
	}

	_, err := CreateAd(context.Background(), mock, CreateAdParams{
		AccountID:  "1",
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	})
	if err == nil {
		t.Fatal("expected error when POST response omits id")
	}
	if !strings.Contains(err.Error(), "ad id") {
		t.Errorf("expected error to mention missing ad id, got: %v", err)
	}
}

// TestCreateAd_PostErrorPropagated asserts POST-level transport errors
// short-circuit the Create flow and are surfaced to the caller.
func TestCreateAd_PostErrorPropagated(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return nil, fmt.Errorf("dns lookup failed")
		},
	}

	_, err := CreateAd(context.Background(), mock, CreateAdParams{
		AccountID:  "1",
		Name:       "Test",
		AdSetID:    "adset_1",
		CreativeID: "cr_1",
	})
	if err == nil {
		t.Fatal("expected POST error to be propagated")
	}
	if !strings.Contains(err.Error(), "dns lookup failed") {
		t.Errorf("expected original error in chain, got: %v", err)
	}
}
