package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// campaignGetFn returns a GetFn suitable for the MockClient that answers the
// follow-up GET issued by CreateCampaign after the POST returns only
// {"id": ...}. The returned body mirrors the shape the real Graph API sends
// for fields=id,name,objective,status,daily_budget,lifetime_budget,bid_strategy,special_ad_categories.
func campaignGetFn(t *testing.T, campaign Campaign) func(ctx context.Context, path string, params url.Values) (*Response, error) {
	t.Helper()
	return func(_ context.Context, path string, params url.Values) (*Response, error) {
		expectedPath := "/" + campaign.ID
		if path != expectedPath {
			t.Errorf("expected GET path %s, got %s", expectedPath, path)
		}
		if params.Get("fields") != campaignFields {
			t.Errorf("expected fields %q, got %q", campaignFields, params.Get("fields"))
		}
		body, _ := json.Marshal(campaign)
		return &Response{Body: body, StatusCode: 200}, nil
	}
}

func TestCreateCampaign_Success_DailyBudget(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			// Meta's /campaigns endpoint only returns the new campaign's id.
			return &Response{Body: []byte(`{"id":"123456"}`), StatusCode: 200}, nil
		},
		GetFn: campaignGetFn(t, Campaign{
			ID:          "123456",
			Name:        "Test Campaign",
			Objective:   "OUTCOME_SALES",
			Status:      "PAUSED",
			DailyBudget: "5000",
		}),
	}

	params := CreateCampaignParams{
		AccountID:        "987654",
		Name:             "Test Campaign",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
	}

	campaign, err := CreateCampaign(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/campaigns" {
		t.Errorf("expected path /act_987654/campaigns, got %s", capturedPath)
	}
	if capturedParams["name"] != "Test Campaign" {
		t.Errorf("expected name 'Test Campaign', got %s", capturedParams["name"])
	}
	if capturedParams["objective"] != "OUTCOME_SALES" {
		t.Errorf("expected objective OUTCOME_SALES, got %s", capturedParams["objective"])
	}
	if capturedParams["daily_budget"] != "5000" {
		t.Errorf("expected daily_budget '5000', got %s", capturedParams["daily_budget"])
	}
	if _, ok := capturedParams["lifetime_budget"]; ok {
		t.Error("lifetime_budget should not be set when daily_budget is used")
	}
	if capturedParams["status"] != "PAUSED" {
		t.Errorf("expected default status PAUSED, got %s", capturedParams["status"])
	}
	if capturedParams["bid_strategy"] != "LOWEST_COST_WITHOUT_CAP" {
		t.Errorf("expected default bid_strategy LOWEST_COST_WITHOUT_CAP, got %s", capturedParams["bid_strategy"])
	}
	if capturedParams["special_ad_categories"] != `["NONE"]` {
		t.Errorf("expected special_ad_categories [\"NONE\"], got %s", capturedParams["special_ad_categories"])
	}

	if campaign.ID != "123456" {
		t.Errorf("expected campaign ID '123456', got %s", campaign.ID)
	}
	if campaign.Name != "Test Campaign" {
		t.Errorf("expected campaign Name 'Test Campaign', got %s", campaign.Name)
	}
	if campaign.Objective != "OUTCOME_SALES" {
		t.Errorf("expected campaign Objective 'OUTCOME_SALES', got %s", campaign.Objective)
	}
	if campaign.Status != "PAUSED" {
		t.Errorf("expected campaign Status 'PAUSED', got %s", campaign.Status)
	}
}

