# meta-cli

CLI and MCP server for the Meta Marketing API.

Create campaigns, upload creatives, configure targeting, and manage ads — from your terminal or any AI agent that speaks MCP.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/enriquefft/meta-cli/main/install.sh | sh
```

Or with Go:

```sh
go install github.com/enriquefft/meta-cli/cmd/meta@latest
```

## Quick start

```sh
# Configure your Meta access token
meta config set access_token <YOUR_TOKEN>

# Verify authentication
meta auth status

# List your ad accounts
meta accounts list

# Create a campaign (paused by default)
meta campaigns create \
  --name "Summer Sale" \
  --objective OUTCOME_SALES \
  --daily-budget 50.00

# Find targeting interests
meta targeting search --query "e-commerce" --type interest

# Full pipeline: campaign → ad set → creative → ad
meta campaigns create --name "Launch" --objective OUTCOME_TRAFFIC --daily-budget 25.00
meta adsets create --campaign-id <ID> --name "US 25-45" --countries US --age-min 25 --age-max 45
meta creatives create --name "Hero Video" --page-id <PAGE_ID> --video-id <VID> --message "Shop now" --link "https://example.com"
meta ads create --adset-id <ADSET_ID> --creative-id <CREATIVE_ID> --name "Ad v1"
```

## MCP server

Start the MCP server for AI agent integration:

```sh
meta serve
```

Add to your MCP client config (Claude Desktop, Cursor, etc.):

```json
{
  "mcpServers": {
    "meta": {
      "command": "meta",
      "args": ["serve"]
    }
  }
}
```

### Available tools

| Tool | Description |
|------|-------------|
| `meta_auth_status` | Verify token validity and permissions |
| `meta_list_accounts` | List accessible ad accounts |
| `meta_list_pages` | List managed Facebook Pages |
| `meta_upload_video` | Upload video asset to an ad account |
| `meta_video_status` | Check video encoding status |
| `meta_search_targeting` | Search interests, behaviors, demographics |
| `meta_create_campaign` | Create campaign with objective and budget |
| `meta_create_adset` | Create ad set with targeting and schedule |
| `meta_create_creative` | Build ad creative with media and copy |
| `meta_create_ad` | Link creative to ad set |

The server includes built-in instructions that guide AI agents through the full ad creation workflow.

## Commands

```
meta config [get|set|unset]     Manage configuration
meta auth status                Verify access token
meta accounts list              List ad accounts
meta pages list                 List Facebook Pages
meta assets upload-video        Upload video for creatives
meta assets status <video_id>   Check video encoding status
meta campaigns create           Create advertising campaign
meta adsets create              Create ad set with targeting
meta creatives create           Create ad creative
meta ads create                 Create ad linking creative to ad set
meta targeting search           Search targeting interests/behaviors
meta serve                      Start MCP server (stdio)
```

### Global flags

```
-a, --account <id>     Override default ad account
-f, --format <format>  Output format: json, table, csv
    --fields <list>    Comma-separated fields to include
    --dry-run          Validate without creating resources
-v, --verbose          Enable debug logging
    --no-color         Disable colored output
```

## Configuration

Config file: `~/.config/meta-cli/config.yaml`

```yaml
access_token: "EAA..."
default_account: "act_123456789"
api_version: "v21.0"
output_format: "json"
```

Environment variables override the config file:

```sh
META_ACCESS_TOKEN=EAA...
META_AD_ACCOUNT=act_123456789
META_API_VERSION=v21.0
META_OUTPUT_FORMAT=json
```

Resolution order: flag > environment variable > config file > default.

## Building from source

Requires Go 1.26+ and [just](https://github.com/casey/just).

```sh
just build     # Build to bin/meta
just test      # Run tests
just lint       # Run golangci-lint
just install   # Install to $GOBIN
```

## Architecture

```
CLI (cobra)  │  MCP (mcp-go)        ← thin shells: parse input, call domain, format output
     ────────┼────────
        internal/meta/               ← domain operations (single source of truth)
             │
        internal/graph/              ← HTTP transport, retry, rate limiting, uploads
             │
        Meta Marketing API
```

- **Deep module design** — few modules with wide interfaces hiding substantial complexity
- **Domain owns the abstraction** — `internal/meta/` defines the `Client` interface; `internal/graph/` implements it
- **No business logic in shells** — CLI and MCP are thin translation layers
- **Budget conversion at the boundary** — users input dollars, domain speaks cents

## License

MIT
