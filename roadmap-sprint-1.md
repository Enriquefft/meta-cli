# Sprint 1 Roadmap — Ad Creation Pipeline

## Goal

A single binary (`meta`) that can: validate auth, upload a video, create a campaign → ad set → creative → ad with targeting, and expose the same pipeline as an MCP server for AI agents.

## Architecture

See `architecture.md` for full contracts, interfaces, and import rules.

```
CLI (cobra)  │  MCP (mcp-go)
      ───────┼───────
         internal/meta/     ← domain (single source of truth)
              │
         internal/graph/    ← HTTP transport (implements meta.Client)
              │
         Meta Graph API
```

---

## TDD Cycle

Every feature follows this loop — no exceptions:

```
1. Define types     → XxxParams, XxxResult structs
2. Write interface  → function signature on meta.Client or domain function
3. Write tests      → mock Client, test happy path + error cases + edge cases
4. Implement        → make tests green
5. Integration test → live API call, tagged //go:build integration
6. Verify           → unit + integration both pass
```

Integration tests use the real Meta API with your access token:
```bash
META_ACCESS_TOKEN=EAA... META_AD_ACCOUNT=act_... go test -tags=integration ./...
```

---

## Phase 0 — Project Scaffold

**Purpose**: Compilable skeleton. Zero domain logic.

**Steps**:
1. `go mod init`
2. Create directory structure per `architecture.md`
3. `cmd/meta/main.go` — compiles, prints version, exits
4. `Makefile` — build, install, lint, test, clean
5. `.golangci.yml` — errcheck, govet, staticcheck, unused, ineffassign, misspell, gofmt, goimports

**Test**: `make build` → binary exists. `make lint` → clean. `./bin/meta --version` → prints version.

---

## Phase 1 — Config (`internal/config/`)

**Purpose**: Single source of truth for all configuration.

**TDD steps**:
1. Define `Config` struct: `AccessToken`, `AppID`, `DefaultAccount`, `APIVersion`, `OutputFormat`
2. Write tests: resolution order (flag > env > file > default), set/get persistence, validation
3. Implement: `Load()`, `Set(key, value)`, `Get(key)`, `Validate()` (requires AccessToken for API ops)
4. Integration test: real config file on disk, verify round-trip

**Files**: `internal/config/config.go`, `internal/config/config_test.go`

---

## Phase 2 — Domain Interface + Graph Client

### 2a. Domain interface (`internal/meta/client.go`, `internal/meta/errors.go`)

**TDD steps**:
1. Define `meta.Client` interface (Get, Post, Upload, Paginate)
2. Define `Response`, `PageIterator`, `GraphError` types
3. Define `MockClient` in `internal/meta/mock_test.go` for all future domain tests
4. Write interface conformance test: `var _ Client = (*MockClient)(nil)`

**Files**: `internal/meta/client.go`, `internal/meta/errors.go`, `internal/meta/mock_test.go`

### 2b. Graph client implementation (`internal/graph/`)

**TDD steps per file**:

**`client.go`**:
1. Test: request includes access token, path includes API version, timeout enforced, dry-run injects validate_only
2. Implement against `httptest.Server`

**`errors.go`**:
1. Test: parse Meta error JSON → GraphError, classify exit codes (auth=2, validation=3, network=5, etc.)
2. Implement error parser

**`pagination.go`**:
1. Test: single page, multi-page cursor following, empty result, --all auto-paginate
2. Implement PageIterator

**`upload.go`**:
1. Test: multipart form constructed correctly, small file upload, resumable trigger for >1GB
2. Implement upload handler

**`rate.go`**:
1. Test: parse X-Business-Use-Case-Usage header, slow down at 75%, backoff at limit, retry on 429/500/502/503
2. Implement rate limiter

**Files**: `internal/graph/client.go`, `internal/graph/client_test.go`, `internal/graph/errors.go`, `internal/graph/errors_test.go`, `internal/graph/pagination.go`, `internal/graph/pagination_test.go`, `internal/graph/upload.go`, `internal/graph/upload_test.go`, `internal/graph/rate.go`, `internal/graph/rate_test.go`

