package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// routingClient is a test helper that dispatches mock Graph responses by
// request path. Matching is prefix-based: the longest registered prefix that
// matches the requested path wins. Each matcher is a sequence of responses
// (to support paginated sources); repeated calls consume the slice in order,
// and if fewer responses are registered than the code issues, the test fails.
type routingClient struct {
	t           *testing.T
	mu          sync.Mutex
	matchers    []routeMatcher
	calls       map[string]int
	concurrency *concurrencyTracker
	preGet      func(path string)
}

type routeMatcher struct {
	prefix    string
	responses []routeResponse
	idx       int
}

type routeResponse struct {
	body   string
	status int
	err    error
}

func newRoutingClient(t *testing.T) *routingClient {
	return &routingClient{
		t:     t,
		calls: make(map[string]int),
	}
}

// route registers a matcher. When the same prefix is registered twice, the
// additional responses are appended to that matcher so tests can build up
// multi-page sequences with one call per page.
func (r *routingClient) route(prefix string, resps ...routeResponse) *routingClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.matchers {
		if r.matchers[i].prefix == prefix {
			r.matchers[i].responses = append(r.matchers[i].responses, resps...)
			return r
		}
	}
	r.matchers = append(r.matchers, routeMatcher{prefix: prefix, responses: append([]routeResponse(nil), resps...)})
	return r
}

// okJSON is a helper for a 200 JSON response.
func okJSON(body string) routeResponse {
	return routeResponse{body: body, status: http.StatusOK}
}

// graphErr is a helper for a Meta-style error payload.
func graphErr(status int, code int, message string) routeResponse {
	body := fmt.Sprintf(`{"error":{"message":%q,"type":"OAuthException","code":%d,"error_subcode":0,"fbtrace_id":"t1"}}`, message, code)
	return routeResponse{body: body, status: status}
}

func (r *routingClient) Get(ctx context.Context, path string, params url.Values) (*Response, error) {
	if r.concurrency != nil {
		r.concurrency.enter()
		defer r.concurrency.leave()
	}
	if r.preGet != nil {
		r.preGet(path)
	}
	r.mu.Lock()
	r.calls[path]++
	var best *routeMatcher
	bestLen := -1
	for i := range r.matchers {
		if strings.HasPrefix(path, r.matchers[i].prefix) && len(r.matchers[i].prefix) > bestLen {
			best = &r.matchers[i]
			bestLen = len(r.matchers[i].prefix)
		}
	}
	if best == nil {
		r.mu.Unlock()
		r.t.Fatalf("routingClient: no route for path %q", path)
		return nil, nil
	}
	if best.idx >= len(best.responses) {
		r.mu.Unlock()
		r.t.Fatalf("routingClient: no more responses for %q (got %d calls)", best.prefix, best.idx+1)
		return nil, nil
	}
	resp := best.responses[best.idx]
	best.idx++
	r.mu.Unlock()

	if resp.err != nil {
		return nil, resp.err
	}
	return &Response{Body: []byte(resp.body), StatusCode: resp.status}, nil
}

// concurrencyTracker records the maximum number of in-flight Get calls.
type concurrencyTracker struct {
	mu      sync.Mutex
	current int
	peak    int
	gate    chan struct{} // optional: if non-nil, each enter() blocks until sent.
}

func newConcurrencyTracker() *concurrencyTracker {
	return &concurrencyTracker{}
}

func (c *concurrencyTracker) enter() {
	c.mu.Lock()
	c.current++
	if c.current > c.peak {
		c.peak = c.current
	}
	c.mu.Unlock()
	if c.gate != nil {
		<-c.gate
	}
}

func (c *concurrencyTracker) leave() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current--
}

func (c *concurrencyTracker) peakConcurrency() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peak
}

// adapter lets routingClient satisfy the full Client interface via MockClient.
func (r *routingClient) asClient() Client {
	return &MockClient{
		GetFn: r.Get,
		PostFn: func(context.Context, string, map[string]string) (*Response, error) {
			r.t.Fatalf("Post should not be called")
			return nil, nil
		},
		PaginateFn: func(context.Context, string, url.Values) *PageIterator {
			r.t.Fatalf("Paginate should not be called")
			return nil
		},
	}
}

// ───────────────────────── Test fixtures ─────────────────────────

const (
	emptyBusinesses = `{"data":[]}`
	emptyPages      = `{"data":[]}`
)

