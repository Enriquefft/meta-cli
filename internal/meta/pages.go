// Package meta: Facebook Pages listing with multi-source aggregation.
//
// Meta does not expose a single endpoint that returns every Page a user can
// access. Enterprise users typically reach Pages through Business Manager,
// which /me/accounts does not surface. The correct answer is to fan out over
// three sources and merge:
//
//  1. GET /me/accounts                    — Pages the user has a direct role on.
//  2. GET /{business}/owned_pages         — Pages owned by each BM the user belongs to.
//  3. GET /{business}/client_pages        — Pages the BM has client access to.
//
// Failures of individual sources become Warnings on the result; the call only
// errors when every attempted source failed.

package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// businessFanoutLimit caps the concurrency of per-Business-Manager fetches.
// The transport layer (internal/graph) handles Meta's rate-limit backoff, so
// this cap only exists to keep goroutine pressure bounded on accounts with
// dozens of BMs.
const businessFanoutLimit = 8

// PageSource describes how the authenticated user has access to a Page.
type PageSource string

const (
	// PageSourceDirect means the user has a direct role on the Page
	// (returned by GET /me/accounts).
	PageSourceDirect PageSource = "direct"
	// PageSourceBusinessOwned means the Page is owned by a Business Manager
	// the user is a member of (GET /{bm}/owned_pages).
	PageSourceBusinessOwned PageSource = "business_owned"
	// PageSourceBusinessClient means a Business Manager the user is a member
	// of has client access to the Page (GET /{bm}/client_pages).
	PageSourceBusinessClient PageSource = "business_client"
)

// ListPagesParams configures the pages listing request.
type ListPagesParams struct {
	// Limit is the per-API-call page size passed to every underlying Graph
	// request (default 25).
	Limit int
	// All triggers exhaustive pagination of every source. Without it, each
	// source fetches only its first page and the results are aggregated.
	All bool
}

// Page represents a Facebook Page the authenticated user can access.
//
// The same logical Page can be reachable through multiple sources (e.g. a
// direct role plus a Business Manager). In that case a single Page entry is
// returned with all sources unioned into Sources.
type Page struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Category is empty for pages returned from /owned_pages and
	// /client_pages unless Meta includes it in the response.
	Category string `json:"category,omitempty"`
	// AccessToken is only present when the Page was reached via
	// /me/accounts. Business-Manager-sourced Pages do not expose a Page
	// access token on this endpoint.
	AccessToken string `json:"access_token,omitempty"`
	// Sources lists every route through which the user can reach this Page,
	// sorted alphabetically for deterministic output.
	Sources []PageSource `json:"sources"`
	// BusinessIDs lists every Business Manager through which the Page is
	// accessible, sorted lexicographically for deterministic output.
	BusinessIDs []string `json:"business_ids,omitempty"`
	// Tasks lists the permitted tasks on the Page, when Meta returns them.
	// The set varies by source; when a Page is reachable through multiple
	// sources the first non-empty set is retained.
	Tasks []string `json:"tasks,omitempty"`
}

// SourceWarning describes a partial failure of one source during aggregation.
type SourceWarning struct {
	// Source is a stable, machine-parseable identifier like "me/accounts" or
	// "business/581184769220955/owned_pages". No leading slash, no version.
	Source string `json:"source"`
	// Code is the Meta error code if the failure came from a Graph error.
	Code int `json:"code,omitempty"`
	// Message is a human-readable description of the failure.
	Message string `json:"message"`
}

// ListPagesResult contains the aggregated pages, any per-source warnings,
// and pagination state.
type ListPagesResult struct {
	Data     []Page          `json:"data"`
	Warnings []SourceWarning `json:"warnings,omitempty"`
	// HasNext is true when at least one source reported a next page. In
	// aggregated mode the caller cannot resume from a single cursor, so this
	// is purely informational.
	HasNext bool `json:"has_next"`
	// Cursor is retained for backward compatibility with the pre-aggregation
	// API shape and is always empty in the aggregated implementation.
	Cursor string `json:"cursor,omitempty"`
}

