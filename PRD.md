# meta-cli — Product Requirements Document

## 1. Problem Statement

Dropshipping operators need to move fast: spot a trending product on TikTok, source it, and launch paid ads on Meta (Facebook/Instagram) before the trend cools. Today this last step — creating Meta ads — is either manual (slow, error-prone) or requires stitching together half-baked tools that cover fragments of the API.

No single CLI tool exists that covers the Meta Marketing API surface with production-grade reliability, structured output for LLM/AI agent consumption, and an integrated MCP server.

## 2. Vision

**meta-cli** is the de-facto command-line interface and MCP server for the Meta Marketing API. It is to Meta Ads what `gh` is to GitHub or `kubectl` is to Kubernetes — a complete, reliable, scriptable interface that becomes the standard way to interact with Meta's ad platform from the terminal or an AI agent.

## 3. Scope Boundary

The Meta platform exposes many API surfaces. This tool covers **Marketing API only** — everything a performance marketer or ad automation system needs to spend money and measure results.

### In Scope

| Area | Coverage |
|------|----------|
| **Ad object hierarchy** | Campaigns, ad sets, ads, ad creatives — full CRUD |
| **Asset management** | Image and video upload, encoding status, thumbnails |
| **Targeting & audiences** | Interest/behavior/demographic search, custom audiences, lookalikes, reach estimation |
| **Insights & reporting** | Account/campaign/adset/ad level metrics, breakdowns, date ranges, async reports |
| **Pixels & conversions** | Pixel management, server-side Conversions API events |
| **Product catalogs** | Catalog management, product feeds, product sets (for dynamic product ads) |
| **Business accounts** | Only what's needed for ad account access and permissions |

### Out of Scope

| Surface | Why |
|---------|-----|
| **Graph API (social)** | Page posts, comments, reactions — different workflow, different tools |
| **Instagram Graph API** | IG media/stories/reels management — content creation, not advertising |
| **WhatsApp Business API** | Messaging platform, has dedicated CLIs |
| **Workplace API** | Enterprise collaboration, unrelated |
| **Threads API** | Social posting, not marketing |

**Rationale**: Focus wins. Every command in `meta-cli` relates to one mental model: *spending money to reach people*. No context-switching between "post a photo" and "create a campaign."

## 4. Target Users

| User | Use Case |
|------|----------|
| **Dropshippers** | Automate ad creation pipeline: trending product → video → live ad |
| **Performance marketers** | Bulk campaign management, reporting, A/B testing from terminal |
| **Agencies** | Manage multiple ad accounts programmatically |
| **AI agents (via MCP)** | Autonomous ad creation, optimization, and reporting |
| **Developers** | Integrate Meta Ads into CI/CD, scripts, and automation pipelines |

## 5. Language & Runtime

**Go 1.23+**

| Criterion | Go |
|-----------|-----|
| Single binary distribution | Yes — zero runtime dependency |
| Cross-compilation | Trivial (`GOOS=linux GOARCH=amd64`) |
| CLI ecosystem | Gold standard (`cobra`, `viper`) |
| Concurrency | Goroutines for parallel uploads/batch ops |
| MCP support | `mcp-go` library |
| Precedent | `gh`, `kubectl`, `docker`, `terraform` |
| Build speed | Fast iteration cycles |

## 6. Architecture

### 6.1 Deep Module Design

Following John Ousterhout's "A Philosophy of Software Design": few modules with deep interfaces that hide substantial complexity behind simple APIs.

