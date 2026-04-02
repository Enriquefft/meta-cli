# architecture.md — meta-cli

## Design Philosophy

Two principles, non-negotiable:

1. **Deep modules** (Ousterhout): Few modules with wide, powerful interfaces. Complexity lives inside. Callers see simple types and function calls.
2. **TDD**: Write the interface → write the test → implement. Every module has its contract defined before a single line of implementation.

---

## Module Boundaries

Three deep modules. Two thin shells. Strict dependency direction — no upward imports, no circular dependencies.

```
┌─────────────────────────────────────────────────────┐
│  Thin Shells: internal/cli/, internal/mcp/          │
│  Parse input → call domain → format output          │
│  Zero business logic.                               │
├─────────────────────────────────────────────────────┤
│  Deep Module: internal/meta/                        │
│  Domain operations. Typed params → typed results.   │
│  Owns the graph.Client interface.                   │
│  Every operation is a pure function of (ctx, client, params). │
├─────────────────────────────────────────────────────┤
│  Deep Module: internal/graph/                       │
│  HTTP transport. Implements meta.Client interface.  │
│  Hides: auth, versioning, retry, rate limiting,     │
│  pagination, upload, error parsing.                 │
├─────────────────────────────────────────────────────┤
│  Deep Module: internal/config/                      │
│  Config resolution. flags > env > file > defaults.  │
│  Single source of truth for all settings.           │
├─────────────────────────────────────────────────────┤
│  Deep Module: internal/output/                      │
│  Output formatting. JSON/table/csv + field filter.  │
│  Structured errors to stderr.                       │
└─────────────────────────────────────────────────────┘
```

### Import rules

| Module | May import |
|--------|-----------|
| `internal/cli/` | `internal/meta/`, `internal/config/`, `internal/output/` |
| `internal/mcp/` | `internal/meta/`, `internal/config/`, `internal/output/` |
| `internal/meta/` | `internal/config/` (for defaults only, never directly) — no other internal imports |
| `internal/graph/` | Nothing internal. Only stdlib + `net/http`. |
| `internal/config/` | Nothing internal. |
| `internal/output/` | Nothing internal. |

**`internal/meta/` never imports `internal/graph/`.** It defines a `Client` interface. `cmd/meta/main.go` wires the concrete graph client into domain functions. This is the dependency inversion point.

---

## Interface Contracts

### graph.Client (defined in internal/meta/client.go)

The domain layer owns this interface. The graph module implements it.

```go
package meta

import (
    "context"
    "io"
    "net/url"
)

// Response wraps a raw Graph API response.
type Response struct {
    Body       []byte
    StatusCode int
    Headers    http.Header
}

// Client is the Graph API transport abstraction.
// Implementation lives in internal/graph/.
// Tests provide mock implementations.
type Client interface {
    Get(ctx context.Context, path string, params url.Values) (*Response, error)
    Post(ctx context.Context, path string, params map[string]string) (*Response, error)
    Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error)
    Paginate(ctx context.Context, path string, params url.Values) *PageIterator
}

// PageIterator yields pages of results.
// Call Next() until it returns false.
type PageIterator struct { /* unexported fields */ }

func (it *PageIterator) Next(ctx context.Context) bool
func (it *PageIterator) Page() (*Response, error)
func (it *PageIterator) Err() error
```

### GraphError (defined in internal/meta/errors.go)

```go
package meta

// GraphError represents a Meta Graph API error.
type GraphError struct {
    Message     string `json:"message"`
    Type        string `json:"type"`
    Code        int    `json:"code"`
    Subcode     int    `json:"error_subcode"`
    TraceID     string `json:"fbtrace_id"`
    IsRetryable bool
}

// Exit codes — shared across CLI and MCP.
const (
    ExitSuccess      = 0
    ExitAPIError     = 1
    ExitAuthError    = 2
    ExitValidationError = 3
    ExitConfigError  = 4
    ExitNetworkError = 5
)

// ClassifyError maps a GraphError to an exit code.
func ClassifyError(err error) int
```

### Domain Functions (defined in their respective files)

Every domain function follows the same pattern:

```go
// XxxParams — typed input, validated in the domain function.
type XxxParams struct {
    AccountID string
    // ... operation-specific fields
}

// XxxResult — typed output, no raw JSON leaks.
type XxxResult struct {
    // ... operation-specific fields
}

// Xxx — the operation. Pure function of (ctx, client, params).
// Returns typed result or structured error.
func Xxx(ctx context.Context, client Client, params XxxParams) (*XxxResult, error)
```

**Dollar handling**: CLI/MCP layers accept dollars as `float64`. Domain layer accepts cents as `int64`. Conversion happens at the shell layer, not inside domain.

```go
// DollarsToCents converts a dollar amount to cents.
// $50.00 → 5000. Panics on negative values (callers validate first).
func DollarsToCents(d float64) int64
```

---

## TDD Strategy

### Three levels of testing

| Level | Tag | What | When |
|-------|-----|------|------|
| **Unit** | (default) | Domain functions with mock `Client` | Every `go test ./...` run |
| **Integration** | `//go:build integration` | Real API calls with live token | On demand: `go test -tags=integration ./...` |
| **Transport** | (default) | Graph client against `httptest.Server` | Every `go test ./...` run |

