//go:build integration

package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	meta "github.com/enriquefft/meta-cli/internal/meta"
)

const apiVersion = "v21.0"

func testClient(t *testing.T) (*Client, string) {
	t.Helper()

	token := os.Getenv("META_ACCESS_TOKEN")
	if token == "" {
		t.Fatal("META_ACCESS_TOKEN must be set")
	}
	accountID := os.Getenv("META_AD_ACCOUNT")
	if accountID == "" {
		t.Fatal("META_AD_ACCOUNT must be set (e.g. act_123456789)")
	}

	client := NewClient(ClientConfig{
		AccessToken: token,
		APIVersion:  apiVersion,
	})
	return client, accountID
}

func TestIntegration_AuthStatus(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	result, err := meta.AuthStatus(ctx, client)
	if err != nil {
		t.Fatalf("AuthStatus failed: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected Valid=true")
	}
	if result.User.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if len(result.Permissions) == 0 {
		t.Error("expected at least one permission")
	}
	t.Logf("authenticated as: %s (ID: %s), permissions: %v", result.User.Name, result.User.ID, result.Permissions)
}

func TestIntegration_ListAccounts(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	result, err := meta.ListAccounts(ctx, client, meta.ListAccountsParams{})
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected at least one ad account")
	}
	for _, acc := range result.Data {
		t.Logf("account: id=%s name=%s currency=%s status=%d", acc.ID, acc.Name, acc.Currency, acc.AccountStatus)
	}
}

func TestIntegration_ListPages(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	result, err := meta.ListPages(ctx, client, meta.ListPagesParams{})
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected at least one page")
	}
	for _, pg := range result.Data {
		t.Logf("page: id=%s name=%s", pg.ID, pg.Name)
	}
}

func TestIntegration_SearchTargeting(t *testing.T) {
	client, _ := testClient(t)
	ctx := context.Background()

	result, err := meta.SearchTargeting(ctx, client, meta.SearchTargetingParams{
		Type:  "interests",
		Query: "technology",
		Limit: 5,
	})
	if err != nil {
		// The /search endpoint is restricted in sandbox environments.
		t.Skipf("SearchTargeting not available in sandbox: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected at least one targeting suggestion")
	}
	for _, s := range result.Data {
		t.Logf("interest: id=%s name=%s audience=%d-%d", s.ID, s.Name, s.AudienceSizeLowerBound, s.AudienceSizeUpperBound)
	}
}