func businessListBody(ids ...string) string {
	type biz struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	bizzes := make([]biz, 0, len(ids))
	for _, id := range ids {
		bizzes = append(bizzes, biz{ID: id, Name: "BM " + id})
	}
	body, _ := json.Marshal(struct {
		Data []biz `json:"data"`
	}{bizzes})
	return string(body)
}

func pagesBody(pages ...Page) string {
	arr := make([]map[string]any, 0, len(pages))
	for _, p := range pages {
		m := map[string]any{
			"id":   p.ID,
			"name": p.Name,
		}
		if p.Category != "" {
			m["category"] = p.Category
		}
		if p.AccessToken != "" {
			m["access_token"] = p.AccessToken
		}
		if len(p.Tasks) > 0 {
			m["tasks"] = p.Tasks
		}
		arr = append(arr, m)
	}
	body, _ := json.Marshal(map[string]any{"data": arr})
	return string(body)
}

// ───────────────────────── Tests ─────────────────────────

// Case 1: direct page + two BM-owned pages (from two different BMs).
func TestListPages_HappyPath_MultiSource(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(
		Page{ID: "100", Name: "Alpha Page", Category: "Business", AccessToken: "direct-tok"},
	)))
	rc.route("/me/businesses", okJSON(businessListBody("b1", "b2")))
	rc.route("/b1/owned_pages", okJSON(pagesBody(
		Page{ID: "200", Name: "Beta Page", Category: "Retail"},
	)))
	rc.route("/b1/client_pages", okJSON(emptyPages))
	rc.route("/b2/owned_pages", okJSON(pagesBody(
		Page{ID: "300", Name: "Gamma Page", Category: "Media"},
	)))
	rc.route("/b2/client_pages", okJSON(emptyPages))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", result.Warnings)
	}
	if len(result.Data) != 3 {
		t.Fatalf("expected 3 pages, got %d: %+v", len(result.Data), result.Data)
	}

	got := map[string]Page{}
	for _, p := range result.Data {
		got[p.ID] = p
	}

	if p, ok := got["100"]; !ok {
		t.Error("direct page missing")
	} else {
		if p.AccessToken != "direct-tok" {
			t.Errorf("direct page access token: got %q", p.AccessToken)
		}
		if len(p.Sources) != 1 || p.Sources[0] != PageSourceDirect {
			t.Errorf("direct page sources: got %v", p.Sources)
		}
	}
	if p, ok := got["200"]; !ok {
		t.Error("b1-owned page missing")
	} else {
		if len(p.Sources) != 1 || p.Sources[0] != PageSourceBusinessOwned {
			t.Errorf("b1-owned page sources: got %v", p.Sources)
		}
		if len(p.BusinessIDs) != 1 || p.BusinessIDs[0] != "b1" {
			t.Errorf("b1-owned page business IDs: got %v", p.BusinessIDs)
		}
	}
	if p, ok := got["300"]; !ok {
		t.Error("b2-owned page missing")
	} else {
		if len(p.BusinessIDs) != 1 || p.BusinessIDs[0] != "b2" {
			t.Errorf("b2-owned page business IDs: got %v", p.BusinessIDs)
		}
	}
}

// Case 2: dedup direct + BM-owned with same page id.
func TestListPages_Dedup_DirectAndBusinessOwned(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(
		Page{ID: "100", Name: "Shopi", Category: "E-commerce website", AccessToken: "EAApage"},
	)))
	rc.route("/me/businesses", okJSON(businessListBody("581184769220955")))
	rc.route("/581184769220955/owned_pages", okJSON(pagesBody(
		Page{ID: "100", Name: "Shopi", Category: "E-commerce website", Tasks: []string{"MANAGE", "CREATE_CONTENT"}},
	)))
	rc.route("/581184769220955/client_pages", okJSON(emptyPages))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 deduped page, got %d", len(result.Data))
	}
	p := result.Data[0]
	if p.ID != "100" {
		t.Errorf("ID: got %q", p.ID)
	}
	wantSources := []PageSource{PageSourceBusinessOwned, PageSourceDirect}
	if !equalSources(p.Sources, wantSources) {
		t.Errorf("Sources: got %v want %v", p.Sources, wantSources)
	}
	if len(p.BusinessIDs) != 1 || p.BusinessIDs[0] != "581184769220955" {
		t.Errorf("BusinessIDs: got %v", p.BusinessIDs)
	}
	if p.AccessToken != "EAApage" {
		t.Errorf("AccessToken: got %q (wanted preserved from direct)", p.AccessToken)
	}
	// Tasks should be populated from the BM source since direct didn't have any.
	if len(p.Tasks) != 2 || p.Tasks[0] != "MANAGE" {
		t.Errorf("Tasks: got %v", p.Tasks)
	}
}