**Integration**: `internal/graph/integration_test.go` — real `GET /me` call to verify auth works.

---

## Phase 3 — Output (`internal/output/`)

**TDD steps**:
1. Define `Print(data, format, fields)`, `PrintError(err)` signatures
2. Write tests: JSON pretty-print, table with columns, CSV with header, field filtering, NO_COLOR respect, structured error to stderr
3. Implement

**Files**: `internal/output/output.go`, `internal/output/output_test.go`

---

## Phase 4 — Domain Layer (`internal/meta/`)

Each operation is one file + one test file. Build in this order (each is independently testable):

### 4.1 `auth.go` + `auth_test.go`

- **Types**: `AuthStatusResult` (Valid, User, App, Permissions, ExpiresAt)
- **Tests**: mock returns /me + /me/permissions + /debug_token responses → verify parsed correctly. Error: expired token → AuthError exit code.
- **Integration**: real `meta auth status` call → verify token is valid.

### 4.2 `accounts.go` + `accounts_test.go`

- **Types**: `ListAccountsParams` (Limit), `AdAccount` struct, `ListAccountsResult` (Data + Paging)
- **Tests**: mock returns adaccounts JSON → verify typed result. Pagination: multi-page → collect all. Empty list.
- **Integration**: real `GET /me/adaccounts` → verify response parses.

### 4.3 `pages.go` + `pages_test.go`

- **Types**: `ListPagesParams`, `Page` struct, `ListPagesResult`
- **Tests**: mock returns pages JSON → verify typed result.
- **Integration**: real `GET /me/accounts` → verify response parses.

### 4.4 `assets.go` + `assets_test.go`

- **Types**: `UploadVideoParams` (AccountID, FilePath, Title), `UploadVideoResult` (ID, Title, UploadStatus), `VideoStatusParams` (VideoID), `VideoStatusResult` (ID, Title, Status, Length)
- **Tests**: upload video → mock verifies multipart constructed. Video status → mock returns encoding states (processing, ready, error).
- **Integration**: upload a real test video → check status until ready. (Small test video file committed to testdata/)

### 4.5 `targeting.go` + `targeting_test.go`

- **Types**: `SearchParams` (Type, Query, Limit), `TargetingSuggestion` (ID, Name, Type, AudienceSize, Path), `SearchResult`
- **Tests**: mock returns search JSON → verify parsed. Empty results. Invalid type → validation error.
- **Integration**: real search for "e-commerce" → verify results returned.

### 4.6 `campaigns.go` + `campaigns_test.go`

- **Types**: `CreateCampaignParams` (AccountID, Name, Objective, DailyBudgetCents, LifetimeBudgetCents, BidStrategy, Status, SpecialAdCategory), `Campaign` struct
- **Tests**: mock verifies POST body has correct fields. Dollar-to-cents conversion. Validation: name required, objective required, one budget required. Dry-run injects validate_only.
- **Integration**: real campaign create with `status=PAUSED` → verify created → delete after.

### 4.7 `adsets.go` + `adsets_test.go`

- **Types**: `CreateAdSetParams` (AccountID, Name, CampaignID, budgets, optimization goal, targeting fields, status), `AdSet` struct
- **Tests**: mock verifies targeting spec built correctly (countries, age range, interests, etc.). Dollar-to-cents. Validation: campaign required, countries required, optimization goal required.
- **Integration**: real adset create under campaign from 4.6 → verify targeting.

### 4.8 `creatives.go` + `creatives_test.go`

- **Types**: `CreateCreativeParams` (AccountID, Name, PageID, VideoID, ImageHash, ImageURL, Message, Headline, Description, CTA, Link, InstagramAccountID), `Creative` struct
- **Tests**: mock verifies object_story_spec constructed correctly for video creative, image creative. Validation: page required, message required, link required, one media source required.
- **Integration**: real creative create with video from 4.4 → verify created.

### 4.9 `ads.go` + `ads_test.go`