```
                    ┌───────────────┐     ┌───────────────┐
                    │  CLI (cobra)  │     │  MCP Server   │
                    │  Thin: parse  │     │  Thin: schema │
                    │  flags, call  │     │  validate,    │
                    │  domain, print│     │  call domain  │
                    └──────┬────────┘     └──────┬────────┘
                           │                     │
                           ▼                     ▼
                    ┌────────────────────────────────────┐
                    │         internal/meta/              │
                    │                                     │
                    │  DEEP: Domain operations            │
                    │  Typed params → Typed results       │
                    │  Hides: field mapping, object IDs,  │
                    │  validation, param encoding         │
                    │                                     │
                    │  campaigns.go   adsets.go   ads.go  │
                    │  creatives.go   assets.go   ...     │
                    └──────────────┬─────────────────────┘
                                   │
                                   ▼
                    ┌────────────────────────────────────┐
                    │         internal/graph/             │
                    │                                     │
                    │  DEEP: HTTP transport layer         │
                    │  Hides: auth injection, API         │
                    │  versioning, rate limit tracking,   │
                    │  retry logic, cursor pagination,    │
                    │  multipart upload, error parsing,   │
                    │  dry-run injection, field selection  │
                    │                                     │
                    │  Interface:                         │
                    │    Get(path, params) → Response     │
                    │    Post(path, params) → Response    │
                    │    Upload(path, file) → Response    │
                    │    Paginate(path) → Iterator        │
                    └──────────────┬─────────────────────┘
                                   │
                                   ▼
                         Meta Graph API v21.0
```

**Single source of truth**: `internal/meta/` is the domain layer. Both CLI commands and MCP tools call it directly. No subprocess indirection, no duplication.

### 6.2 Project Structure

```
meta-cli/
├── cmd/
│   └── meta/
│       └── main.go                     # Entry point — wires cobra root, runs
├── internal/
│   ├── cli/                            # CLI command definitions (thin)
│   │   ├── root.go                     # Root command, global flags, config init
│   │   ├── auth.go                     # meta auth *
│   │   ├── accounts.go                 # meta accounts *
│   │   ├── campaigns.go                # meta campaigns *
│   │   ├── adsets.go                   # meta adsets *
│   │   ├── ads.go                      # meta ads *
│   │   ├── creatives.go                # meta creatives *
│   │   ├── assets.go                   # meta assets *
│   │   ├── targeting.go                # meta targeting *
│   │   ├── pages.go                    # meta pages *
│   │   ├── insights.go                 # meta insights *
│   │   ├── audiences.go                # meta audiences *
│   │   ├── pixels.go                   # meta pixels *
│   │   ├── catalogs.go                 # meta catalogs *
│   │   ├── conversions.go              # meta conversions *
│   │   ├── business.go                 # meta business *
│   │   ├── config_cmd.go               # meta config *
│   │   └── serve.go                    # meta serve (MCP)
│   ├── graph/                          # HTTP transport (deep module)
│   │   ├── client.go                   # Core client: auth, versioning, retry
│   │   ├── errors.go                   # Meta error types & classification
│   │   ├── pagination.go               # Cursor-based page iterator
│   │   └── upload.go                   # Multipart + resumable upload
│   ├── meta/                           # Domain operations (deep module)
│   │   ├── accounts.go                 # AdAccount ops + types
│   │   ├── campaigns.go                # Campaign CRUD + types
│   │   ├── adsets.go                   # AdSet CRUD + types
│   │   ├── ads.go                      # Ad CRUD + types
│   │   ├── creatives.go                # AdCreative CRUD + types
│   │   ├── assets.go                   # Image/Video upload + types
│   │   ├── targeting.go                # Search + reach estimation + types
│   │   ├── pages.go                    # Page ops + types
│   │   ├── insights.go                 # Reporting + types
│   │   ├── audiences.go                # Custom audience ops + types
│   │   ├── pixels.go                   # Pixel ops + types
│   │   ├── catalogs.go                 # Product catalog + feed + set ops + types
│   │   ├── conversions.go              # Server-side event ops + types
│   │   └── business.go                 # Business account/user ops + types
│   ├── mcp/                            # MCP server (thin)
│   │   ├── server.go                   # MCP setup, stdio transport
│   │   └── tools.go                    # Tool schemas → call internal/meta/
│   ├── config/                         # Config resolution (deep module)
│   │   └── config.go                   # flags > env > file > defaults
│   └── output/                         # Output formatting (deep module)
│       └── output.go                   # JSON / table / CSV + field filtering
├── go.mod
├── go.sum
├── Makefile
├── goreleaser.yaml
├── flake.nix
├── .envrc
├── .golangci.yml
├── lefthook.yml
├── .commitlintrc.yaml
├── LICENSE
├── README.md
└── PRD.md
```

## 7. Feature Specification

### 7.1 Full Command Tree

Every command the CLI will support. Sprint 1 commands marked with `[S1]`.

