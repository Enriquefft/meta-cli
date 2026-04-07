package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

// pagesRouter is a thread-safe helper that dispatches mock Graph responses by
// path prefix and supports returning a sequence of responses for paginated
// sources. The aggregated ListPages implementation fans out across multiple
// goroutines, so this must be safe for concurrent Get calls.
type pagesRouter struct {
	t     *testing.T
	mu    sync.Mutex
	m     map[string][]*meta.Response
	count map[string]int
}

func newPagesRouter(t *testing.T) *pagesRouter {
	return &pagesRouter{t: t, m: make(map[string][]*meta.Response), count: make(map[string]int)}
}

func (r *pagesRouter) add(prefix, body string) *pagesRouter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[prefix] = append(r.m[prefix], &meta.Response{Body: []byte(body), StatusCode: 200})
	return r
}

func (r *pagesRouter) handler() func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
	return func(_ context.Context, path string, _ url.Values) (*meta.Response, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		// Longest-prefix match for path.
		var bestPrefix string
		for p := range r.m {
			if strings.HasPrefix(path, p) && len(p) > len(bestPrefix) {
				bestPrefix = p
			}
		}
		if bestPrefix == "" {
			return nil, fmt.Errorf("unexpected path %q", path)
		}
		i := r.count[bestPrefix]
		seq := r.m[bestPrefix]
		if i >= len(seq) {
			return nil, fmt.Errorf("no more responses for %q", bestPrefix)
		}
		r.count[bestPrefix] = i + 1
		return seq[i], nil
	}
}

func TestPagesCommand_Success(t *testing.T) {
	rr := newPagesRouter(t)
	rr.add("/me/accounts", `{"data":[{"id":"pg_1","name":"Test Page","category":"BUSINESS","access_token":"patok"}]}`)
	rr.add("/me/businesses", `{"data":[]}`)

	mc := &mockClient{getFn: rr.handler()}
	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 page, got %d", len(result.Data))
	}
	if result.Data[0].Name != "Test Page" {
		t.Errorf("expected page name Test Page, got %q", result.Data[0].Name)
	}
	if result.Data[0].AccessToken != "patok" {
		t.Errorf("expected access token to round-trip, got %q", result.Data[0].AccessToken)
	}
	if len(result.Data[0].Sources) != 1 || result.Data[0].Sources[0] != meta.PageSourceDirect {
		t.Errorf("expected direct source, got %v", result.Data[0].Sources)
	}
}

func TestPagesCommand_EmptyResult(t *testing.T) {
	rr := newPagesRouter(t)
	rr.add("/me/accounts", `{"data":[]}`)
	rr.add("/me/businesses", `{"data":[]}`)

	mc := &mockClient{getFn: rr.handler()}
	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 pages, got %d", len(result.Data))
	}
}

func TestPagesCommand_CustomLimit(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	mc := &mockClient{
		getFn: func(_ context.Context, path string, params url.Values) (*meta.Response, error) {
			mu.Lock()
			seen[path] = params.Get("limit")
			mu.Unlock()
			switch path {
			case "/me/accounts":
				return &meta.Response{Body: []byte(`{"data":[]}`), StatusCode: 200}, nil
			case "/me/businesses":
				return &meta.Response{Body: []byte(`{"data":[]}`), StatusCode: 200}, nil
			default:
				return nil, fmt.Errorf("unexpected path %q", path)
			}
		},
	}

	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	_, _, err := executeCommand(cmd, "--limit", "5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen["/me/accounts"] != "5" {
		t.Errorf("expected /me/accounts limit '5', got %q", seen["/me/accounts"])
	}
	if seen["/me/businesses"] != "5" {
		t.Errorf("expected /me/businesses limit '5', got %q", seen["/me/businesses"])
	}
}

// TestPagesCommand_AllFlag verifies that --all drives exhaustive pagination
// through the domain layer across multiple sources.
func TestPagesCommand_AllFlag(t *testing.T) {
	rr := newPagesRouter(t)
	// /me/accounts: two pages.
	rr.add("/me/accounts", `{"data":[{"id":"pg_1","name":"Page 1","category":"BUSINESS","access_token":"tok1"}],"paging":{"cursors":{"after":"cur_a"},"next":"https://graph.facebook.com/v21.0/me/accounts?after=cur_a"}}`)
	rr.add("/me/accounts", `{"data":[{"id":"pg_2","name":"Page 2","category":"BUSINESS","access_token":"tok2"}]}`)
	rr.add("/me/businesses", `{"data":[]}`)

	mc := &mockClient{getFn: rr.handler()}
	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd, "--all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(result.Data))
	}
	// Deterministic order is by Name.
	if result.Data[0].Name != "Page 1" || result.Data[1].Name != "Page 2" {
		t.Errorf("unexpected ordering: %+v", result.Data)
	}
}

// TestPagesCommand_BusinessAggregation verifies that a Page only reachable
// through Business Manager is surfaced by the CLI.
func TestPagesCommand_BusinessAggregation(t *testing.T) {
	rr := newPagesRouter(t)
	rr.add("/me/accounts", `{"data":[]}`)
	rr.add("/me/businesses", `{"data":[{"id":"bm_42","name":"Acme Ltd"}]}`)
	rr.add("/bm_42/owned_pages", `{"data":[{"id":"pg_100","name":"Acme Storefront","category":"E-commerce"}]}`)
	rr.add("/bm_42/client_pages", `{"data":[]}`)

	mc := &mockClient{getFn: rr.handler()}
	deps := testDeps(mc)
	cmd := NewPagesCommand(deps)
	stdout, _, err := executeCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.ListPagesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing pages result: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "pg_100" {
		t.Fatalf("expected BM-owned page pg_100, got %+v", result.Data)
	}
	if len(result.Data[0].Sources) != 1 || result.Data[0].Sources[0] != meta.PageSourceBusinessOwned {
		t.Errorf("expected business_owned source, got %v", result.Data[0].Sources)
	}
	if len(result.Data[0].BusinessIDs) != 1 || result.Data[0].BusinessIDs[0] != "bm_42" {
		t.Errorf("expected BusinessIDs [bm_42], got %v", result.Data[0].BusinessIDs)
	}
}