- **Types**: `CreateAdParams` (AccountID, Name, AdSetID, CreativeID, Status), `Ad` struct
- **Tests**: mock verifies POST body. Validation: name, adset, creative required.
- **Integration**: real ad create linking creative to adset → verify created.

---

## Phase 5 — CLI Commands (`internal/cli/`)

**Purpose**: Thin shell. Parse flags → call domain → print output.

Each command file follows the same pattern:
1. Define cobra command with flags
2. In RunE: parse flags → build domain params → call domain function → output.Print result → handle errors

**Build order**:
1. `root.go` — root command, global flags, wire config + client
2. `config_cmd.go` — set/get
3. `auth.go` — auth status
4. `accounts.go`, `pages.go` — list commands
5. `assets.go` — upload-video, video-status
6. `campaigns.go` — create
7. `adsets.go` — create
8. `creatives.go` — create
9. `ads.go` — create
10. `targeting.go` — search

**Test**: build binary, run each command with `--dry-run` and real token. Verify JSON output is valid. Full pipeline: `meta campaigns create --dry-run ... → meta adsets create --dry-run ... → meta creatives create --dry-run ... → meta ads create --dry-run ...`

---

## Phase 6 — MCP Server (`internal/mcp/`)

**Build order**:

### 6.1 `server.go`
1. Test: server starts, responds to initialize, has correct instructions
2. Implement: MCP server with stdio transport, server instructions

### 6.2 `tools.go`
1. Define all 10 tool schemas (input schemas as JSON Schema objects)
2. Test: each tool handler calls the correct domain function with correct params, returns JSON
3. Implement: 10 tool handlers, each calling internal/meta/ functions

### 6.3 `internal/cli/serve.go`
1. Test: `meta serve` starts and exits cleanly
2. Implement: serve command that starts MCP server

**Tools**: meta_auth_status, meta_list_accounts, meta_list_pages, meta_upload_video, meta_video_status, meta_create_campaign, meta_create_adset, meta_create_creative, meta_create_ad, meta_search_targeting

---

## Phase 7 — Infrastructure

- `flake.nix` — dev shell
- `lefthook.yml` — pre-commit: lint + test; commit-msg: conventional commits
- `.commitlintrc.yaml`
- `goreleaser.yaml` — cross-compile
- `.envrc`

**Acceptance**: clean clone → `direnv allow` → `make build && make lint && make test` → all green.

---

## Dependency Graph

```
Phase 0 (scaffold)
  └→ Phase 1 (config) — tests first
       └→ Phase 2 (domain interface + graph client) — tests first
            └→ Phase 3 (output) — tests first
                 └→ Phase 4 (domain) — each op: types → test → impl → integration test
                      └→ Phase 5 (CLI)
                      └→ Phase 6 (MCP)
       └→ Phase 7 (infra) — parallel with 4-6
```

---

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Graph client | Interface in `internal/meta/`, impl in `internal/graph/` | Domain owns abstraction. Fully testable without HTTP. |
| Budget input | Dollars (float64) at CLI/MCP, cents (int64) at domain | Users think in dollars. Domain speaks API. Conversion at shell boundary. |
| Types location | Co-located with operation file | No shared model package. Single source of truth per operation. |
| Pagination | `--all` from day one | Enterprise scripts must not manually paginate. |
| Error output | Structured JSON to stderr | Machine-parseable. Not just exit codes. |
| No Meta SDK | Raw `net/http` | REST/JSON API. SDK = indirection without value. |
| TDD | Types → tests → impl → integration | Every line of implementation justified by a test first. |
| Integration tests | `//go:build integration` + real API | Live token validates against real Meta API. Not CI-dependent. |

---

## Integration Test Completion Phases (Full Finish)

This section splits the remaining implementation and hardening work to fully finish integration testing for the core flow:

1. Upload downloaded video to ad account
2. Create campaign → ad set → creative → ad
3. Apply audience targeting (country, age, interests)
4. Link creative CTA to product landing page / store

### Current baseline

- Domain/transport integration exists in `internal/graph/integration_test.go`, including a full pipeline test.
- CLI/MCP layers do not yet have equivalent live integration coverage for the full pipeline.
- Several CLI test files need stabilization before the suite can be treated as reliable.