// Case 3: triple dedup (direct + owned + client for same id).
func TestListPages_Dedup_TripleSource(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(
		Page{ID: "100", Name: "Shopi", AccessToken: "direct-tok"},
	)))
	rc.route("/me/businesses", okJSON(businessListBody("bm1")))
	rc.route("/bm1/owned_pages", okJSON(pagesBody(Page{ID: "100", Name: "Shopi"})))
	rc.route("/bm1/client_pages", okJSON(pagesBody(Page{ID: "100", Name: "Shopi"})))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 deduped page, got %d", len(result.Data))
	}
	p := result.Data[0]
	want := []PageSource{PageSourceBusinessClient, PageSourceBusinessOwned, PageSourceDirect}
	if !equalSources(p.Sources, want) {
		t.Errorf("Sources: got %v want %v", p.Sources, want)
	}
	if len(p.BusinessIDs) != 1 || p.BusinessIDs[0] != "bm1" {
		t.Errorf("BusinessIDs: got %v", p.BusinessIDs)
	}
}

// Case 4: /me/accounts fails — BM pages still come back with a warning for /me/accounts.
func TestListPages_PartialFailure_MeAccounts(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", graphErr(http.StatusForbidden, 10, "requires pages_read_engagement"))
	rc.route("/me/businesses", okJSON(businessListBody("b1")))
	rc.route("/b1/owned_pages", okJSON(pagesBody(Page{ID: "200", Name: "Beta"})))
	rc.route("/b1/client_pages", okJSON(emptyPages))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "200" {
		t.Fatalf("expected BM page 200, got %+v", result.Data)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(result.Warnings), result.Warnings)
	}
	if result.Warnings[0].Source != "me/accounts" {
		t.Errorf("warning source: got %q", result.Warnings[0].Source)
	}
	if result.Warnings[0].Code != 10 {
		t.Errorf("warning code: got %d", result.Warnings[0].Code)
	}
}

// Case 5: /me/businesses fails — direct pages still come back with a warning.
func TestListPages_PartialFailure_MeBusinesses(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(Page{ID: "100", Name: "Alpha", AccessToken: "tok"})))
	rc.route("/me/businesses", graphErr(http.StatusForbidden, 200, "no business_management"))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "100" {
		t.Fatalf("expected direct page, got %+v", result.Data)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Source != "me/businesses" {
		t.Errorf("warning source: got %q", result.Warnings[0].Source)
	}
}

// Case 6: one BM of two fails individually — the other's pages come through.
func TestListPages_PartialFailure_SingleBusiness(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(emptyPages))
	rc.route("/me/businesses", okJSON(businessListBody("ok_bm", "bad_bm")))
	rc.route("/ok_bm/owned_pages", okJSON(pagesBody(Page{ID: "500", Name: "Good"})))
	rc.route("/ok_bm/client_pages", okJSON(emptyPages))
	rc.route("/bad_bm/owned_pages", graphErr(http.StatusForbidden, 10, "no access"))
	rc.route("/bad_bm/client_pages", graphErr(http.StatusForbidden, 10, "no access"))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "500" {
		t.Fatalf("expected page 500 from ok_bm, got %+v", result.Data)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected 2 warnings (owned+client of bad_bm), got %d: %+v", len(result.Warnings), result.Warnings)
	}
	// Warnings are sorted by source.
	wantSources := []string{"business/bad_bm/client_pages", "business/bad_bm/owned_pages"}
	gotSources := []string{result.Warnings[0].Source, result.Warnings[1].Source}
	if !equalStrings(gotSources, wantSources) {
		t.Errorf("warning sources: got %v want %v", gotSources, wantSources)
	}
}