### TDD cycle per domain function

1. **Define types** — `XxxParams` and `XxxResult` structs
2. **Write the function signature** — `func Xxx(ctx, client, params) (*Result, error)`
3. **Write unit tests** — mock `Client`, test happy path + error cases
4. **Implement** — make tests pass
5. **Write integration test** — real API call, tagged `integration`
6. **Verify** — both unit and integration pass

### Mock Client

A test helper in `internal/meta/meta_test.go` (or `internal/meta/mock_test.go`):

```go
type MockClient struct {
    GetFn      func(ctx context.Context, path string, params url.Values) (*meta.Response, error)
    PostFn     func(ctx context.Context, path string, params map[string]string) (*meta.Response, error)
    UploadFn   func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error)
    PaginateFn func(ctx context.Context, path string, params url.Values) *meta.PageIterator
}

// Implement meta.Client interface by calling the Fn fields.
```

Each test sets only the `Fn` it needs. Untested methods panic with "not implemented".

### Integration tests

Live API calls gated behind `//go:build integration`. Require env vars:
- `META_ACCESS_TOKEN`
- `META_AD_ACCOUNT`
- `META_APP_ID`

Pattern:
```go
//go:build integration

func TestAuthStatusLive(t *testing.T) {
    cfg := config.Load()
    client := graph.NewClient(cfg)
    result, err := meta.AuthStatus(context.Background(), client)
    require.NoError(t, err)
    assert.True(t, result.Valid)
}
```

Run: `META_ACCESS_TOKEN=... go test -tags=integration ./...`

---

## Type Ownership

Types live with their operation. No shared model packages.

| File | Owns |
|------|------|
| `internal/meta/client.go` | `Client` interface, `Response`, `PageIterator` |
| `internal/meta/errors.go` | `GraphError`, exit codes, `ClassifyError`, `DollarsToCents` |
| `internal/meta/auth.go` | `AuthStatusParams`, `AuthStatusResult`, `AuthStatus()` |
| `internal/meta/accounts.go` | `ListAccountsParams`, `AdAccount`, `ListAccountsResult`, `ListAccounts()` |
| `internal/meta/pages.go` | `ListPagesParams`, `Page`, `ListPagesResult`, `ListPages()` |
| `internal/meta/assets.go` | `UploadVideoParams`, `VideoStatusResult`, `UploadVideo()`, `VideoStatus()` |
| `internal/meta/targeting.go` | `SearchParams`, `TargetingSuggestion`, `SearchResult`, `SearchTargeting()` |
| `internal/meta/campaigns.go` | `CreateCampaignParams`, `Campaign`, `CreateCampaign()` |
| `internal/meta/adsets.go` | `CreateAdSetParams`, `AdSet`, `CreateAdSet()` |
| `internal/meta/creatives.go` | `CreateCreativeParams`, `Creative`, `CreateCreative()` |
| `internal/meta/ads.go` | `CreateAdParams`, `Ad`, `CreateAd()` |

---

## File Responsibility Summary

| File | Lines | Responsibility |
|------|-------|---------------|
| `internal/graph/client.go` | ~200 | HTTP client: auth, versioning, timeout, retry, logging |
| `internal/graph/errors.go` | ~80 | Parse Meta error JSON → `GraphError` |
| `internal/graph/pagination.go` | ~100 | Cursor-based page iterator |
| `internal/graph/upload.go` | ~120 | Multipart upload + resumable for >1GB |
| `internal/graph/rate.go` | ~80 | Rate limit header parsing + backoff |
| `internal/config/config.go` | ~150 | Load, resolve, get, set, validate |
| `internal/output/output.go` | ~150 | JSON/table/csv formatting + field filter + error output |
| `internal/meta/*.go` (9 files) | ~50-100 each | Domain operations + types |
| `internal/cli/*.go` (14 files) | ~30-80 each | Flag parsing → domain call → print |
| `internal/mcp/server.go` | ~50 | MCP server setup |
| `internal/mcp/tools.go` | ~300 | 10 tool schemas + handlers |
| `cmd/meta/main.go` | ~30 | Wire everything, run |

---

## Dependency Injection

`main.go` is the composition root. It:
1. Loads config
2. Creates `graph.NewClient(cfg)`
3. Passes the client to CLI/MCP handlers
4. Handlers pass it to domain functions

No global state. No init functions. No singletons.

```go
func main() {
    cfg := config.Load()
    client := graph.NewClient(cfg)
    // wire client into cobra commands / MCP server
}
```

---

## Error Flow

```
Meta API → HTTP response → graph/client.go parses into GraphError
  → domain function returns GraphError to caller
    → CLI: output.PrintError(err) → structured JSON to stderr + os.Exit(code)
    → MCP: tool handler returns error in MCP error response
```

`GraphError` is the single error type. It carries everything: message, code, subcode, trace ID, retryability, and the computed exit code. No wrapping in other error types. No error chains. One structured error, propagated as-is.
