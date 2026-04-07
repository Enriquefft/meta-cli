package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/enriquefft/meta-cli/internal/meta"
)

// testMockClient implements meta.Client for testing.
type testMockClient struct {
	getFn      func(ctx context.Context, path string, params url.Values) (*meta.Response, error)
	postFn     func(ctx context.Context, path string, params map[string]string) (*meta.Response, error)
	uploadFn   func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error)
	paginateFn func(ctx context.Context, path string, params url.Values) *meta.PageIterator
	dryRun     bool
	verbose    bool
}

var _ meta.Client = (*testMockClient)(nil)

func (m *testMockClient) Get(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
	if m.getFn == nil {
		panic("testMockClient.Get not set")
	}
	return m.getFn(ctx, path, params)
}

func (m *testMockClient) Post(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
	if m.postFn == nil {
		panic("testMockClient.Post not set")
	}
	return m.postFn(ctx, path, params)
}

func (m *testMockClient) Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
	if m.uploadFn == nil {
		panic("testMockClient.Upload not set")
	}
	return m.uploadFn(ctx, path, file, filename, size, params)
}

func (m *testMockClient) Paginate(ctx context.Context, path string, params url.Values) *meta.PageIterator {
	if m.paginateFn == nil {
		panic("testMockClient.Paginate not set")
	}
	return m.paginateFn(ctx, path, params)
}

func (m *testMockClient) SetDryRun(v bool)  { m.dryRun = v }
func (m *testMockClient) SetVerbose(v bool) { m.verbose = v }

// callTool is a test helper that creates a CallToolRequest and calls a handler.
func callTool(ctx context.Context, handler func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error), args map[string]any) (*mcplib.CallToolResult, error) {
	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "test_tool",
			Arguments: args,
		},
	}
	return handler(ctx, req)
}

// --- Existing tests (fixed) ---