// ListPages retrieves every Facebook Page the authenticated user can access
// by aggregating /me/accounts, /me/businesses → /{bm}/owned_pages, and
// /{bm}/client_pages. Per-source failures become Warnings; the call only
// errors when every attempted source failed.
func ListPages(ctx context.Context, client Client, params ListPagesParams) (*ListPagesResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	agg := newPagesAggregator()

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		fetchDirectPages(gctx, client, limit, params.All, agg)
		return nil
	})

	g.Go(func() error {
		fetchBusinessPages(gctx, client, limit, params.All, agg)
		return nil
	})

	// Worker funcs never return an error — they record warnings and data on
	// the aggregator. Wait cannot fail, but honor its contract anyway.
	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := agg.build()

	if len(result.Data) == 0 && agg.allAttemptedSourcesFailed() {
		return nil, &AggregateSourceError{Warnings: result.Warnings}
	}

	return result, nil
}

// AggregateSourceError is returned from ListPages when every attempted source
// produced an error and no pages were recovered. The caller can still inspect
// individual failures through Warnings.
type AggregateSourceError struct {
	Warnings []SourceWarning
}

func (e *AggregateSourceError) Error() string {
	if len(e.Warnings) == 0 {
		return "listing pages: all sources failed"
	}
	parts := make([]string, 0, len(e.Warnings))
	for _, w := range e.Warnings {
		parts = append(parts, fmt.Sprintf("%s: %s", w.Source, w.Message))
	}
	return "listing pages: all sources failed: " + strings.Join(parts, "; ")
}

// pagesAggregator is a thread-safe collector of pages and per-source warnings.
type pagesAggregator struct {
	mu        sync.Mutex
	pages     map[string]*Page
	warnings  []SourceWarning
	hasNext   bool
	attempted int
	failed    int
}

func newPagesAggregator() *pagesAggregator {
	return &pagesAggregator{pages: make(map[string]*Page)}
}

// recordAttempt marks that one source was attempted. Call exactly once per
// source (so every owned_pages / client_pages call counts separately).
func (a *pagesAggregator) recordAttempt() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attempted++
}

// recordWarning marks the current source as failed and stores the warning.
func (a *pagesAggregator) recordWarning(w SourceWarning) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.warnings = append(a.warnings, w)
	a.failed++
}

// mergePage unions a new page entry into the aggregator.
func (a *pagesAggregator) mergePage(p Page) {
	if p.ID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	existing, ok := a.pages[p.ID]
	if !ok {
		clone := p
		existing = &clone
		a.pages[p.ID] = existing
	} else {
		if existing.Name == "" {
			existing.Name = p.Name
		}
		if existing.Category == "" {
			existing.Category = p.Category
		}
		// Prefer a direct AccessToken (only direct pages populate it; if
		// merging from different sources, the direct one wins). If neither
		// is from the direct source, take whichever non-empty token exists.
		if existing.AccessToken == "" {
			existing.AccessToken = p.AccessToken
		}
		if len(existing.Tasks) == 0 {
			existing.Tasks = append(existing.Tasks, p.Tasks...)
		}
		existing.Sources = append(existing.Sources, p.Sources...)
		existing.BusinessIDs = append(existing.BusinessIDs, p.BusinessIDs...)
	}
}

// markHasNext records that at least one source reported pagination.
func (a *pagesAggregator) markHasNext() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hasNext = true
}

// allAttemptedSourcesFailed reports whether every attempted source errored.
// Must be called after Wait().
func (a *pagesAggregator) allAttemptedSourcesFailed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.attempted > 0 && a.failed == a.attempted
}

