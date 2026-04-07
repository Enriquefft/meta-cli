package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// creativeGetFn returns a GetFn suitable for the MockClient that answers the
// follow-up GET issued by CreateCreative after the POST returns only
// {"id": ...}. The returned body mirrors the shape the real Graph API sends
// for the creativeFields field list.
func creativeGetFn(t *testing.T, creative Creative) func(ctx context.Context, path string, params url.Values) (*Response, error) {
	t.Helper()
	return func(_ context.Context, path string, params url.Values) (*Response, error) {
		expectedPath := "/" + creative.ID
		if path != expectedPath {
			t.Errorf("expected GET path %s, got %s", expectedPath, path)
		}
		if params.Get("fields") != creativeFields {
			t.Errorf("expected fields %q, got %q", creativeFields, params.Get("fields"))
		}
		body, _ := json.Marshal(creative)
		return &Response{Body: body, StatusCode: 200}, nil
	}
}

func TestCreateCreative_Video(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			return &Response{Body: []byte(`{"id":"cr_123"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_123", Name: "Video Creative"}),
	}

	// ImageHash is provided explicitly so this test exercises the pure
	// POST-body-shape path. The auto-thumbnail resolution path is covered
	// by TestCreateCreative_AutoResolvesVideoThumbnail and the helper's
	// own tests in assets_test.go.
	params := CreateCreativeParams{
		AccountID: "987654",
		Name:      "Video Creative",
		PageID:    "page_123",
		VideoID:   "vid_456",
		ImageHash: "explicit_thumb_hash",
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
			return &Response{Body: []byte(`{"id":"cr_456"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_456", Name: "Image Creative"}),
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
			return &Response{Body: []byte(`{"id":"cr_789"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_789"}),
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
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "1"}),
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
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "1"}),
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
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "1"}),
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

// TestCreateCreative_FetchesFullRecordAfterCreate is the regression test for
// the empty-fields bug: Meta's POST /adcreatives returns only {"id": ...},
// so CreateCreative must issue a follow-up GET to enrich the returned
// Creative with its name.
func TestCreateCreative_FetchesFullRecordAfterCreate(t *testing.T) {
	var postCalled, getCalled bool

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			postCalled = true
			if path != "/act_55/adcreatives" {
				t.Errorf("expected POST path /act_55/adcreatives, got %s", path)
			}
			return &Response{Body: []byte(`{"id":"cr_full"}`), StatusCode: 200}, nil
		},
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			getCalled = true
			if path != "/cr_full" {
				t.Errorf("expected GET path /cr_full, got %s", path)
			}
			if params.Get("fields") != creativeFields {
				t.Errorf("expected fields %q, got %q", creativeFields, params.Get("fields"))
			}
			return &Response{Body: []byte(`{"id":"cr_full","name":"Enriched Creative"}`), StatusCode: 200}, nil
		},
	}

	result, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "55",
		Name:      "Enriched Creative",
		PageID:    "page_55",
		ImageHash: "hash_55",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Fatal("expected CreateCreative to POST")
	}
	if !getCalled {
		t.Fatal("expected CreateCreative to issue a follow-up GET for full metadata")
	}
	if result.ID != "cr_full" {
		t.Errorf("expected ID cr_full, got %q", result.ID)
	}
	if result.Name != "Enriched Creative" {
		t.Errorf("expected Name Enriched Creative, got %q", result.Name)
	}
}

// TestCreateCreative_MissingIDInPostResponse asserts a defensive error is
// returned when the POST response lacks an id.
func TestCreateCreative_MissingIDInPostResponse(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{}`), StatusCode: 200}, nil
		},
	}

	_, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "1",
		Name:      "Test",
		PageID:    "page_1",
		ImageHash: "hash",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err == nil {
		t.Fatal("expected error when POST response omits id")
	}
	if !strings.Contains(err.Error(), "creative id") {
		t.Errorf("expected error to mention missing creative id, got: %v", err)
	}
}

// TestCreateCreative_PostErrorPropagated asserts POST-level transport errors
// short-circuit the Create flow and are surfaced to the caller.
func TestCreateCreative_PostErrorPropagated(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return nil, fmt.Errorf("i/o timeout")
		},
	}

	_, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "1",
		Name:      "Test",
		PageID:    "page_1",
		ImageHash: "hash",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err == nil {
		t.Fatal("expected POST error to be propagated")
	}
	if !strings.Contains(err.Error(), "i/o timeout") {
		t.Errorf("expected original error in chain, got: %v", err)
	}
}