// Case 7: every source fails → aggregated error, no silent empty result.
func TestListPages_TotalFailure(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", graphErr(http.StatusUnauthorized, 190, "Invalid token"))
	rc.route("/me/businesses", graphErr(http.StatusUnauthorized, 190, "Invalid token"))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on total failure, got %+v", result)
	}
	var ae *AggregateSourceError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AggregateSourceError, got %T: %v", err, err)
	}
	if len(ae.Warnings) != 2 {
		t.Fatalf("expected 2 warnings inside error, got %d", len(ae.Warnings))
	}
}

// Case 8: every source returns empty → empty Data slice, no warnings, no error.
func TestListPages_EmptySuccess(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(emptyPages))
	rc.route("/me/businesses", okJSON(emptyBusinesses))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 pages, got %d", len(result.Data))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %v", result.Warnings)
	}
}

// Case 9: exhaustive pagination walks multiple pages of every source.
func TestListPages_Pagination_All(t *testing.T) {
	rc := newRoutingClient(t)
	// /me/accounts: two pages.
	rc.route("/me/accounts",
		okJSON(`{"data":[{"id":"a1","name":"Direct 1"}],"paging":{"cursors":{"after":"cur_a"},"next":"https://graph.facebook.com/v21.0/me/accounts?after=cur_a"}}`),
		okJSON(`{"data":[{"id":"a2","name":"Direct 2"}]}`),
	)
	// /me/businesses: two pages.
	rc.route("/me/businesses",
		okJSON(`{"data":[{"id":"bm1","name":"BM One"}],"paging":{"cursors":{"after":"cur_b"},"next":"https://graph.facebook.com/v21.0/me/businesses?after=cur_b"}}`),
		okJSON(`{"data":[{"id":"bm2","name":"BM Two"}]}`),
	)
	// bm1 owned: two pages.
	rc.route("/bm1/owned_pages",
		okJSON(`{"data":[{"id":"p1","name":"P1"}],"paging":{"cursors":{"after":"cur_p"},"next":"https://graph.facebook.com/v21.0/bm1/owned_pages?after=cur_p"}}`),
		okJSON(`{"data":[{"id":"p2","name":"P2"}]}`),
	)
	rc.route("/bm1/client_pages", okJSON(emptyPages))
	rc.route("/bm2/owned_pages", okJSON(`{"data":[{"id":"p3","name":"P3"}]}`))
	rc.route("/bm2/client_pages", okJSON(emptyPages))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{All: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := make([]string, 0, len(result.Data))
	for _, p := range result.Data {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	wantIDs := []string{"a1", "a2", "p1", "p2", "p3"}
	if !equalStrings(ids, wantIDs) {
		t.Errorf("ids: got %v want %v", ids, wantIDs)
	}
}

// Case 10: result slice is sorted by Name then ID across all sources.
func TestListPages_DeterministicOrder(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(
		Page{ID: "z9", Name: "Charlie"},
	)))
	rc.route("/me/businesses", okJSON(businessListBody("bmA")))
	rc.route("/bmA/owned_pages", okJSON(pagesBody(
		Page{ID: "a1", Name: "Alpha"},
		Page{ID: "m5", Name: "Bravo"},
	)))
	rc.route("/bmA/client_pages", okJSON(emptyPages))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := []string{}
	for _, p := range result.Data {
		names = append(names, p.Name)
	}
	want := []string{"Alpha", "Bravo", "Charlie"}
	if !equalStrings(names, want) {
		t.Errorf("page order: got %v want %v", names, want)
	}

	// Two successive runs must produce identical output.
	rc2 := newRoutingClient(t)
	rc2.route("/me/accounts", okJSON(pagesBody(Page{ID: "z9", Name: "Charlie"})))
	rc2.route("/me/businesses", okJSON(businessListBody("bmA")))
	rc2.route("/bmA/owned_pages", okJSON(pagesBody(
		Page{ID: "m5", Name: "Bravo"},
		Page{ID: "a1", Name: "Alpha"},
	)))
	rc2.route("/bmA/client_pages", okJSON(emptyPages))

	result2, err := ListPages(context.Background(), rc2.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out1, _ := json.Marshal(result.Data)
	out2, _ := json.Marshal(result2.Data)
	if string(out1) != string(out2) {
		t.Errorf("non-deterministic output:\n%s\n%s", out1, out2)
	}
}

// Case 11: per-BM fetches run in parallel. A gated routing client blocks each
// in-flight Get until released, so we can assert the peak concurrency hit.
func TestListPages_BusinessFetchConcurrency(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(emptyPages))
	rc.route("/me/businesses", okJSON(businessListBody("bm1", "bm2", "bm3")))
	rc.route("/bm1/owned_pages", okJSON(emptyPages))
	rc.route("/bm1/client_pages", okJSON(emptyPages))
	rc.route("/bm2/owned_pages", okJSON(emptyPages))
	rc.route("/bm2/client_pages", okJSON(emptyPages))
	rc.route("/bm3/owned_pages", okJSON(emptyPages))
	rc.route("/bm3/client_pages", okJSON(emptyPages))

	tracker := newConcurrencyTracker()
	rc.concurrency = tracker

	// Delay every BM request slightly so multiple goroutines are guaranteed
	// to be in-flight at once. The delay only fires for BM endpoints so the
	// /me/accounts call (which is unrelated to the BM fanout) isn't skewing
	// the measurement.
	var bmCalls atomic.Int32
	rc.preGet = func(path string) {
		if strings.Contains(path, "/owned_pages") || strings.Contains(path, "/client_pages") {
			bmCalls.Add(1)
			time.Sleep(30 * time.Millisecond)
		}
	}

	_, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bmCalls.Load() != 6 {
		t.Errorf("expected 6 BM calls (3 BMs * 2 endpoints), got %d", bmCalls.Load())
	}
	if peak := tracker.peakConcurrency(); peak < 2 {
		t.Errorf("expected BM fetches to run in parallel (peak > 1), got peak=%d", peak)
	}
}