func TestCreateCampaign_Success_LifetimeBudget(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"123456"}`), StatusCode: 200}, nil
		},
		GetFn: campaignGetFn(t, Campaign{
			ID:             "123456",
			Name:           "Lifetime Campaign",
			Objective:      "OUTCOME_TRAFFIC",
			Status:         "PAUSED",
			LifetimeBudget: "100000",
		}),
	}

	params := CreateCampaignParams{
		AccountID:           "987654",
		Name:                "Lifetime Campaign",
		Objective:           "OUTCOME_TRAFFIC",
		LifetimeBudgetCents: 100000,
	}

	campaign, err := CreateCampaign(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["lifetime_budget"] != "100000" {
		t.Errorf("expected lifetime_budget '100000', got %s", capturedParams["lifetime_budget"])
	}
	if _, ok := capturedParams["daily_budget"]; ok {
		t.Error("daily_budget should not be set when lifetime_budget is used")
	}
	if campaign.LifetimeBudget != "100000" {
		t.Errorf("expected LifetimeBudget '100000', got %s", campaign.LifetimeBudget)
	}
}

func TestCreateCampaign_AccountIDNormalization(t *testing.T) {
	var capturedPath string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			capturedPath = path
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: campaignGetFn(t, Campaign{ID: "1"}),
	}

	params := CreateCampaignParams{
		AccountID:        "act_987654",
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/campaigns" {
		t.Errorf("expected path /act_987654/campaigns, got %s", capturedPath)
	}
}

func TestCreateCampaign_DefaultStatus(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: campaignGetFn(t, Campaign{ID: "1"}),
	}

	params := CreateCampaignParams{
		AccountID:        "123",
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["status"] != "PAUSED" {
		t.Errorf("expected default status PAUSED, got %s", capturedParams["status"])
	}
}

func TestCreateCampaign_CustomStatus(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: campaignGetFn(t, Campaign{ID: "1"}),
	}

	params := CreateCampaignParams{
		AccountID:        "123",
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
		Status:           "ACTIVE",
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["status"] != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", capturedParams["status"])
	}
}

func TestCreateCampaign_MissingAccountID(t *testing.T) {
	mock := &MockClient{}

	params := CreateCampaignParams{
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing AccountID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCampaign_MissingName(t *testing.T) {
	mock := &MockClient{}

	params := CreateCampaignParams{
		AccountID:        "123",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 5000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Name")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCampaign_MissingObjective(t *testing.T) {
	mock := &MockClient{}

	params := CreateCampaignParams{
		AccountID:        "123",
		Name:             "Test",
		DailyBudgetCents: 5000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Objective")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCampaign_NoBudget(t *testing.T) {
	mock := &MockClient{}

	params := CreateCampaignParams{
		AccountID: "123",
		Name:      "Test",
		Objective: "OUTCOME_SALES",
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing budget")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCampaign_BothBudgets(t *testing.T) {
	mock := &MockClient{}

	params := CreateCampaignParams{
		AccountID:           "123",
		Name:                "Test",
		Objective:           "OUTCOME_SALES",
		DailyBudgetCents:    5000,
		LifetimeBudgetCents: 100000,
	}

	_, err := CreateCampaign(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for both budgets set")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

// TestCreateCampaign_FetchesFullRecordAfterCreate is the regression test for
// the empty-fields bug: Meta's POST /campaigns returns only {"id": ...}, so
// CreateCampaign must issue a follow-up GET to enrich the returned Campaign
// with name, objective, status, and budget fields.
func TestCreateCampaign_FetchesFullRecordAfterCreate(t *testing.T) {
	var postCalled, getCalled bool

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			postCalled = true
			if path != "/act_42/campaigns" {
				t.Errorf("expected POST path /act_42/campaigns, got %s", path)
			}
			return &Response{Body: []byte(`{"id":"camp_full"}`), StatusCode: 200}, nil
		},
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			getCalled = true
			if path != "/camp_full" {
				t.Errorf("expected GET path /camp_full, got %s", path)
			}
			if params.Get("fields") != campaignFields {
				t.Errorf("expected fields %q, got %q", campaignFields, params.Get("fields"))
			}
			body := `{"id":"camp_full","name":"Enriched","objective":"OUTCOME_TRAFFIC","status":"PAUSED","daily_budget":"1000"}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
	}

	result, err := CreateCampaign(context.Background(), mock, CreateCampaignParams{
		AccountID:        "42",
		Name:             "Enriched",
		Objective:        "OUTCOME_TRAFFIC",
		DailyBudgetCents: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Fatal("expected CreateCampaign to POST")
	}
	if !getCalled {
		t.Fatal("expected CreateCampaign to issue a follow-up GET for full metadata")
	}
	if result.ID != "camp_full" {
		t.Errorf("expected ID camp_full, got %q", result.ID)
	}
	if result.Name != "Enriched" {
		t.Errorf("expected Name Enriched, got %q", result.Name)
	}
	if result.Objective != "OUTCOME_TRAFFIC" {
		t.Errorf("expected Objective OUTCOME_TRAFFIC, got %q", result.Objective)
	}
	if result.Status != "PAUSED" {
		t.Errorf("expected Status PAUSED, got %q", result.Status)
	}
	if result.DailyBudget != "1000" {
		t.Errorf("expected DailyBudget 1000, got %q", result.DailyBudget)
	}
}

// TestCreateCampaign_MissingIDInPostResponse asserts a defensive error is
// returned when the POST response lacks an id, preventing silent propagation
// of a malformed campaign reference.
func TestCreateCampaign_MissingIDInPostResponse(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{}`), StatusCode: 200}, nil
		},
	}

	_, err := CreateCampaign(context.Background(), mock, CreateCampaignParams{
		AccountID:        "1",
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 1000,
	})
	if err == nil {
		t.Fatal("expected error when POST response omits id")
	}
	if !strings.Contains(err.Error(), "campaign id") {
		t.Errorf("expected error to mention missing campaign id, got: %v", err)
	}
}

// TestCreateCampaign_PostErrorPropagated asserts POST-level transport errors
// short-circuit the Create flow and are surfaced to the caller.
func TestCreateCampaign_PostErrorPropagated(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return nil, fmt.Errorf("network is unreachable")
		},
	}

	_, err := CreateCampaign(context.Background(), mock, CreateCampaignParams{
		AccountID:        "1",
		Name:             "Test",
		Objective:        "OUTCOME_SALES",
		DailyBudgetCents: 1000,
	})
	if err == nil {
		t.Fatal("expected POST error to be propagated")
	}
	if !strings.Contains(err.Error(), "network is unreachable") {
		t.Errorf("expected original error in chain, got: %v", err)
	}
}