// TestCreateCreative_AutoResolvesVideoThumbnail is the canonical regression
// for the original bug (Meta error subcode 1443226: "Your ad needs a video
// thumbnail"). When the caller passes a video_id but no image_hash or
// image_url, CreateCreative must transparently fetch the video's picture,
// upload it to /adimages, and include the resulting hash in video_data.
func TestCreateCreative_AutoResolvesVideoThumbnail(t *testing.T) {
	const (
		accountID     = "987654"
		videoID       = "vid_auto_thumb"
		pictureURL    = "https://scontent.xx.fbcdn.net/v/auto-thumb.jpg"
		expectHash    = "auto_thumb_hash"
		jpegPayload   = "auto-thumb-bytes"
		creativeID    = "cr_auto"
		thumbFilename = "thumb_vid_auto_thumb.jpg"
	)

	var events []string
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			events = append(events, "get:"+path)
			switch path {
			case "/" + videoID:
				if params.Get("fields") != videoFields {
					t.Errorf("video GET expected fields %q, got %q", videoFields, params.Get("fields"))
				}
				body, _ := json.Marshal(map[string]any{
					"id":      videoID,
					"title":   "Auto Thumb Clip",
					"length":  5.0,
					"picture": pictureURL,
					"status": map[string]any{
						"video_status":        "ready",
						"processing_progress": 100,
					},
				})
				return &Response{Body: body, StatusCode: 200}, nil
			case "/" + creativeID:
				if params.Get("fields") != creativeFields {
					t.Errorf("creative GET expected fields %q, got %q", creativeFields, params.Get("fields"))
				}
				body, _ := json.Marshal(Creative{ID: creativeID, Name: "Auto Video Creative"})
				return &Response{Body: body, StatusCode: 200}, nil
			default:
				t.Fatalf("unexpected GET path %s", path)
				return nil, nil
			}
		},
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			events = append(events, "upload:"+path)
			wantPath := "/act_" + accountID + "/adimages"
			if path != wantPath {
				t.Errorf("expected Upload path %s, got %s", wantPath, path)
			}
			if filename != thumbFilename {
				t.Errorf("expected filename %s, got %s", thumbFilename, filename)
			}
			return &Response{
				Body:       standardImageResponse(thumbFilename, expectHash),
				StatusCode: 200,
			}, nil
		},
	}

	var capturedPostBody map[string]string
	mock.PostFn = func(ctx context.Context, path string, params map[string]string) (*Response, error) {
		events = append(events, "post:"+path)
		wantPath := "/act_" + accountID + "/adcreatives"
		if path != wantPath {
			t.Errorf("expected POST path %s, got %s", wantPath, path)
		}
		capturedPostBody = params
		return &Response{Body: []byte(`{"id":"` + creativeID + `"}`), StatusCode: 200}, nil
	}

	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			events = append(events, "http:"+req.URL.String())
			if req.URL.String() != pictureURL {
				t.Errorf("expected HTTP GET %s, got %s", pictureURL, req.URL.String())
			}
			return newFakeHTTPResponse(200, []byte(jpegPayload)), nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	creative, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: accountID,
		Name:      "Auto Video Creative",
		PageID:    "page_99",
		VideoID:   videoID,
		Message:   "Try it now",
		Headline:  "Amazing",
		CTA:       "LEARN_MORE",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creative.ID != creativeID {
		t.Errorf("expected creative ID %s, got %s", creativeID, creative.ID)
	}

	// Assert the exact orchestration order: video GET, CDN download, image
	// upload, creative POST, chained creative GET.
	wantOrder := []string{
		"get:/" + videoID,
		"http:" + pictureURL,
		"upload:/act_" + accountID + "/adimages",
		"post:/act_" + accountID + "/adcreatives",
		"get:/" + creativeID,
	}
	if len(events) != len(wantOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(wantOrder), len(events), events)
	}
	for i, want := range wantOrder {
		if events[i] != want {
			t.Errorf("event %d: expected %q, got %q", i, want, events[i])
		}
	}

	// The creative POST body must include the auto-resolved image_hash in
	// video_data — this is the canonical regression for the original bug.
	ossJSON := capturedPostBody["object_story_spec"]
	if ossJSON == "" {
		t.Fatal("expected object_story_spec in POST body")
	}
	var oss map[string]any
	if err := json.Unmarshal([]byte(ossJSON), &oss); err != nil {
		t.Fatalf("parsing object_story_spec: %v", err)
	}
	videoData, ok := oss["video_data"].(map[string]any)
	if !ok {
		t.Fatal("expected video_data in object_story_spec")
	}
	if videoData["video_id"] != videoID {
		t.Errorf("expected video_id %s, got %v", videoID, videoData["video_id"])
	}
	if videoData["image_hash"] != expectHash {
		t.Errorf("expected image_hash %q, got %v", expectHash, videoData["image_hash"])
	}
}