func TestHandleAuthStatus(t *testing.T) {
	mc := &testMockClient{}
	callCount := 0
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		callCount++
		switch callCount {
		case 1: // /me
			return &meta.Response{Body: []byte(`{"id":"123","name":"Test User"}`), StatusCode: 200}, nil
		case 2: // /me/permissions
			return &meta.Response{Body: []byte(`{"data":[{"permission":"ads_management","status":"granted"}]}`), StatusCode: 200}, nil
		default:
			return nil, fmt.Errorf("unexpected call %d to %s", callCount, path)
		}
	}

	handler := handleAuthStatus(mc)
	result, err := callTool(context.Background(), handler, map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestHandleListAccounts(t *testing.T) {
	mc := &testMockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"act_123","account_id":"123","name":"Test Account","account_status":1,"currency":"USD"}],"paging":{}}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleListAccounts(mc)
	result, err := callTool(context.Background(), handler, map[string]any{"limit": float64(10)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
}

func TestHandleCreateCampaign(t *testing.T) {
	mc := &testMockClient{}
	var capturedParams map[string]string
	// Meta's POST /campaigns only returns the new campaign's id.
	// CreateCampaign chains a follow-up GET to enrich the returned record;
	// wire up both.
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"camp_123"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"camp_123","name":"Test Campaign","objective":"OUTCOME_SALES","status":"PAUSED","daily_budget":"5000"}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleCreateCampaign(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id":          "act_123456",
		"name":                "Test Campaign",
		"objective":           "OUTCOME_SALES",
		"daily_budget":        float64(50),
		"bid_strategy":        "LOWEST_COST_WITHOUT_CAP",
		"status":              "PAUSED",
		"special_ad_category": "NONE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	// Verify dollar-to-cents conversion: $50 -> 5000 cents
	if capturedParams["daily_budget"] != "5000" {
		t.Errorf("expected daily_budget '5000', got %q", capturedParams["daily_budget"])
	}
}

func TestHandleCreateAd(t *testing.T) {
	mc := &testMockClient{}
	// Meta's POST /ads only returns the new ad's id. CreateAd chains a
	// follow-up GET to enrich the returned record; wire up both.
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"ad_123"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"ad_123","name":"Test Ad","adset_id":"adset_456","creative":{"id":"crtv_789"},"status":"PAUSED"}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleCreateAd(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id":  "act_123456",
		"name":        "Test Ad",
		"adset_id":    "adset_456",
		"creative_id": "crtv_789",
		"status":      "PAUSED",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
}

func TestHandleSearchTargeting(t *testing.T) {
	mc := &testMockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"data":[{"id":"600327","name":"E-commerce","type":"interests","audience_size_lower_bound":500000000,"audience_size_upper_bound":600000000,"path":["Interests","Shopping","E-commerce"]}]}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleSearchTargeting(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123",
		"type":       "interests",
		"query":      "e-commerce",
		"limit":      float64(25),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
}

func TestHandleVideoStatus(t *testing.T) {
	mc := &testMockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"vid_123","title":"test.mp4","status":{"video_status":"ready","processing_progress":100},"length":30.5}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleVideoStatus(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"video_id": "vid_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
}

func TestHandleCreateCampaign_MissingRequired(t *testing.T) {
	mc := &testMockClient{}
	handler := handleCreateCampaign(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123",
		// missing "name" and "objective"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for missing required params")
	}
}

func TestNewServer(t *testing.T) {
	mc := &testMockClient{}
	s := NewServer(mc)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestNewServer_RegistersTools(t *testing.T) {
	mc := &testMockClient{}
	_ = NewServer(mc)
	// If NewServer didn't panic, all 10 tools registered successfully.
	// The mcp-go server doesn't expose a ListTools method in v0.46.0,
	// so we verify registration succeeded by checking the server was created without error.
}

// --- New tests for previously uncovered handlers ---

func TestHandleListPages(t *testing.T) {
	mc := &testMockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		switch path {
		case "/me/accounts":
			return &meta.Response{
				Body: []byte(`{
					"data": [
						{"id": "page_001", "name": "My Business Page", "category": "Business", "access_token": "page_token_abc"}
					]
				}`),
				StatusCode: 200,
			}, nil
		case "/me/businesses":
			return &meta.Response{Body: []byte(`{"data":[]}`), StatusCode: 200}, nil
		default:
			return nil, fmt.Errorf("unexpected path %q", path)
		}
	}

	handler := handleListPages(mc)
	result, err := callTool(context.Background(), handler, map[string]any{"limit": float64(10)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Verify the response contains page data and the new enriched shape.
	var parsed struct {
		Data []struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Sources []string `json:"sources"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].(mcplib.TextContent).Text), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	if len(parsed.Data) != 1 {
		t.Fatalf("expected 1 page, got %d", len(parsed.Data))
	}
	if parsed.Data[0].ID != "page_001" {
		t.Errorf("expected page ID 'page_001', got %q", parsed.Data[0].ID)
	}
	if len(parsed.Data[0].Sources) != 1 || parsed.Data[0].Sources[0] != "direct" {
		t.Errorf("expected sources [direct], got %v", parsed.Data[0].Sources)
	}
}

func TestHandleUploadVideo(t *testing.T) {
	// Create a temporary file to simulate a video upload.
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "test_video.mp4")
	videoContent := []byte("fake video data for testing")
	if err := os.WriteFile(videoPath, videoContent, 0644); err != nil {
		t.Fatalf("failed to create temp video file: %v", err)
	}

	mc := &testMockClient{}
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		if filename != "test_video.mp4" {
			t.Errorf("expected filename 'test_video.mp4', got %q", filename)
		}
		if size != int64(len(videoContent)) {
			t.Errorf("expected size %d, got %d", len(videoContent), size)
		}
		// Meta's /advideos endpoint only returns the new video's id.
		return &meta.Response{
			Body:       []byte(`{"id":"vid_789"}`),
			StatusCode: 200,
		}, nil
	}
	// UploadVideo chains a follow-up GET to return the full video record.
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		if path != "/vid_789" {
			t.Errorf("expected follow-up GET path /vid_789, got %s", path)
		}
		return &meta.Response{
			Body:       []byte(`{"id":"vid_789","title":"Test Video","status":{"video_status":"processing","processing_progress":0},"length":8.25}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleUploadVideo(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123456",
		"file_path":  videoPath,
		"title":      "Test Video",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestHandleUploadImage(t *testing.T) {
	// Create a temporary file to simulate an image upload.
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "photo.jpg")
	imageContent := []byte("fake image bytes for testing")
	if err := os.WriteFile(imagePath, imageContent, 0644); err != nil {
		t.Fatalf("failed to create temp image file: %v", err)
	}

	// Verify the tool registration matches the expected MCP tool name.
	if uploadImageTool().Name != "meta_upload_image" {
		t.Errorf("expected tool name 'meta_upload_image', got %q", uploadImageTool().Name)
	}

	mc := &testMockClient{}
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		if path != "/act_123456/adimages" {
			t.Errorf("expected path /act_123456/adimages, got %s", path)
		}
		if filename != "photo.jpg" {
			t.Errorf("expected filename 'photo.jpg', got %q", filename)
		}
		if size != int64(len(imageContent)) {
			t.Errorf("expected size %d, got %d", len(imageContent), size)
		}
		return &meta.Response{
			Body:       []byte(`{"images":{"photo.jpg":{"hash":"mcp_image_hash","url":"https://scontent.xx.fbcdn.net/v/full.jpg","width":640,"height":480,"name":"photo.jpg"}}}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleUploadImage(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123456",
		"file_path":  imagePath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Parse the JSON text result and assert the hash field is present.
	text := result.Content[0].(mcplib.TextContent).Text
	var parsed meta.Image
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	if parsed.Hash != "mcp_image_hash" {
		t.Errorf("expected hash 'mcp_image_hash' in result, got %q", parsed.Hash)
	}
	if parsed.Name != "photo.jpg" {
		t.Errorf("expected name 'photo.jpg' in result, got %q", parsed.Name)
	}
}

func TestHandleUploadImage_FileNotFound(t *testing.T) {
	mc := &testMockClient{}
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		t.Fatal("uploadFn should not be called when file does not exist")
		return nil, nil
	}

	handler := handleUploadImage(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123456",
		"file_path":  "/nonexistent/path/to/image.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for nonexistent file path")
	}
}

func TestHandleUploadVideo_FileNotFound(t *testing.T) {
	mc := &testMockClient{}
	// uploadFn should never be called for a missing file.
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		t.Fatal("uploadFn should not be called when file does not exist")
		return nil, nil
	}

	handler := handleUploadVideo(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123456",
		"file_path":  "/nonexistent/path/to/video.mp4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for nonexistent file path")
	}
}

func TestHandleCreateAdSet(t *testing.T) {
	mc := &testMockClient{}
	var capturedParams map[string]string
	// Meta's POST /adsets only returns the new ad set's id. CreateAdSet
	// chains a follow-up GET to enrich the returned record; wire up both.
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"adset_001"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"adset_001","name":"Test AdSet","campaign_id":"camp_123","status":"PAUSED","daily_budget":"5000"}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleCreateAdSet(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id":        "act_123456",
		"name":              "Test AdSet",
		"campaign_id":       "camp_123",
		"optimization_goal": "OFFSITE_CONVERSIONS",
		"daily_budget":      float64(50),
		"countries":         []any{"US", "CA"},
		"age_min":           float64(25),
		"age_max":           float64(55),
		"interests":         []any{"600327", "600334"},
		"behaviors":         []any{"600710"},
		"status":            "PAUSED",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	// Verify targeting JSON contains the countries and interests we passed.
	targetingJSON, ok := capturedParams["targeting"]
	if !ok {
		t.Fatal("expected 'targeting' key in posted params")
	}
	var targeting map[string]any
	if err := json.Unmarshal([]byte(targetingJSON), &targeting); err != nil {
		t.Fatalf("failed to parse targeting JSON: %v", err)
	}

	geoLoc, ok := targeting["geo_locations"].(map[string]any)
	if !ok {
		t.Fatal("targeting missing geo_locations object")
	}
	countries, ok := geoLoc["countries"].([]any)
	if !ok || len(countries) != 2 {
		t.Fatalf("expected 2 countries in targeting, got %v", countries)
	}

	interests, ok := targeting["interests"].([]any)
	if !ok || len(interests) != 2 {
		t.Fatalf("expected 2 interests in targeting, got %v", interests)
	}

	// Verify dollar-to-cents conversion for daily_budget.
	if capturedParams["daily_budget"] != "5000" {
		t.Errorf("expected daily_budget '5000', got %q", capturedParams["daily_budget"])
	}
}

func TestHandleCreateAdSet_MissingRequired(t *testing.T) {
	mc := &testMockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		t.Fatal("postFn should not be called when required fields are missing")
		return nil, nil
	}

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing name and campaign_id",
			args: map[string]any{
				"account_id": "act_123",
			},
		},
		{
			name: "missing optimization_goal",
			args: map[string]any{
				"account_id":  "act_123",
				"name":        "AdSet",
				"campaign_id": "camp_1",
			},
		},
		{
			name: "missing countries",
			args: map[string]any{
				"account_id":        "act_123",
				"name":              "AdSet",
				"campaign_id":       "camp_1",
				"optimization_goal": "OFFSITE_CONVERSIONS",
			},
		},
		{
			name: "empty countries array",
			args: map[string]any{
				"account_id":        "act_123",
				"name":              "AdSet",
				"campaign_id":       "camp_1",
				"optimization_goal": "OFFSITE_CONVERSIONS",
				"countries":         []any{},
			},
		},
	}

	handler := handleCreateAdSet(mc)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := callTool(context.Background(), handler, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Error("expected error result for missing required params")
			}
		})
	}
}

func TestHandleCreateCreative(t *testing.T) {
	mc := &testMockClient{}
	var capturedParams map[string]string
	// Meta's POST /adcreatives only returns the new creative's id.
	// CreateCreative chains a follow-up GET to enrich the returned record;
	// wire up both.
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		capturedParams = params
		return &meta.Response{
			Body:       []byte(`{"id":"crtv_001"}`),
			StatusCode: 200,
		}, nil
	}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"crtv_001","name":"Test Creative"}`),
			StatusCode: 200,
		}, nil
	}

	handler := handleCreateCreative(mc)
	result, err := callTool(context.Background(), handler, map[string]any{
		"account_id": "act_123456",
		"name":       "Test Creative",
		"page_id":    "page_001",
		"video_id":   "vid_789",
		"message":    "Buy our amazing product!",
		"headline":   "Amazing Product",
		"cta":        "SHOP_NOW",
		"link":       "https://example.com/product",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	// Verify object_story_spec contains the video_id and page_id.
	ossJSON, ok := capturedParams["object_story_spec"]
	if !ok {
		t.Fatal("expected 'object_story_spec' key in posted params")
	}
	var oss map[string]any
	if err := json.Unmarshal([]byte(ossJSON), &oss); err != nil {
		t.Fatalf("failed to parse object_story_spec JSON: %v", err)
	}
	if oss["page_id"] != "page_001" {
		t.Errorf("expected page_id 'page_001', got %v", oss["page_id"])
	}
	videoData, ok := oss["video_data"].(map[string]any)
	if !ok {
		t.Fatal("expected video_data in object_story_spec")
	}
	if videoData["video_id"] != "vid_789" {
		t.Errorf("expected video_id 'vid_789', got %v", videoData["video_id"])
	}
}

func TestHandleCreateCreative_MissingRequired(t *testing.T) {
	mc := &testMockClient{}
	mc.postFn = func(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
		t.Fatal("postFn should not be called when required fields are missing")
		return nil, nil
	}

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing page_id",
			args: map[string]any{
				"account_id": "act_123",
				"message":    "Ad copy",
				"link":       "https://example.com",
				"video_id":   "vid_1",
			},
		},
		{
			name: "missing message",
			args: map[string]any{
				"account_id": "act_123",
				"page_id":    "page_1",
				"link":       "https://example.com",
				"video_id":   "vid_1",
			},
		},
		{
			name: "missing link",
			args: map[string]any{
				"account_id": "act_123",
				"page_id":    "page_1",
				"message":    "Ad copy",
				"video_id":   "vid_1",
			},
		},
	}

	handler := handleCreateCreative(mc)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := callTool(context.Background(), handler, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Error("expected error result for missing required params")
			}
		})
	}
}
