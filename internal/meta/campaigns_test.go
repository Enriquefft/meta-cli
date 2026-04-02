package meta

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateCampaign_Success_DailyBudget(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			body, _ := json.Marshal(Campaign{
				ID:          "123456",
				Name:        "Test Campaign",
				Objective:   "OUTCOME_SALES",
				Status:      "PAUSED",
				DailyBudget: "5000",
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Campaign{
				ID:             "123456",
				Name:           "Lifetime Campaign",
				Objective:      "OUTCOME_TRAFFIC",
				Status:         "PAUSED",
				LifetimeBudget: "100000",
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Campaign{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Campaign{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
			body, _ := json.Marshal(Campaign{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
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