```
meta
│
├── auth
│   └── status                [S1]    # Token validity, permissions, expiry
│
├── config
│   ├── set <key> <value>     [S1]    # Set config value
│   ├── get <key>             [S1]    # Get config value
│   └── list                          # List all config
│
├── accounts
│   ├── list                  [S1]    # List all ad accounts
│   └── get <id>                      # Account details + spending
│
├── pages
│   ├── list                  [S1]    # List pages you manage
│   └── get <id>                      # Page details
│
├── assets
│   ├── upload-video          [S1]    # Upload video to ad account
│   ├── upload-image                  # Upload image to ad account
│   ├── video-status <id>    [S1]    # Check video encoding status
│   ├── list-videos                   # List uploaded videos
│   ├── list-images                   # List uploaded images
│   └── thumbnails <id>              # Get video thumbnails
│
├── campaigns
│   ├── create                [S1]    # Create campaign
│   ├── list                          # List campaigns (filterable)
│   ├── get <id>                      # Campaign details
│   ├── update <id>                   # Update campaign fields
│   ├── delete <id>                   # Archive/delete campaign
│   └── duplicate <id>               # Duplicate campaign
│
├── adsets
│   ├── create                [S1]    # Create ad set with targeting
│   ├── list                          # List ad sets (filterable)
│   ├── get <id>                      # Ad set details
│   ├── update <id>                   # Update ad set fields
│   └── delete <id>                   # Archive/delete ad set
│
├── ads
│   ├── create                [S1]    # Create ad
│   ├── list                          # List ads (filterable)
│   ├── get <id>                      # Ad details
│   ├── update <id>                   # Update ad fields
│   └── delete <id>                   # Archive/delete ad
│
├── creatives
│   ├── create                [S1]    # Create ad creative
│   ├── list                          # List creatives
│   ├── get <id>                      # Creative details
│   └── delete <id>                   # Delete creative
│
├── targeting
│   ├── search                [S1]    # Search interests/behaviors/demographics
│   ├── reach                         # Estimate audience reach
│   └── suggestions                   # Get targeting suggestions
│
├── insights
│   ├── account <id>                  # Account-level reporting
│   ├── campaign <id>                 # Campaign-level reporting
│   ├── adset <id>                    # Ad set-level reporting
│   └── ad <id>                       # Ad-level reporting
│
├── audiences
│   ├── list                          # List custom audiences
│   ├── get <id>                      # Audience details
│   ├── create                        # Create custom audience
│   ├── create-lookalike              # Create lookalike from source audience
│   └── delete <id>                   # Delete audience
│
├── pixels
│   ├── list                          # List pixels
│   ├── get <id>                      # Pixel details
│   └── events <id>                   # List recent pixel events
│
├── conversions
│   ├── send                          # Send server-side event(s) via Conversions API
│   ├── test                          # Send test event to verify pixel integration
│   └── deduplicate                   # Check event deduplication status
│
├── catalogs
│   ├── list                          # List product catalogs
│   ├── get <id>                      # Catalog details
│   ├── create                        # Create product catalog
│   ├── delete <id>                   # Delete catalog
│   ├── products list <catalog_id>    # List products in catalog
│   ├── products get <product_id>     # Get product details
│   ├── products add <catalog_id>     # Add product to catalog
│   ├── products update <product_id>  # Update product fields
│   ├── products delete <product_id>  # Remove product from catalog
│   ├── feeds list <catalog_id>       # List product feeds
│   ├── feeds create <catalog_id>     # Create product feed (URL or file)
│   ├── feeds upload <feed_id>        # Upload feed file
│   └── sets list <catalog_id>        # List product sets
│
├── business
│   ├── list                          # List business accounts
│   ├── get <id>                      # Business account details
│   ├── users <id>                    # List users in business account
│   └── ad-accounts <id>             # List ad accounts in business
│
└── serve                     [S1]    # Start MCP server (stdio transport)
```

### 7.2 Global Flags