### Phase 1 — Stabilize test suite correctness

**Purpose**: Ensure test files are structurally valid and aligned with current command constructors.

**Implementation scope**:
- Repair malformed CLI test files (syntax, imports, function names, helper calls).
- Align tests to exported constructors (`NewCampaignsCommand`, `NewCreativesCommand`, etc.).
- Remove dead/duplicated fragments and keep one canonical test per command behavior.

**Exit criteria**:
- CLI test package compiles cleanly.
- No broken/duplicate test files remain.

### Phase 2 — Shared integration harness

**Purpose**: Create one reusable, deterministic foundation for live integration tests.

**Implementation scope**:
- Add shared helpers for required env vars, unique resource naming, and polling with bounded timeout.
- Standardize test preflight checks (token/account/page availability, required permissions).
- Add consistent skip behavior for sandbox-restricted endpoints.

**Exit criteria**:
- All integration tests use common helpers (no copy-pasted setup logic).
- Polling/retry behavior is centralized and bounded.

### Phase 3 — Harden domain-level pipeline integration

**Purpose**: Make the existing domain/graph live pipeline test production-grade and less brittle.

**Implementation scope**:
- Keep the end-to-end domain test for upload → campaign → adset → creative → ad.
- Replace brittle assumptions where possible (resource selection and targeting setup).
- Explicitly assert targeting payload outcomes (country/age/interests) and creative link fields.

**Exit criteria**:
- Domain integration validates all required flow attributes, not just resource IDs.
- Test output clearly shows each pipeline step and failure point.

### Phase 4 — Add CLI live integration pipeline

**Purpose**: Prove the user-facing CLI can run the same full workflow end-to-end.

**Implementation scope**:
- Add `//go:build integration` CLI tests that execute command flow using real credentials.
- Verify command outputs are parseable and chain IDs between steps.
- Assert CTA link and targeting values are correctly propagated through CLI flags.

**Exit criteria**:
- One CLI integration test covers full flow with real API calls.
- CLI path verifies upload, campaign, adset targeting, creative link, and final ad creation.

### Phase 5 — Add MCP live integration pipeline

**Purpose**: Validate the same business flow through MCP tools, not only direct domain calls.

**Implementation scope**:
- Add live integration tests for the MCP tool handlers in pipeline sequence.
- Verify tool input/output contracts for account/video/campaign/adset/creative/ad chaining.
- Confirm targeting and destination link are preserved through MCP argument mapping.

**Exit criteria**:
- One MCP integration test covers the complete flow and passes with live env.
- Tool-level contract assertions exist for key fields (targeting + link + IDs).

### Phase 6 — Cleanup and resource lifecycle guarantees

**Purpose**: Prevent account pollution and make repeated runs safe.

**Implementation scope**:
- Add deterministic cleanup for test-created resources where API allows it.
- Ensure cleanup runs on both success and failure paths.
- Prefix and tag all test assets for discoverability.

**Exit criteria**:
- Integration runs leave no orphaned campaigns/adsets/ads in normal execution.
- Failed runs are still easy to clean up due to deterministic naming/tagging.

### Phase 7 — Negative-path and resilience coverage

**Purpose**: Validate failure handling for realistic integration failures.

**Implementation scope**:
- Add live/near-live cases for invalid targeting, invalid landing URL, and media-not-ready creative creation.
- Verify surfaced errors remain structured and actionable.
- Cover permission/sandbox constraints with explicit assertions or skip semantics.

**Exit criteria**:
- Core failure modes are tested and produce expected error classification.
- No silent failures in pipeline orchestration.

### Phase 8 — Continuous verification gates

**Purpose**: Make integration quality persistent, not one-off.

**Implementation scope**:
- Define canonical commands for local and CI live runs (`-tags=integration` subsets).
- Document required env matrix and account prerequisites for contributors.
- Add gating policy: unit always, integration on controlled triggers/environments.

**Exit criteria**:
- Team has one documented, repeatable way to run the full integration suite.
- Integration regressions are detectable before release.
