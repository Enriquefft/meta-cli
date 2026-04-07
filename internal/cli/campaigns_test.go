package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestCampaignsCommand_Success(t *testing.T) {
	mc := &mockClient{}
	var capturedPath string
	var capturedParams map[string]string
	// Meta's POST /campaigns only returns the new campaign's id. CreateCampaign
	// chains a follow-up GET to enrich the returned record; wire up both.
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedPath = path
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"camp_789"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"camp_789","name":"Test Campaign","objective":"OUTCOME_SALES","status":"PAUSED","daily_budget":"5000"}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewCampaignsCommand(deps)
	stdout, _, err := executeCommand(cmd,
		"--name", "Test Campaign",
		"--objective", "OUTCOME_SALES",
		"--daily-budget", "50",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_123456/campaigns" {
		t.Errorf("expected path /act_123456/campaigns, got %q", capturedPath)
	}
	if capturedParams["name"] != "Test Campaign" {
		t.Errorf("expected name Test Campaign, got %q", capturedParams["name"])
	}
	if capturedParams["objective"] != "OUTCOME_SALES" {
		t.Errorf("expected objective OUTCOME_SALES, got %q", capturedParams["objective"])
	}
	if capturedParams["daily_budget"] != "5000" {
		t.Errorf("expected daily_budget 5000, got %q", capturedParams["daily_budget"])
	}

	var result meta.Campaign
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.ID != "camp_789" {
		t.Errorf("expected campaign ID camp_789, got %q", result.ID)
	}
	if result.Name != "Test Campaign" {
		t.Errorf("expected campaign name Test Campaign, got %q", result.Name)
	}
}

func TestCampaignsCommand_MissingName(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewCampaignsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--objective", "OUTCOME_SALES",
		"--daily-budget", "5",
	)
	if stderr == "" {
		t.Fatal("expected validation error output for missing --name")
	}
}

func TestCampaignsCommand_MissingObjective(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewCampaignsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "Test Campaign",
		"--daily-budget", "5",
	)
	if stderr == "" {
		t.Fatal("expected validation error output for missing --objective")
	}
}

func TestCampaignsCommand_LifetimeBudget(t *testing.T) {
	mc := &mockClient{}
	var capturedParams map[string]string
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"camp_lt"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"camp_lt","name":"Lifetime Campaign","objective":"OUTCOME_TRAFFIC","status":"PAUSED","lifetime_budget":"10000"}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewCampaignsCommand(deps)
	_, _, err := executeCommand(cmd,
		"--name", "Lifetime Campaign",
		"--objective", "OUTCOME_TRAFFIC",
		"--lifetime-budget", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["lifetime_budget"] != "10000" {
		t.Errorf("expected lifetime_budget 10000, got %q", capturedParams["lifetime_budget"])
	}
	if _, ok := capturedParams["daily_budget"]; ok {
		t.Error("did not expect daily_budget when lifetime budget is set")
	}
}

func TestCampaignsCommand_APIError(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		return nil, &meta.GraphError{
			Message: "Invalid parameter",
			Code:    100,
		}
	}

	deps := testDeps(mc)
	cmd := NewCampaignsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "Fail Campaign",
		"--objective", "OUTCOME_SALES",
		"--daily-budget", "5",
	)
	if stderr == "" {
		t.Fatal("expected error output for API error")
	}
}