// TestCreateCreative_DoesNotAutoResolveWhenImageHashProvided ensures the
// auto-resolution fast path is a strict opt-out: if the user provides
// ImageHash explicitly, no video GET / CDN download / image upload occurs.
func TestCreateCreative_DoesNotAutoResolveWhenImageHashProvided(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{"id":"cr_hash"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_hash", Name: "User Hash"}),
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn must not be called when ImageHash is supplied")
			return nil, nil
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpDoer must not be called when ImageHash is supplied")
			return nil, nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	_, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "1",
		Name:      "T",
		PageID:    "page_1",
		VideoID:   "vid_no_fetch",
		ImageHash: "user_hash",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("expected zero HTTP calls, got %d", fake.calls)
	}
}

// TestCreateCreative_DoesNotAutoResolveWhenImageURLProvided asserts the same
// opt-out for the ImageURL path — Meta accepts either image_hash or image_url
// inside video_data, so providing the URL must bypass the auto-resolution.
func TestCreateCreative_DoesNotAutoResolveWhenImageURLProvided(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			var oss map[string]any
			if err := json.Unmarshal([]byte(params["object_story_spec"]), &oss); err != nil {
				t.Fatalf("parsing oss: %v", err)
			}
			videoData := oss["video_data"].(map[string]any)
			if videoData["image_url"] != "https://example.com/thumb.jpg" {
				t.Errorf("expected image_url in video_data, got %v", videoData["image_url"])
			}
			if _, ok := videoData["image_hash"]; ok {
				t.Error("expected no image_hash when image_url is supplied")
			}
			return &Response{Body: []byte(`{"id":"cr_url"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_url", Name: "User URL"}),
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn must not be called when ImageURL is supplied")
			return nil, nil
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpDoer must not be called when ImageURL is supplied")
			return nil, nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	_, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "1",
		Name:      "T",
		PageID:    "page_1",
		VideoID:   "vid_url",
		ImageURL:  "https://example.com/thumb.jpg",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("expected zero HTTP calls, got %d", fake.calls)
	}
}

// TestCreateCreative_NoVideoNoAuto asserts image-only creatives remain
// unchanged: no video id means no thumbnail to resolve, regardless of whether
// an image_hash or image_url is supplied.
func TestCreateCreative_NoVideoNoAuto(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{"id":"cr_img_only"}`), StatusCode: 200}, nil
		},
		GetFn: creativeGetFn(t, Creative{ID: "cr_img_only", Name: "Image Only"}),
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn must not be called for image-only creatives")
			return nil, nil
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpDoer must not be called for image-only creatives")
			return nil, nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	_, err := CreateCreative(context.Background(), mock, CreateCreativeParams{
		AccountID: "1",
		Name:      "Image Only",
		PageID:    "page_1",
		ImageHash: "existing",
		Message:   "Msg",
		Link:      "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("expected zero HTTP calls, got %d", fake.calls)
	}
}
