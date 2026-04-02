package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"github.com/enriquefft/meta-cli/internal/meta"
)

const (
	serverName    = "meta-cli"
	serverVersion = "1.0.0"
)

const serverInstructions = `You are connected to the Meta Marketing API via meta-cli.

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

Always create campaigns and ads as PAUSED first. Budgets are in dollars (50.00 = $50).`

// NewServer creates a new MCP server with all meta-cli tools registered.
func NewServer(client meta.Client) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithInstructions(serverInstructions),
	)

	registerTools(s, client)
	return s
}

// RunServer creates and starts the MCP server on stdio transport.
func RunServer(ctx context.Context, client meta.Client) error {
	s := NewServer(client)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeStdio(s)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("server shutdown: %w", ctx.Err())
	case err := <-errCh:
		return err
	}
}

// registerTools registers all MCP tools on the server.
func registerTools(s *server.MCPServer, client meta.Client) {
	s.AddTool(authStatusTool(), handleAuthStatus(client))
	s.AddTool(listAccountsTool(), handleListAccounts(client))
	s.AddTool(listPagesTool(), handleListPages(client))
	s.AddTool(uploadVideoTool(), handleUploadVideo(client))
	s.AddTool(videoStatusTool(), handleVideoStatus(client))
	s.AddTool(createCampaignTool(), handleCreateCampaign(client))
	s.AddTool(createAdSetTool(), handleCreateAdSet(client))
	s.AddTool(createCreativeTool(), handleCreateCreative(client))
	s.AddTool(createAdTool(), handleCreateAd(client))
	s.AddTool(searchTargetingTool(), handleSearchTargeting(client))
}
