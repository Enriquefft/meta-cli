package meta

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateCreative_Video(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			body, _ := json.Marshal(Creative{
				ID:   "cr_123",
				Name: "Video Creative",
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID: "987654",
		Name:      "Video Creative",
		PageID:    "page_123",
		VideoID:   "vid_456",
		Message:   "Check this out!",
		Headline:  "Amazing Product",
		CTA:       "LEARN_MORE",
		Link:      "https://example.com",
	}

	creative, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/adcreatives" {
		t.Errorf("expected path /act_987654/adcreatives, got %s", capturedPath)
	}
	if capturedParams["name"] != "Video Creative" {
		t.Errorf("expected name 'Video Creative', got %s", capturedParams["name"])
	}

	ossJSON := capturedParams["object_story_spec"]
	if ossJSON == "" {
		t.Fatal("expected object_story_spec to be set")
	}

	var oss map[string]interface{}
	if err := json.Unmarshal([]byte(ossJSON), &oss); err != nil {
		t.Fatalf("object_story_spec is not valid JSON: %v", err)
	}

	if oss["page_id"] != "page_123" {
		t.Errorf("expected page_id 'page_123', got %v", oss["page_id"])
	}

	videoData, ok := oss["video_data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected video_data in object_story_spec")
	}
	if videoData["video_id"] != "vid_456" {
		t.Errorf("expected video_id 'vid_456', got %v", videoData["video_id"])
	}
	if videoData["message"] != "Check this out!" {
		t.Errorf("expected message 'Check this out!', got %v", videoData["message"])
	}
	if videoData["title"] != "Amazing Product" {
		t.Errorf("expected title 'Amazing Product', got %v", videoData["title"])
	}

	cta, ok := videoData["call_to_action"].(map[string]interface{})
	if !ok {
		t.Fatal("expected call_to_action in video_data")
	}
	if cta["type"] != "LEARN_MORE" {
		t.Errorf("expected CTA type 'LEARN_MORE', got %v", cta["type"])
	}
	ctaValue, ok := cta["value"].(map[string]interface{})
	if !ok {
		t.Fatal("expected value in call_to_action")
	}
	if ctaValue["link"] != "https://example.com" {
		t.Errorf("expected link 'https://example.com', got %v", ctaValue["link"])
	}

	if creative.ID != "cr_123" {
		t.Errorf("expected creative ID 'cr_123', got %s", creative.ID)
	}
	if creative.Name != "Video Creative" {
		t.Errorf("expected creative Name 'Video Creative', got %s", creative.Name)
	}
}

func TestCreateCreative_ImageHash(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			body, _ := json.Marshal(Creative{ID: "cr_456", Name: "Image Creative"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID:   "123",
		Name:        "Image Creative",
		PageID:      "page_123",
		ImageHash:   "abc123hash",
		Message:     "Great product!",
		Headline:    "Buy Now",
		Description: "The best product ever",
		CTA:         "SHOP_NOW",
		Link:        "https://example.com/shop",
	}

	_, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oss map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["object_story_spec"]), &oss); err != nil {
		t.Fatalf("object_story_spec is not valid JSON: %v", err)
	}

	linkData, ok := oss["link_data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected link_data in object_story_spec for image creative")
	}
	if linkData["image_hash"] != "abc123hash" {
		t.Errorf("expected image_hash 'abc123hash', got %v", linkData["image_hash"])
	}
	if linkData["message"] != "Great product!" {
		t.Errorf("expected message 'Great product!', got %v", linkData["message"])
	}
	if linkData["name"] != "Buy Now" {
		t.Errorf("expected name (headline) 'Buy Now', got %v", linkData["name"])
	}
	if linkData["description"] != "The best product ever" {
		t.Errorf("expected description 'The best product ever', got %v", linkData["description"])
	}

	cta := linkData["call_to_action"].(map[string]interface{})
	if cta["type"] != "SHOP_NOW" {
		t.Errorf("expected CTA type 'SHOP_NOW', got %v", cta["type"])
	}

	// video_data should not be present
	if _, ok := oss["video_data"]; ok {
		t.Error("video_data should not be present for image creative")
	}
}