func TestIntegration_AdCreationPipeline(t *testing.T) {
	client, accountID := testClient(t)
	ctx := context.Background()
	ts := time.Now().Unix()
	namePrefix := fmt.Sprintf("int_test_%d", ts)

	// ── Step 0: Discover prerequisites ──────────────────────────────────

	pagesResult, err := meta.ListPages(ctx, client, meta.ListPagesParams{})
	if err != nil {
		t.Fatalf("listing pages: %v", err)
	}
	if len(pagesResult.Data) == 0 {
		t.Fatal("need at least one page to create a creative")
	}
	pageID := pagesResult.Data[0].ID
	t.Logf("using page: %s (%s)", pageID, pagesResult.Data[0].Name)

	// The /search endpoint is restricted in sandbox, so we use a well-known
	// interest ID directly. 6003129739455 = "Technology" on Meta.
	interestID := "6003129739455"
	t.Logf("using interest: %s (Technology)", interestID)

	// ── Step 1: Upload video ────────────────────────────────────────────

	videoPath := os.Getenv("META_TEST_VIDEO")
	if videoPath == "" {
		videoPath = filepath.Join("..", "..", "example.mp4")
	}

	videoFile, err := os.Open(videoPath)
	if err != nil {
		t.Fatalf("opening video file %s: %v", videoPath, err)
	}
	defer videoFile.Close()

	fi, err := videoFile.Stat()
	if err != nil {
		t.Fatalf("stat video file: %v", err)
	}

	uploadResult, err := meta.UploadVideo(ctx, client, meta.UploadVideoParams{
		AccountID: accountID,
		File:      videoFile,
		Filename:  fmt.Sprintf("%s_video.mp4", namePrefix),
		FileSize:  fi.Size(),
		Title:     fmt.Sprintf("%s video", namePrefix),
	})
	if err != nil {
		t.Fatalf("uploading video: %v", err)
	}
	videoID := uploadResult.ID
	if videoID == "" {
		t.Fatal("expected non-empty video ID")
	}
	t.Logf("uploaded video: id=%s", videoID)

	// ── Step 2: Poll video status until ready ────────────────────────────

	videoReady := false
	for i := 0; i < 30; i++ {
		status, err := meta.GetVideoStatus(ctx, client, meta.VideoStatusParams{VideoID: videoID})
		if err != nil {
			t.Logf("video status poll %d: error: %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		t.Logf("video status: %s (progress: %d%%)", status.Status.VideoStatus, status.Status.ProcessingProgress)
		if status.Status.VideoStatus == "ready" {
			videoReady = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !videoReady {
		t.Fatal("video did not become ready within 60s")
	}

	// ── Step 3: Create campaign ─────────────────────────────────────────

	campaign, err := meta.CreateCampaign(ctx, client, meta.CreateCampaignParams{
		AccountID:        accountID,
		Name:             fmt.Sprintf("%s campaign", namePrefix),
		Objective:        "OUTCOME_TRAFFIC",
		DailyBudgetCents: 100, // $1.00/day
		Status:           "PAUSED",
	})
	if err != nil {
		t.Fatalf("creating campaign: %v", err)
	}
	if campaign.ID == "" {
		t.Fatal("expected non-empty campaign ID")
	}
	t.Logf("created campaign: id=%s name=%s", campaign.ID, campaign.Name)

	// ── Step 4: Create ad set ───────────────────────────────────────────

	adset, err := meta.CreateAdSet(ctx, client, meta.CreateAdSetParams{
		AccountID:        accountID,
		Name:             fmt.Sprintf("%s adset", namePrefix),
		CampaignID:       campaign.ID,
		DailyBudgetCents: 100,
		OptimizationGoal: "LINK_CLICKS",
		BillingEvent:     "IMPRESSIONS",
		Countries:        []string{"US"},
		AgeMin:           25,
		AgeMax:           45,
		Genders:          []int{1, 2},
		Interests:        []string{interestID},
		Status:           "PAUSED",
	})
	if err != nil {
		t.Fatalf("creating ad set: %v", err)
	}
	if adset.ID == "" {
		t.Fatal("expected non-empty ad set ID")
	}
	t.Logf("created ad set: id=%s name=%s", adset.ID, adset.Name)

	// ── Step 5: Create creative ─────────────────────────────────────────

	creative, err := meta.CreateCreative(ctx, client, meta.CreateCreativeParams{
		AccountID: accountID,
		Name:      fmt.Sprintf("%s creative", namePrefix),
		PageID:    pageID,
		VideoID:   videoID,
		Message:   "Discover our latest product — check it out now!",
		Headline:  "Shop the Latest",
		CTA:       "SHOP_NOW",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("creating creative: %v", err)
	}
	if creative.ID == "" {
		t.Fatal("expected non-empty creative ID")
	}
	t.Logf("created creative: id=%s name=%s", creative.ID, creative.Name)

	// ── Step 6: Create ad ───────────────────────────────────────────────

	ad, err := meta.CreateAd(ctx, client, meta.CreateAdParams{
		AccountID:  accountID,
		Name:       fmt.Sprintf("%s ad", namePrefix),
		AdSetID:    adset.ID,
		CreativeID: creative.ID,
		Status:     "PAUSED",
	})
	if err != nil {
		t.Fatalf("creating ad: %v", err)
	}
	if ad.ID == "" {
		t.Fatal("expected non-empty ad ID")
	}
	t.Logf("created ad: id=%s name=%s", ad.ID, ad.Name)

	t.Log("=== pipeline complete ===")
	t.Logf("campaign=%s → adset=%s → creative=%s → ad=%s", campaign.ID, adset.ID, creative.ID, ad.ID)
}
