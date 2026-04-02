package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestAdSetsCommand_Success(t *testing.T) {
	mc := &mockClient{}
	var capturedPath string
	var capturedParams map[string]string
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedPath = path
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"adset_123","name":"My AdSet","campaign_id":"camp_1","status":"PAUSED","daily_budget":"500"}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAdSetsCommand(deps)
	stdout, _, err := executeCommand(cmd,
		"--name", "My AdSet",
		"--campaign", "camp_1",
		"--optimization-goal", "LINK_CLICKS",
		"--countries", "US,CA",
		"--daily-budget", "5",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_123456/adsets" {
		t.Errorf("expected path /act_123456/adsets, got %q", capturedPath)
	}
	if capturedParams["campaign_id"] != "camp_1" {
		t.Errorf("expected campaign_id camp_1, got %q", capturedParams["campaign_id"])
	}
	if capturedParams["daily_budget"] != "500" {
		t.Errorf("expected daily_budget 500, got %q", capturedParams["daily_budget"])
	}
	if capturedParams["targeting"] == "" {
		t.Fatal("expected targeting JSON in params")
	}

	var result meta.AdSet
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if result.ID != "adset_123" {
		t.Errorf("expected adset ID adset_123, got %q", result.ID)
	}
	if result.CampaignID != "camp_1" {
		t.Errorf("expected campaign_id camp_1, got %q", result.CampaignID)
	}
}

func TestAdSetsCommand_MissingName(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAdSetsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--campaign", "camp_1",
		"--optimization-goal", "LINK_CLICKS",
		"--countries", "US",
	)
	if stderr == "" {
		t.Fatal("expected validation error output for missing --name")
	}
}

func TestAdSetsCommand_MissingCampaign(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAdSetsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "My AdSet",
		"--optimization-goal", "LINK_CLICKS",
		"--countries", "US",
	)
	if stderr == "" {
		t.Fatal("expected validation error output for missing --campaign")
	}
}

func TestAdSetsCommand_MissingCountries(t *testing.T) {
	origExit := osExit
	osExit = func(int) {}
	defer func() { osExit = origExit }()

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAdSetsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "My AdSet",
		"--campaign", "camp_1",
		"--optimization-goal", "LINK_CLICKS",
	)
	if stderr == "" {
		t.Fatal("expected validation error output for missing --countries")
	}
}

func TestAdSetsCommand_APIError(t *testing.T) {
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
	cmd := NewAdSetsCommand(deps)
	_, stderr, _ := executeCommand(cmd,
		"--name", "Fail AdSet",
		"--campaign", "camp_1",
		"--optimization-goal", "LINK_CLICKS",
		"--countries", "US",
	)
	if stderr == "" {
		t.Fatal("expected error output for API error")
	}
}
