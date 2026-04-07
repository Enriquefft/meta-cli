package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/enriquefft/meta-cli/internal/meta"
)

// toolHandler is the function signature for MCP tool handlers from the mcp-go server package.
type toolHandler = server.ToolHandlerFunc

// --- Tool Definitions ---

func authStatusTool() mcplib.Tool {
	return mcplib.NewTool("meta_auth_status",
		mcplib.WithDescription("Verify the current access token is valid and show granted permissions."),
	)
}

func listAccountsTool() mcplib.Tool {
	return mcplib.NewTool("meta_list_accounts",
		mcplib.WithDescription("List ad accounts accessible by the authenticated user."),
		mcplib.WithNumber("limit",
			mcplib.Description("Maximum number of accounts to return"),
			mcplib.DefaultNumber(25),
		),
	)
}

func listPagesTool() mcplib.Tool {
	return mcplib.NewTool("meta_list_pages",
		mcplib.WithDescription("List Facebook Pages managed by the authenticated user."),
		mcplib.WithNumber("limit",
			mcplib.Description("Maximum number of pages to return"),
			mcplib.DefaultNumber(25),
		),
	)
}

func uploadVideoTool() mcplib.Tool {
	return mcplib.NewTool("meta_upload_video",
		mcplib.WithDescription("Upload a video file to an ad account for use in ad creatives."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("file_path",
			mcplib.Description("Local file path to the video file"),
			mcplib.Required(),
		),
		mcplib.WithString("title",
			mcplib.Description("Optional title for the video"),
		),
	)
}

func uploadImageTool() mcplib.Tool {
	return mcplib.NewTool("meta_upload_image",
		mcplib.WithDescription("Upload an image file to an ad account for use in ad creatives (including as a video creative thumbnail)."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("file_path",
			mcplib.Description("Local file path to the image file"),
			mcplib.Required(),
		),
	)
}

func videoStatusTool() mcplib.Tool {
	return mcplib.NewTool("meta_video_status",
		mcplib.WithDescription("Check the encoding status of an uploaded video. Wait until status is 'ready' before using in creatives."),
		mcplib.WithString("video_id",
			mcplib.Description("The video ID returned from meta_upload_video"),
			mcplib.Required(),
		),
	)
}

func createCampaignTool() mcplib.Tool {
	return mcplib.NewTool("meta_create_campaign",
		mcplib.WithDescription("Create a new advertising campaign under the specified ad account."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("name",
			mcplib.Description("Campaign name"),
			mcplib.Required(),
		),
		mcplib.WithString("objective",
			mcplib.Description("Campaign objective"),
			mcplib.Required(),
			mcplib.Enum("OUTCOME_SALES", "OUTCOME_TRAFFIC", "OUTCOME_ENGAGEMENT", "OUTCOME_AWARENESS", "OUTCOME_LEADS", "OUTCOME_APP_PROMOTION"),
		),
		mcplib.WithNumber("daily_budget",
			mcplib.Description("Daily budget in dollars (e.g. 50.00 = $50). Mutually exclusive with lifetime_budget; at least one is required."),
		),
		mcplib.WithNumber("lifetime_budget",
			mcplib.Description("Lifetime budget in dollars (e.g. 500.00 = $500). Mutually exclusive with daily_budget; at least one is required."),
		),
		mcplib.WithString("bid_strategy",
			mcplib.Description("Bid strategy"),
			mcplib.DefaultString("LOWEST_COST_WITHOUT_CAP"),
		),
		mcplib.WithString("status",
			mcplib.Description("Initial campaign status"),
			mcplib.DefaultString("PAUSED"),
		),
		mcplib.WithString("special_ad_category",
			mcplib.Description("Special ad category"),
			mcplib.DefaultString("NONE"),
		),
	)
}

func createAdSetTool() mcplib.Tool {
	return mcplib.NewTool("meta_create_adset",
		mcplib.WithDescription("Create a new ad set under a campaign, defining targeting, scheduling, and budget."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("name",
			mcplib.Description("Ad set name"),
			mcplib.Required(),
		),
		mcplib.WithString("campaign_id",
			mcplib.Description("Parent campaign ID"),
			mcplib.Required(),
		),
		mcplib.WithNumber("daily_budget",
			mcplib.Description("Daily budget in dollars"),
		),
		mcplib.WithNumber("lifetime_budget",
			mcplib.Description("Lifetime budget in dollars"),
		),
		mcplib.WithString("optimization_goal",
			mcplib.Description("Optimization goal for the ad set"),
			mcplib.Required(),
		),
		mcplib.WithString("billing_event",
			mcplib.Description("Billing event (default: IMPRESSIONS)"),
		),
		mcplib.WithNumber("bid_amount",
			mcplib.Description("Bid amount in dollars"),
		),
		mcplib.WithArray("countries",
			mcplib.Description("Target countries (ISO country codes)"),
			mcplib.Required(),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithNumber("age_min",
			mcplib.Description("Minimum age"),
			mcplib.DefaultNumber(18),
		),
		mcplib.WithNumber("age_max",
			mcplib.Description("Maximum age"),
			mcplib.DefaultNumber(65),
		),
		mcplib.WithArray("genders",
			mcplib.Description("Target genders (1=male, 2=female). Default: [1,2]"),
			mcplib.Items(map[string]any{"type": "number"}),
		),
		mcplib.WithArray("interests",
			mcplib.Description("Interest IDs for targeting (use meta_search_targeting with type 'interests' to find IDs)"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithArray("behaviors",
			mcplib.Description("Behavior IDs for targeting (use meta_search_targeting with type 'behaviors' to find IDs)"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithArray("custom_audiences",
			mcplib.Description("Custom audience IDs from the ad account"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithArray("excluded_countries",
			mcplib.Description("Countries to exclude from targeting"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithArray("publisher_platforms",
			mcplib.Description("Publisher platforms (facebook, instagram, audience_network, messenger)"),
			mcplib.Items(map[string]any{"type": "string"}),
		),
		mcplib.WithString("pixel_id",
			mcplib.Description("Facebook Pixel ID for conversion tracking"),
		),
		mcplib.WithString("custom_event_type",
			mcplib.Description("Custom event type for the pixel"),
		),
		mcplib.WithString("start_time",
			mcplib.Description("Ad set start time (ISO 8601)"),
		),
		mcplib.WithString("end_time",
			mcplib.Description("Ad set end time (ISO 8601)"),
		),
		mcplib.WithBoolean("advantage_audience",
			mcplib.Description("Enable Advantage+ audience expansion (default: false, uses explicit targeting)"),
		),
		mcplib.WithString("status",
			mcplib.Description("Initial ad set status"),
			mcplib.DefaultString("PAUSED"),
		),
	)
}

func createCreativeTool() mcplib.Tool {
	return mcplib.NewTool("meta_create_creative",
		mcplib.WithDescription("Create an ad creative with video/image, text, and call-to-action. For video creatives, the video's auto-generated thumbnail is resolved automatically — pass image_hash or image_url only if you want a custom thumbnail."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("name",
			mcplib.Description("Creative name"),
		),
		mcplib.WithString("page_id",
			mcplib.Description("Facebook Page ID"),
			mcplib.Required(),
		),
		mcplib.WithString("video_id",
			mcplib.Description("Video ID from meta_upload_video"),
		),
		mcplib.WithString("image_hash",
			mcplib.Description("Optional. Hash of a previously uploaded image to use as a custom thumbnail. Omit for video creatives to let meta-cli auto-resolve the video's default thumbnail."),
		),
		mcplib.WithString("image_url",
			mcplib.Description("Optional. URL of an image to use as a custom thumbnail. Omit for video creatives to let meta-cli auto-resolve the video's default thumbnail."),
		),
		mcplib.WithString("message",
			mcplib.Description("Ad copy / body text"),
			mcplib.Required(),
		),
		mcplib.WithString("headline",
			mcplib.Description("Ad headline"),
		),
		mcplib.WithString("description",
			mcplib.Description("Ad description"),
		),
		mcplib.WithString("cta",
			mcplib.Description("Call-to-action button type"),
			mcplib.DefaultString("SHOP_NOW"),
		),
		mcplib.WithString("link",
			mcplib.Description("Destination URL"),
			mcplib.Required(),
		),
		mcplib.WithString("instagram_account_id",
			mcplib.Description("Instagram account ID for Instagram placement"),
		),
	)
}

func createAdTool() mcplib.Tool {
	return mcplib.NewTool("meta_create_ad",
		mcplib.WithDescription("Create a new ad by linking a creative to an ad set."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("name",
			mcplib.Description("Ad name"),
			mcplib.Required(),
		),
		mcplib.WithString("adset_id",
			mcplib.Description("Ad set ID to place this ad in"),
			mcplib.Required(),
		),
		mcplib.WithString("creative_id",
			mcplib.Description("Creative ID to use for this ad"),
			mcplib.Required(),
		),
		mcplib.WithString("status",
			mcplib.Description("Initial ad status"),
			mcplib.DefaultString("PAUSED"),
		),
	)
}

func searchTargetingTool() mcplib.Tool {
	return mcplib.NewTool("meta_search_targeting",
		mcplib.WithDescription("Search for targeting options (interests, behaviors, demographics, etc.)."),
		mcplib.WithString("account_id",
			mcplib.Description("The ad account ID"),
			mcplib.Required(),
		),
		mcplib.WithString("type",
			mcplib.Description("Type of targeting to search"),
			mcplib.Required(),
			mcplib.Enum("interests", "behaviors", "demographics", "education_schools", "education_majors", "work_employers", "work_positions"),
		),
		mcplib.WithString("query",
			mcplib.Description("Search query string"),
			mcplib.Required(),
		),
		mcplib.WithNumber("limit",
			mcplib.Description("Maximum number of results"),
			mcplib.DefaultNumber(25),
		),
	)
}

// --- Tool Handlers ---

func handleAuthStatus(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result, err := meta.AuthStatus(ctx, client)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleListAccounts(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		limit := request.GetInt("limit", 25)

		result, err := meta.ListAccounts(ctx, client, meta.ListAccountsParams{
			Limit: limit,
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleListPages(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		limit := request.GetInt("limit", 25)

		result, err := meta.ListPages(ctx, client, meta.ListPagesParams{
			Limit: limit,
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleUploadVideo(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		filePath, err := request.RequireString("file_path")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		title := request.GetString("title", "")

		f, err := os.Open(filePath)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open file: %v", err)), nil
		}
		defer func() { _ = f.Close() }()

		fi, err := f.Stat()
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to stat file: %v", err)), nil
		}

		result, err := meta.UploadVideo(ctx, client, meta.UploadVideoParams{
			AccountID: accountID,
			File:      f,
			Filename:  fi.Name(),
			FileSize:  fi.Size(),
			Title:     title,
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleUploadImage(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		filePath, err := request.RequireString("file_path")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		f, err := os.Open(filePath)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open file: %v", err)), nil
		}
		defer func() { _ = f.Close() }()

		fi, err := f.Stat()
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to stat file: %v", err)), nil
		}

		result, err := meta.UploadImage(ctx, client, meta.UploadImageParams{
			AccountID: accountID,
			File:      f,
			Filename:  fi.Name(),
			FileSize:  fi.Size(),
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleVideoStatus(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		videoID, err := request.RequireString("video_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		result, err := meta.GetVideoStatus(ctx, client, meta.VideoStatusParams{
			VideoID: videoID,
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleCreateCampaign(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		name, err := request.RequireString("name")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		objective, err := request.RequireString("objective")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		params := meta.CreateCampaignParams{
			AccountID:         accountID,
			Name:              name,
			Objective:         objective,
			BidStrategy:       request.GetString("bid_strategy", "LOWEST_COST_WITHOUT_CAP"),
			Status:            request.GetString("status", "PAUSED"),
			SpecialAdCategory: request.GetString("special_ad_category", "NONE"),
		}

		if dailyBudget := request.GetFloat("daily_budget", 0); dailyBudget > 0 {
			cents, convErr := meta.DollarsToCents(dailyBudget)
			if convErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid daily_budget: %v", convErr)), nil
			}
			params.DailyBudgetCents = cents
		}

		if lifetimeBudget := request.GetFloat("lifetime_budget", 0); lifetimeBudget > 0 {
			cents, convErr := meta.DollarsToCents(lifetimeBudget)
			if convErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid lifetime_budget: %v", convErr)), nil
			}
			params.LifetimeBudgetCents = cents
		}

		result, err := meta.CreateCampaign(ctx, client, params)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleCreateAdSet(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		name, err := request.RequireString("name")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		campaignID, err := request.RequireString("campaign_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		optimizationGoal, err := request.RequireString("optimization_goal")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		countries := request.GetStringSlice("countries", nil)
		if len(countries) == 0 {
			return mcplib.NewToolResultError("required argument \"countries\" not found"), nil
		}

		params := meta.CreateAdSetParams{
			AccountID:          accountID,
			Name:               name,
			CampaignID:         campaignID,
			OptimizationGoal:   optimizationGoal,
			Countries:          countries,
			BillingEvent:       request.GetString("billing_event", ""),
			AgeMin:             request.GetInt("age_min", 18),
			AgeMax:             request.GetInt("age_max", 65),
			Genders:            request.GetIntSlice("genders", nil),
			Interests:          request.GetStringSlice("interests", nil),
			Behaviors:          request.GetStringSlice("behaviors", nil),
			CustomAudiences:    request.GetStringSlice("custom_audiences", nil),
			ExcludedCountries:  request.GetStringSlice("excluded_countries", nil),
			PublisherPlatforms: request.GetStringSlice("publisher_platforms", nil),
			PixelID:            request.GetString("pixel_id", ""),
			CustomEventType:    request.GetString("custom_event_type", ""),
			StartTime:          request.GetString("start_time", ""),
			EndTime:            request.GetString("end_time", ""),
			AdvantageAudience:  request.GetBool("advantage_audience", false),
			Status:             request.GetString("status", "PAUSED"),
		}

		if dailyBudget := request.GetFloat("daily_budget", 0); dailyBudget > 0 {
			cents, convErr := meta.DollarsToCents(dailyBudget)
			if convErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid daily_budget: %v", convErr)), nil
			}
			params.DailyBudgetCents = cents
		}

		if lifetimeBudget := request.GetFloat("lifetime_budget", 0); lifetimeBudget > 0 {
			cents, convErr := meta.DollarsToCents(lifetimeBudget)
			if convErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid lifetime_budget: %v", convErr)), nil
			}
			params.LifetimeBudgetCents = cents
		}

		if bidAmount := request.GetFloat("bid_amount", 0); bidAmount > 0 {
			cents, convErr := meta.DollarsToCents(bidAmount)
			if convErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid bid_amount: %v", convErr)), nil
			}
			params.BidAmountCents = cents
		}

		result, err := meta.CreateAdSet(ctx, client, params)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleCreateCreative(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		pageID, err := request.RequireString("page_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		message, err := request.RequireString("message")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		link, err := request.RequireString("link")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		params := meta.CreateCreativeParams{
			AccountID:          accountID,
			Name:               request.GetString("name", ""),
			PageID:             pageID,
			VideoID:            request.GetString("video_id", ""),
			ImageHash:          request.GetString("image_hash", ""),
			ImageURL:           request.GetString("image_url", ""),
			Message:            message,
			Headline:           request.GetString("headline", ""),
			Description:        request.GetString("description", ""),
			CTA:                request.GetString("cta", "SHOP_NOW"),
			Link:               link,
			InstagramAccountID: request.GetString("instagram_account_id", ""),
		}

		result, err := meta.CreateCreative(ctx, client, params)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleCreateAd(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		name, err := request.RequireString("name")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		adsetID, err := request.RequireString("adset_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		creativeID, err := request.RequireString("creative_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		params := meta.CreateAdParams{
			AccountID:  accountID,
			Name:       name,
			AdSetID:    adsetID,
			CreativeID: creativeID,
			Status:     request.GetString("status", "PAUSED"),
		}

		result, err := meta.CreateAd(ctx, client, params)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

func handleSearchTargeting(client meta.Client) toolHandler {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		accountID, err := request.RequireString("account_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		targetingType, err := request.RequireString("type")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		result, err := meta.SearchTargeting(ctx, client, meta.SearchTargetingParams{
			AccountID: accountID,
			Type:      targetingType,
			Query:     query,
			Limit:     request.GetInt("limit", 25),
		})
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return jsonResult(result)
	}
}

// jsonResult marshals v to JSON and returns it as a text tool result.
func jsonResult(v any) (*mcplib.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}
