//go:build integration

package meta_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/enriquefft/meta-cli/internal/graph"
	meta "github.com/enriquefft/meta-cli/internal/meta"
)

const integrationAPIVersion = "v21.0"

// TestIntegration_ListPages_Aggregated verifies that meta.ListPages can reach
// Pages via at least one source against the live Meta Marketing API. The test
// is gated on the META_ACCESS_TOKEN environment variable and runs only with
// `go test -tags integration ./internal/meta/`.
//
// Expected outcomes:
//   - No error (total failure is treated as a bug).
//   - Either len(Data) > 0, or every attempted source produced a warning
//     explaining why — a clean "user has zero accessible pages" scenario.
func TestIntegration_ListPages_Aggregated(t *testing.T) {
	token := os.Getenv("META_ACCESS_TOKEN")
	if token == "" {
		t.Skip("META_ACCESS_TOKEN not set; skipping integration test")
	}

	client := graph.NewClient(graph.ClientConfig{
		AccessToken: token,
		APIVersion:  integrationAPIVersion,
	})

	result, err := meta.ListPages(context.Background(), client, meta.ListPagesParams{All: true})
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	// Pretty-print for visibility.
	pretty, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("ListPages result:\n%s", pretty)

	if len(result.Data) == 0 && len(result.Warnings) == 0 {
		t.Log("no pages and no warnings — user may have zero accessible pages, clean exit")
		return
	}

	if len(result.Data) == 0 {
		t.Logf("no pages returned, but %d warning(s) explain why:", len(result.Warnings))
		for _, w := range result.Warnings {
			t.Logf("  source=%s code=%d message=%s", w.Source, w.Code, w.Message)
		}
		return
	}

	for _, p := range result.Data {
		t.Logf("page: id=%s name=%q sources=%v business_ids=%v", p.ID, p.Name, p.Sources, p.BusinessIDs)
	}
}