// build finalizes the aggregator into a deterministic ListPagesResult. Every
// Page's Sources and BusinessIDs are deduplicated and sorted, and the result
// slice is sorted by (Name, ID).
func (a *pagesAggregator) build() *ListPagesResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	data := make([]Page, 0, len(a.pages))
	for _, p := range a.pages {
		p.Sources = dedupSortedSources(p.Sources)
		p.BusinessIDs = dedupSortedStrings(p.BusinessIDs)
		data = append(data, *p)
	}
	sort.Slice(data, func(i, j int) bool {
		if data[i].Name != data[j].Name {
			return data[i].Name < data[j].Name
		}
		return data[i].ID < data[j].ID
	})

	warnings := make([]SourceWarning, len(a.warnings))
	copy(warnings, a.warnings)
	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Source < warnings[j].Source
	})

	return &ListPagesResult{
		Data:     data,
		Warnings: warnings,
		HasNext:  a.hasNext,
	}
}

func dedupSortedSources(in []PageSource) []PageSource {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[PageSource]struct{}, len(in))
	out := make([]PageSource, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func dedupSortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// rawPage is the wire shape returned by every Pages-listing endpoint.
type rawPage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
}

type rawPagesPayload struct {
	Data   []rawPage `json:"data"`
	Paging struct {
		Cursors struct {
			After string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging"`
}

// fetchAllPagesOfSource walks an endpoint and invokes onPage for each decoded
// entry. When exhaustive is false, it stops after the first response and, if
// Meta reports a next page, calls markHasNext. Any error short-circuits and
// is returned to the caller (the caller converts it into a warning).
func fetchAllPagesOfSource(
	ctx context.Context,
	client Client,
	path string,
	fields string,
	limit int,
	exhaustive bool,
	onPage func(rawPage),
	markHasNext func(),
) error {
	qp := url.Values{}
	qp.Set("fields", fields)
	qp.Set("limit", strconv.Itoa(limit))

	for {
		resp, err := client.Get(ctx, path, qp)
		if err != nil {
			return err
		}
		if ge := ParseGraphError(resp.Body); ge != nil {
			return ge
		}

		var payload rawPagesPayload
		if err := json.Unmarshal(resp.Body, &payload); err != nil {
			return fmt.Errorf("parsing %s response: %w", path, err)
		}

		for _, p := range payload.Data {
			onPage(p)
		}

		if payload.Paging.Cursors.After == "" || payload.Paging.Next == "" {
			return nil
		}
		if !exhaustive {
			if markHasNext != nil {
				markHasNext()
			}
			return nil
		}

		next := url.Values{}
		for k, v := range qp {
			if k == "after" {
				continue
			}
			next[k] = v
		}
		next.Set("after", payload.Paging.Cursors.After)
		qp = next
	}
}

// fetchDirectPages calls GET /me/accounts and records the result on the
// aggregator. It never returns an error; failures are recorded as warnings.
func fetchDirectPages(ctx context.Context, client Client, limit int, exhaustive bool, agg *pagesAggregator) {
	const source = "me/accounts"
	agg.recordAttempt()

	err := fetchAllPagesOfSource(
		ctx, client,
		"/me/accounts",
		"id,name,category,access_token,tasks",
		limit, exhaustive,
		func(p rawPage) {
			agg.mergePage(Page{
				ID:          p.ID,
				Name:        p.Name,
				Category:    p.Category,
				AccessToken: p.AccessToken,
				Tasks:       p.Tasks,
				Sources:     []PageSource{PageSourceDirect},
			})
		},
		agg.markHasNext,
	)
	if err != nil {
		agg.recordWarning(warningFromErr(source, err))
	}
}

// rawBusiness is the wire shape returned by GET /me/businesses.
type rawBusiness struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawBusinessesPayload struct {
	Data   []rawBusiness `json:"data"`
	Paging struct {
		Cursors struct {
			After string `json:"after"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging"`
}

// listBusinesses enumerates every Business Manager the user belongs to. In
// exhaustive mode it walks all pages; otherwise it stops after the first
// response (and marks has_next on the aggregator).
func listBusinesses(ctx context.Context, client Client, limit int, exhaustive bool, agg *pagesAggregator) ([]rawBusiness, error) {
	qp := url.Values{}
	qp.Set("fields", "id,name")
	qp.Set("limit", strconv.Itoa(limit))

	var all []rawBusiness
	for {
		resp, err := client.Get(ctx, "/me/businesses", qp)
		if err != nil {
			return nil, err
		}
		if ge := ParseGraphError(resp.Body); ge != nil {
			return nil, ge
		}

		var payload rawBusinessesPayload
		if err := json.Unmarshal(resp.Body, &payload); err != nil {
			return nil, fmt.Errorf("parsing /me/businesses response: %w", err)
		}
		all = append(all, payload.Data...)

		if payload.Paging.Cursors.After == "" || payload.Paging.Next == "" {
			return all, nil
		}
		if !exhaustive {
			agg.markHasNext()
			return all, nil
		}

		next := url.Values{}
		for k, v := range qp {
			if k == "after" {
				continue
			}
			next[k] = v
		}
		next.Set("after", payload.Paging.Cursors.After)
		qp = next
	}
}

// fetchBusinessPages enumerates /me/businesses and fans out per-BM to
// /owned_pages and /client_pages in parallel, capped by businessFanoutLimit.
// /me/businesses failure records a single warning and returns. Per-BM
// endpoint failures each record their own warning.
func fetchBusinessPages(ctx context.Context, client Client, limit int, exhaustive bool, agg *pagesAggregator) {
	const listSource = "me/businesses"
	agg.recordAttempt()

	businesses, err := listBusinesses(ctx, client, limit, exhaustive, agg)
	if err != nil {
		agg.recordWarning(warningFromErr(listSource, err))
		return
	}

	if len(businesses) == 0 {
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(businessFanoutLimit)

	for _, b := range businesses {
		b := b
		g.Go(func() error {
			fetchBusinessSourceForPage(gctx, client, b, "owned_pages", PageSourceBusinessOwned, limit, exhaustive, agg)
			return nil
		})
		g.Go(func() error {
			fetchBusinessSourceForPage(gctx, client, b, "client_pages", PageSourceBusinessClient, limit, exhaustive, agg)
			return nil
		})
	}

	_ = g.Wait()
}

// fetchBusinessSourceForPage calls one of /{bm}/owned_pages or
// /{bm}/client_pages. Failures become a warning scoped to that BM+endpoint.
func fetchBusinessSourceForPage(
	ctx context.Context,
	client Client,
	b rawBusiness,
	endpoint string,
	pageSource PageSource,
	limit int,
	exhaustive bool,
	agg *pagesAggregator,
) {
	source := fmt.Sprintf("business/%s/%s", b.ID, endpoint)
	path := fmt.Sprintf("/%s/%s", b.ID, endpoint)

	agg.recordAttempt()

	err := fetchAllPagesOfSource(
		ctx, client,
		path,
		// BM page endpoints don't return access_token (by Graph design)
		// but do return category and tasks.
		"id,name,category,tasks",
		limit, exhaustive,
		func(p rawPage) {
			agg.mergePage(Page{
				ID:          p.ID,
				Name:        p.Name,
				Category:    p.Category,
				Tasks:       p.Tasks,
				Sources:     []PageSource{pageSource},
				BusinessIDs: []string{b.ID},
			})
		},
		agg.markHasNext,
	)
	if err != nil {
		agg.recordWarning(warningFromErr(source, err))
	}
}

// warningFromErr converts an error (Graph or transport) to a SourceWarning.
func warningFromErr(source string, err error) SourceWarning {
	w := SourceWarning{Source: source, Message: err.Error()}
	var ge *GraphError
	if errors.As(err, &ge) {
		w.Code = ge.Code
		w.Message = ge.Message
	}
	return w
}