// Case 12: a direct-only page still has its access_token populated after the
// new aggregation pipeline (back-compat for callers that need the Page token).
func TestListPages_DirectAccessTokenPreserved(t *testing.T) {
	rc := newRoutingClient(t)
	rc.route("/me/accounts", okJSON(pagesBody(
		Page{ID: "500", Name: "Alpha", AccessToken: "EAApage123"},
	)))
	rc.route("/me/businesses", okJSON(emptyBusinesses))

	result, err := ListPages(context.Background(), rc.asClient(), ListPagesParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 page, got %d", len(result.Data))
	}
	if result.Data[0].AccessToken != "EAApage123" {
		t.Errorf("AccessToken: got %q", result.Data[0].AccessToken)
	}
}

// Additional: the new JSON shape round-trips. Ensures sources/warnings appear
// and empty optional fields are omitted cleanly.
func TestListPagesResult_JSONSerialization(t *testing.T) {
	result := &ListPagesResult{
		Data: []Page{{
			ID:      "1",
			Name:    "P1",
			Sources: []PageSource{PageSourceDirect},
		}},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"sources":["direct"]`) {
		t.Errorf("sources not serialized: %s", data)
	}
	if strings.Contains(string(data), `"access_token"`) {
		t.Errorf("empty access_token should be omitted: %s", data)
	}
	if strings.Contains(string(data), `"warnings"`) {
		t.Errorf("empty warnings should be omitted: %s", data)
	}

	var decoded ListPagesResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded.Data) != 1 || decoded.Data[0].Sources[0] != PageSourceDirect {
		t.Errorf("round-trip: got %+v", decoded)
	}
}

// Limit is forwarded to the underlying Graph calls unchanged.
func TestListPages_CustomLimit(t *testing.T) {
	var mu sync.Mutex
	seenLimits := map[string]string{}
	mock := &MockClient{
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			mu.Lock()
			seenLimits[path] = params.Get("limit")
			mu.Unlock()
			switch path {
			case "/me/accounts":
				return &Response{Body: []byte(emptyPages), StatusCode: 200}, nil
			case "/me/businesses":
				return &Response{Body: []byte(emptyBusinesses), StatusCode: 200}, nil
			default:
				t.Errorf("unexpected path %q", path)
				return nil, fmt.Errorf("unexpected path %q", path)
			}
		},
	}
	_, err := ListPages(context.Background(), mock, ListPagesParams{Limit: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seenLimits["/me/accounts"] != "7" {
		t.Errorf("expected /me/accounts limit=7, got %q", seenLimits["/me/accounts"])
	}
	if seenLimits["/me/businesses"] != "7" {
		t.Errorf("expected /me/businesses limit=7, got %q", seenLimits["/me/businesses"])
	}
}

// ───────────────────────── helpers ─────────────────────────

func equalSources(a, b []PageSource) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
