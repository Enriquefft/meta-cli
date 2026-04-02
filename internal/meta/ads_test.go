package meta

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateAd_Success(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			body, _ := json.Marshal(Ad{
				ID:       "ad_123",
				Name:     "Test Ad",
				AdSetID:  "adset_456",
				Creative: AdCreative{ID: "cr_789"},
				Status:   "PAUSED",
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Ad{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Ad{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Ad{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