| Flag | Short | Env Var | Config Key | Default | Description |
|------|-------|---------|------------|---------|-------------|
| `--account` | `-a` | `META_AD_ACCOUNT` | `default_account` | — | Ad account ID |
| `--format` | `-f` | — | `output_format` | `json` | Output: `json`, `table`, `csv` |
| `--fields` | — | — | — | — | Comma-separated field selection |
| `--dry-run` | — | — | — | `false` | Validate without creating |
| `--verbose` | `-v` | — | — | `false` | Show HTTP details |
| `--version` | — | — | — | — | Print version and exit |
| `--no-color` | — | `NO_COLOR` | — | `false` | Disable color output |

### 7.3 Config System

**File location**: `~/.config/meta-cli/config.yaml`

**Resolution order** (highest priority wins):
1. CLI flags
2. Environment variables
3. Config file
4. Built-in defaults

**Config keys**:
```yaml
# Authentication
access_token: ""              # META_ACCESS_TOKEN
app_id: ""                    # META_APP_ID

# Defaults
default_account: ""           # META_AD_ACCOUNT
api_version: "v21.0"          # META_API_VERSION
output_format: "json"         # json | table | csv
```

**Environment variable mapping**:
| Env Var | Config Key |
|---------|------------|
| `META_ACCESS_TOKEN` | `access_token` |
| `META_APP_ID` | `app_id` |
| `META_AD_ACCOUNT` | `default_account` |
| `META_API_VERSION` | `api_version` |

## 8. Sprint 1 — Detailed Specifications

### 8.1 `meta auth status`

**Purpose**: Verify the access token works and show what it can do.

**API calls**:
- `GET /me` — validate token, get user identity
- `GET /me/permissions` — list granted permissions
- `GET /debug_token?input_token={token}` — expiry, scopes, app info

**Output (JSON)**:
```json
{
  "valid": true,
  "user": { "id": "10164900255584540", "name": "Frankz Kastner Cam" },
  "app": { "id": "1592995508651325", "name": "RepliClaw" },
  "permissions": ["ads_management", "ads_read", "read_insights", "business_management"],
  "expires_at": "2026-04-15T00:00:00Z",
  "scopes": ["ads_management", "ads_read", "read_insights", "business_management"]
}
```

### 8.2 `meta accounts list`

**Purpose**: List all ad accounts the token has access to.

**API call**: `GET /me/adaccounts?fields=account_id,name,account_status,currency,timezone_name,amount_spent,balance`

**Flags**:
| Flag | Type | Description |
|------|------|-------------|
| `--limit` | int | Max results (default 25) |

**Output (JSON)**:
```json
{
  "data": [
    {
      "id": "act_1817754108935948",
      "account_id": "1817754108935948",
      "name": "My Store Ads",
      "account_status": 1,
      "currency": "USD",
      "timezone_name": "America/Los_Angeles",
      "amount_spent": "15234",
      "balance": "0"
    }
  ],
  "paging": { "has_next": true, "cursor": "..." }
}
```

### 8.3 `meta pages list`

**Purpose**: List Facebook Pages the user manages (required for creating ad creatives).

**API call**: `GET /me/accounts?fields=id,name,category,access_token`

**Output (JSON)**:
```json
{
  "data": [
    {
      "id": "123456789",
      "name": "My Store",
      "category": "E-commerce website",
      "access_token": "EAA..."
    }
  ]
}
```

### 8.4 `meta assets upload-video`

**Purpose**: Upload a video file to an ad account for use in ad creatives.

**API call**: `POST /act_{account_id}/advideos` (multipart)

