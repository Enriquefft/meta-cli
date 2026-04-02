package mcp

import (
	"strings"
	"testing"
)

func TestServerCreation(t *testing.T) {
	mc := &testMockClient{}

	s := NewServer(mc)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServerInstructions_NonEmpty(t *testing.T) {
	if strings.TrimSpace(serverInstructions) == "" {
		t.Fatal("serverInstructions must not be empty")
	}
}

func TestServerInstructions_ContainsWorkflow(t *testing.T) {
	required := []string{
		"meta_auth_status",
		"meta_list_accounts",
		"meta_create_campaign",
		"meta_create_ad",
		"PAUSED",
	}
	for _, substr := range required {
		if !strings.Contains(serverInstructions, substr) {
			t.Errorf("serverInstructions missing expected substring %q", substr)
		}
	}
}