func TestCreateCreative_ImageURL(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			body, _ := json.Marshal(Creative{ID: "cr_789"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "URL Creative",
		PageID:    "page_123",
		ImageURL:  "https://example.com/image.jpg",
		Message:   "Check it out",
		Link:      "https://example.com",
	}

	_, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oss map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["object_story_spec"]), &oss); err != nil {
		t.Fatalf("object_story_spec is not valid JSON: %v", err)
	}

	linkData, ok := oss["link_data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected link_data in object_story_spec for image URL creative")
	}
	if linkData["picture"] != "https://example.com/image.jpg" {
		t.Errorf("expected picture URL, got %v", linkData["picture"])
	}
	if _, ok := linkData["image_hash"]; ok {
		t.Error("image_hash should not be set when using image URL")
	}
}

func TestCreateCreative_DefaultCTA(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			body, _ := json.Marshal(Creative{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "Test",
		PageID:    "page_123",
		ImageHash: "hash",
		Message:   "Msg",
		Link:      "https://example.com",
	}

	_, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oss map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["object_story_spec"]), &oss); err != nil {
		t.Fatalf("object_story_spec is not valid JSON: %v", err)
	}

	linkData := oss["link_data"].(map[string]interface{})
	cta := linkData["call_to_action"].(map[string]interface{})
	if cta["type"] != "SHOP_NOW" {
		t.Errorf("expected default CTA 'SHOP_NOW', got %v", cta["type"])
	}
}

func TestCreateCreative_InstagramAccount(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			body, _ := json.Marshal(Creative{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID:          "123",
		Name:               "Test",
		PageID:             "page_123",
		ImageHash:          "hash",
		Message:            "Msg",
		Link:               "https://example.com",
		InstagramAccountID: "ig_456",
	}

	_, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oss map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["object_story_spec"]), &oss); err != nil {
		t.Fatalf("object_story_spec is not valid JSON: %v", err)
	}

	if oss["instagram_actor_id"] != "ig_456" {
		t.Errorf("expected instagram_actor_id 'ig_456', got %v", oss["instagram_actor_id"])
	}
}

func TestCreateCreative_AccountIDNormalization(t *testing.T) {
	var capturedPath string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			capturedPath = path
			body, _ := json.Marshal(Creative{ID: "1"})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	params := CreateCreativeParams{
		AccountID: "act_123",
		Name:      "Test",
		PageID:    "page_123",
		ImageHash: "hash",
		Message:   "Msg",
		Link:      "https://example.com",
	}

	_, err := CreateCreative(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_123/adcreatives" {
		t.Errorf("expected path /act_123/adcreatives, got %s", capturedPath)
	}
}

func TestCreateCreative_MissingAccountID(t *testing.T) {
	mock := &MockClient{}
	params := CreateCreativeParams{
		Name:    "Test",
		PageID:  "page_123",
		VideoID: "vid",
		Message: "Msg",
		Link:    "https://example.com",
	}
	_, err := CreateCreative(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing AccountID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCreative_MissingPageID(t *testing.T) {
	mock := &MockClient{}
	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "Test",
		VideoID:   "vid",
		Message:   "Msg",
		Link:      "https://example.com",
	}
	_, err := CreateCreative(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing PageID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCreative_MissingMessage(t *testing.T) {
	mock := &MockClient{}
	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "Test",
		PageID:    "page_123",
		VideoID:   "vid",
		Link:      "https://example.com",
	}
	_, err := CreateCreative(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Message")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCreative_MissingLink(t *testing.T) {
	mock := &MockClient{}
	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "Test",
		PageID:    "page_123",
		VideoID:   "vid",
		Message:   "Msg",
	}
	_, err := CreateCreative(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Link")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateCreative_NoMediaSource(t *testing.T) {
	mock := &MockClient{}
	params := CreateCreativeParams{
		AccountID: "123",
		Name:      "Test",
		PageID:    "page_123",
		Message:   "Msg",
		Link:      "https://example.com",
	}
	_, err := CreateCreative(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing media source")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}