**Flags**:
| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--file` | string | yes | Path to video file |
| `--title` | string | no | Video title |

**Behavior**:
- Detects file size; if > 1GB, uses resumable upload (chunked)
- Shows upload progress on stderr (doesn't pollute JSON stdout)
- Returns immediately — video still encoding. Use `video-status` to check.

**Output (JSON)**:
```json
{
  "id": "1234567890",
  "title": "product-video.mp4",
  "upload_status": "processing"
}
```

### 8.5 `meta assets video-status`

**Purpose**: Check if an uploaded video has finished encoding and is ready for ads.

**API call**: `GET /{video_id}?fields=id,title,status,length,source`

**Output (JSON)**:
```json
{
  "id": "1234567890",
  "title": "product-video.mp4",
  "status": {
    "video_status": "ready",
    "processing_progress": 100
  },
  "length": 30.5
}
```

### 8.6 `meta campaigns create`

**Purpose**: Create an ad campaign.

**API call**: `POST /act_{account_id}/campaigns`

**Flags**:
| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--name` | string | yes | — | Campaign name |
| `--objective` | enum | yes | — | `OUTCOME_SALES`, `OUTCOME_TRAFFIC`, `OUTCOME_ENGAGEMENT`, `OUTCOME_AWARENESS`, `OUTCOME_LEADS`, `OUTCOME_APP_PROMOTION` |
| `--daily-budget` | int | no* | — | Daily budget in cents |
| `--lifetime-budget` | int | no* | — | Lifetime budget in cents |
| `--bid-strategy` | enum | no | `LOWEST_COST_WITHOUT_CAP` | Bid strategy |
| `--status` | enum | no | `PAUSED` | `PAUSED` or `ACTIVE` |
| `--special-ad-category` | enum | no | `NONE` | `NONE`, `HOUSING`, `CREDIT`, `EMPLOYMENT`, `ISSUES_ELECTIONS_POLITICS` |

*One of `--daily-budget` or `--lifetime-budget` required.

**Output (JSON)**:
```json
{
  "id": "120212345678901234",
  "name": "My Conversions Campaign",
  "objective": "OUTCOME_SALES",
  "status": "PAUSED",
  "daily_budget": "5000"
}
```

### 8.7 `meta adsets create`

**Purpose**: Create an ad set with targeting, budget, and schedule.

**API call**: `POST /act_{account_id}/adsets`

**Flags**:
| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--name` | string | yes | — | Ad set name |
| `--campaign` | string | yes | — | Campaign ID |
| `--daily-budget` | int | no | — | Daily budget in cents |
| `--lifetime-budget` | int | no | — | Lifetime budget in cents |
| `--optimization-goal` | enum | yes | — | See optimization goals table |
| `--billing-event` | enum | no | `IMPRESSIONS` | `IMPRESSIONS`, `LINK_CLICKS`, `THRUPLAY` |
| `--bid-amount` | int | no | — | Bid cap in cents |
| `--countries` | []string | yes | — | Target countries (ISO codes) |
| `--age-min` | int | no | `18` | Minimum age |
| `--age-max` | int | no | `65` | Maximum age |
| `--genders` | []int | no | `[1,2]` | 1=male, 2=female |
| `--interests` | []string | no | — | Interest IDs (from targeting search) |
| `--behaviors` | []string | no | — | Behavior IDs |
| `--custom-audiences` | []string | no | — | Custom audience IDs |
| `--excluded-countries` | []string | no | — | Excluded countries |
| `--publisher-platforms` | []string | no | — | `facebook`, `instagram`, `messenger` |
| `--pixel-id` | string | no | — | Pixel ID for conversion tracking |
| `--custom-event-type` | string | no | — | Event type: `PURCHASE`, `ADD_TO_CART`, etc. |
| `--start-time` | string | no | — | ISO 8601 start time |
| `--end-time` | string | no | — | ISO 8601 end time |
| `--status` | enum | no | `PAUSED` | `PAUSED` or `ACTIVE` |

**Optimization goals**: `OFFSITE_CONVERSIONS`, `LINK_CLICKS`, `LANDING_PAGE_VIEWS`, `REACH`, `IMPRESSIONS`, `APP_INSTALLS`, `LEAD_GENERATION`, `THRUPLAY`, `VALUE`, `PAGE_LIKES`, `POST_ENGAGEMENT`

**Output (JSON)**:
```json
{
  "id": "120212345678901234",
  "name": "US 18-45 Shopping Interest",
  "campaign_id": "120212345678901234",
  "status": "PAUSED",
  "daily_budget": "5000",
  "targeting": {
    "geo_locations": { "countries": ["US"] },
    "age_min": 18,
    "age_max": 45,
    "interests": [{ "id": "6003276047589", "name": "E-commerce" }]
  }
}
```

### 8.8 `meta creatives create`

**Purpose**: Create an ad creative with video, copy, and CTA.

**API call**: `POST /act_{account_id}/adcreatives`

**Flags**:
| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--name` | string | no | Creative name |
| `--page` | string | yes | Facebook Page ID |
| `--video` | string | yes* | Video ID (from upload) |
| `--image-hash` | string | yes* | Image hash (for image ads) |
| `--image-url` | string | yes* | Image URL (alternative to hash) |
| `--message` | string | yes | Primary text above the ad |
| `--headline` | string | no | Headline below media |
| `--description` | string | no | Description text |
| `--cta` | enum | no | `SHOP_NOW` | CTA button type |
| `--link` | string | yes | Destination URL |
| `--instagram-account` | string | no | Instagram account ID for IG placement |

*One of `--video`, `--image-hash`, or `--image-url` required.

**CTA types**: `SHOP_NOW`, `LEARN_MORE`, `SIGN_UP`, `BOOK_NOW`, `DOWNLOAD`, `GET_OFFER`, `ORDER_NOW`, `BUY_NOW`, `SUBSCRIBE`, `CONTACT_US`, `WATCH_MORE`, `APPLY_NOW`, `GET_QUOTE`, `SEND_MESSAGE`

**Output (JSON)**:
```json
{
  "id": "120212345678901234",
  "name": "Video Ad - Trending Product",
  "object_story_spec": {
    "page_id": "123456789",
    "video_data": {
      "video_id": "1234567890",
      "message": "Check this out!",
      "title": "Trending Product",
      "call_to_action": { "type": "SHOP_NOW", "value": { "link": "https://mystore.com" } }
    }
  }
}
```

### 8.9 `meta ads create`

**Purpose**: Create an ad that links a creative to an ad set.

**API call**: `POST /act_{account_id}/ads`

**Flags**:
| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--name` | string | yes | — | Ad name |
| `--adset` | string | yes | — | Ad set ID |
| `--creative` | string | yes | — | Creative ID |
| `--status` | enum | no | `PAUSED` | `PAUSED` or `ACTIVE` |

**Output (JSON)**:
```json
{
  "id": "120212345678901234",
  "name": "Video Ad v1",
  "adset_id": "120212345678901234",
  "creative": { "id": "120212345678901234" },
  "status": "PAUSED"
}
```

### 8.10 `meta targeting search`

**Purpose**: Search for targeting options (interests, behaviors, demographics) to use in ad sets.

**API call**: `GET /search?type={type}&q={query}`

**Flags**:
| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--type` | enum | yes | — | `interests`, `behaviors`, `demographics`, `education_schools`, `education_majors`, `work_employers`, `work_positions` |
| `--query` | string | yes | — | Search term |
| `--limit` | int | no | 25 | Max results |

**Output (JSON)**:
```json
{
  "data": [
    {
      "id": "6003276047589",
      "name": "E-commerce",
      "type": "interests",
      "audience_size_lower_bound": 500000000,
      "audience_size_upper_bound": 600000000,
      "path": ["Interests", "Shopping and fashion", "E-commerce"]
    }
  ]
}
```

### 8.11 `meta config set` / `meta config get`

**Purpose**: Manage persistent configuration.

**Examples**:
```bash
meta config set default_account act_1817754108935948
meta config set access_token EAAWo0...
meta config get default_account
```

**Output**:
```json
{ "key": "default_account", "value": "act_1817754108935948" }
```

### 8.12 `meta serve`

**Purpose**: Start the MCP server on stdio for AI agent integration.

**Behavior**:
- Reads config from same config file / env vars
- Speaks JSON-RPC over stdin/stdout (MCP protocol)
- Exposes tools listed in section 8

## 9. MCP Server Specification

### 9.1 Sprint 1 Tools

| Tool Name | Maps To | Description for LLM |
|-----------|---------|---------------------|
| `meta_auth_status` | `meta auth status` | Check if the Meta API access token is valid and list permissions |
| `meta_list_accounts` | `meta accounts list` | List all ad accounts accessible with the current token |
| `meta_list_pages` | `meta pages list` | List Facebook Pages you manage (required for ad creatives) |
| `meta_upload_video` | `meta assets upload-video` | Upload a video file to an ad account for use in ads |
| `meta_video_status` | `meta assets video-status` | Check if an uploaded video has finished encoding |
| `meta_create_campaign` | `meta campaigns create` | Create a new ad campaign with objective and budget |
| `meta_create_adset` | `meta adsets create` | Create an ad set with targeting, budget, and schedule |
| `meta_create_creative` | `meta creatives create` | Create an ad creative with video/image, copy, and CTA |
| `meta_create_ad` | `meta ads create` | Create an ad linking a creative to an ad set |
| `meta_search_targeting` | `meta targeting search` | Search for interests, behaviors, or demographics for targeting |

### 9.2 Tool Design Principles

1. **Rich descriptions**: Every tool has a detailed description explaining what it does, when to use it, and what prerequisites exist (e.g., "You must upload a video before creating a video creative")
2. **Typed schemas**: Every parameter has a type, description, and validation
3. **Sequencing hints**: Tool descriptions explain the dependency chain: accounts → campaign → ad set → creative → ad
4. **JSON output**: All tools return structured JSON (same format as CLI)
5. **Error clarity**: Errors include the Meta error message and a suggestion for how to fix

### 9.3 MCP Server Instructions

Provided in the server's `instructions` field, read by the AI agent on connection:

```
You are connected to the Meta Marketing API via meta-cli.

The ad creation hierarchy is strict:
  Ad Account → Campaign → Ad Set → Ad Creative → Ad

Typical workflow:
1. meta_auth_status — verify token works
2. meta_list_accounts — find the right ad account
3. meta_list_pages — get page_id (required for creatives)
4. meta_upload_video — upload the video asset
5. meta_video_status — wait until encoding completes (status: "ready")
6. meta_search_targeting — find interest/behavior IDs for audience
7. meta_create_campaign — set objective and budget
8. meta_create_adset — define targeting and schedule
9. meta_create_creative — build the ad creative with video + copy + CTA
10. meta_create_ad — link creative to ad set

Always create campaigns and ads as PAUSED first. Budgets are in cents (5000 = $50).
```

## 10. API Client Specification (internal/graph/)

### 10.1 Client Interface

```go
type Client struct { /* hidden */ }

func NewClient(cfg Config) *Client

func (c *Client) Get(ctx context.Context, path string, params url.Values) (*Response, error)
func (c *Client) Post(ctx context.Context, path string, params map[string]string) (*Response, error)
func (c *Client) Upload(ctx context.Context, path string, file io.Reader, filename string, params map[string]string) (*Response, error)
func (c *Client) Paginate(ctx context.Context, path string, params url.Values) *PageIterator
```

### 10.2 Hidden Complexity

| Concern | Implementation |
|---------|---------------|
| **Auth** | Access token appended to every request automatically |
| **API versioning** | Path prefixed with configured version (e.g., `/v21.0/`) |
| **Rate limiting** | Parse `X-Business-Use-Case-Usage` header; when usage > 75%, slow down; at limit, backoff and retry |
| **Retry** | Exponential backoff: 1s, 2s, 4s. Max 3 retries. Only on 429, 500, 502, 503 |
| **Error parsing** | Meta JSON error → typed `GraphError` with code, subcode, message, trace ID |
| **Pagination** | `PageIterator` follows `paging.next` cursor automatically |
| **Dry-run** | When enabled, injects `execution_options: ["validate_only"]` into POST params |
| **Logging** | `--verbose` enables request/response logging via `slog` |
| **Timeouts** | Per-request context timeout; uploads get longer timeout |
| **Multipart upload** | Files > 1GB use chunked/resumable upload protocol |

### 10.3 Error Types

```go
type GraphError struct {
    Message    string `json:"message"`
    Type       string `json:"type"`
    Code       int    `json:"code"`
    Subcode    int    `json:"error_subcode"`
    TraceID    string `json:"fbtrace_id"`
    IsRetryable bool  // computed
}
```

**Exit codes**:
| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | API error (general) |
| 2 | Authentication error (expired/invalid token) |
| 3 | Validation error (bad params) |
| 4 | Configuration error (missing required config) |
| 5 | Network error (timeout, connection refused) |

## 11. Output Specification (internal/output/)

### 11.1 Formats

| Format | Flag | Use Case |
|--------|------|----------|
| `json` | `--format json` (default) | Scripting, LLM consumption, piping to `jq` |
| `table` | `--format table` | Human terminal usage |
| `csv` | `--format csv` | Spreadsheet export |

### 11.2 Behavior

- **JSON**: Pretty-printed to stdout. Errors go to stderr.
- **Table**: Auto-sized columns, color-coded status (ACTIVE=green, PAUSED=yellow). Respects `NO_COLOR`.
- **CSV**: Header row + data rows. No color.
- **`--fields`**: Filters output to specified fields only. Works with all formats.

### 11.3 Pagination Output

For list commands, pagination metadata is included:
```json
{
  "data": [...],
  "paging": {
    "has_next": true,
    "cursor": "abc123"
  }
}
```

Use `--limit` to control page size. Use `--after <cursor>` for next page. Future: `--all` to auto-paginate and collect everything.

## 12. Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/spf13/cobra` | latest | CLI framework |
| `github.com/spf13/viper` | latest | Config (file + env + flags) |
| `github.com/mark3labs/mcp-go` | latest | MCP server protocol |
| `github.com/charmbracelet/lipgloss` | latest | Terminal table styling |
| `github.com/charmbracelet/bubbles/table` | latest | Table rendering |
| `golang.org/x/time/rate` | latest | Rate limiter |

No Meta SDK — raw HTTP via `net/http`. The Graph API is REST/JSON; an SDK adds indirection without value.

## 13. Build, Lint, Release

### 13.1 Makefile

```makefile
.PHONY: build install lint test clean

build:
	go build -o bin/meta ./cmd/meta

install:
	go install ./cmd/meta

lint:
	golangci-lint run

test:
	go test ./...

clean:
	rm -rf bin/
```

### 13.2 golangci-lint

Strict config matching user's golang-template: errcheck, govet, staticcheck, unused, ineffassign, misspell, gofmt, goimports.

### 13.3 GoReleaser

Cross-compile for:
- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`

Publish to GitHub Releases. Future: Homebrew tap, AUR package.

### 13.4 Git Hooks (Lefthook)

- **pre-commit**: `golangci-lint run`, `go test ./...`
- **commit-msg**: commitlint (conventional commits)

### 13.5 Nix Flake

Dev shell with: `go`, `golangci-lint`, `goreleaser`, `lefthook`, `commitlint`

## 14. Sprint Roadmap

### Sprint 1 (Current) — Ad Creation Pipeline
- `auth status`, `accounts list`, `pages list`
- `assets upload-video`, `assets video-status`
- `campaigns create`, `adsets create`, `creatives create`, `ads create`
- `targeting search`
- `config set/get`
- `serve` (MCP with 10 tools)

### Sprint 2 — Full CRUD + Read
- `list`, `get`, `update`, `delete` for campaigns, adsets, ads, creatives
- `assets upload-image`, `assets list-*`, `assets thumbnails`
- `config list`
- Expand MCP tools to cover all new commands

### Sprint 3 — Insights & Reporting
- `insights account/campaign/adset/ad` with date ranges, breakdowns, time increments
- Async insights for large reports
- MCP insight tools

### Sprint 4 — Audiences & Targeting
- `audiences list/get/create/delete`, `audiences create-lookalike`
- `pixels list/get/events`
- `targeting reach`, `targeting suggestions`
- `campaigns duplicate`
- `--all` auto-pagination flag

### Sprint 5 — Conversions API
- `conversions send` — server-side event tracking (purchase, add-to-cart, etc.)
- `conversions test` — test event verification
- `conversions deduplicate` — deduplication status
- MCP tools for conversion tracking

### Sprint 6 — Product Catalogs
- `catalogs list/get/create/delete`
- `catalogs products list/get/add/update/delete`
- `catalogs feeds list/create/upload`
- `catalogs sets list`
- MCP tools for catalog management (dynamic product ads)

### Sprint 7 — Business Management
- `business list/get`
- `business users`, `business ad-accounts`
- Multi-account workflows
- Batch operations (multiple creates in one API call)

### Sprint 8 — Distribution & Polish
- Homebrew tap
- AUR package
- Nix package (nixpkgs or flake overlay)
- Docker image
- Comprehensive README with examples
- Shell completions (bash, zsh, fish)
- Man pages
