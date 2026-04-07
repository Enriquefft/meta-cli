package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// fakeHTTPDoer is the in-memory httpDoer used by ensureVideoThumbnail tests
// to return canned responses without touching the network. It captures every
// incoming request URL so tests can assert the call was well-formed.
type fakeHTTPDoer struct {
	fn       func(req *http.Request) (*http.Response, error)
	calls    int
	lastURL  string
	lastCtx  context.Context
	lastMeth string
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	f.lastURL = req.URL.String()
	f.lastCtx = req.Context()
	f.lastMeth = req.Method
	return f.fn(req)
}

// newFakeHTTPResponse builds a minimal *http.Response for tests.
func newFakeHTTPResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

// videoGetFn returns a GetFn suitable for the MockClient that answers the
// follow-up GET issued by UploadVideo after the upload completes. The returned
// body mirrors the shape the real Graph API sends for the videoFields field
// list.
func videoGetFn(t *testing.T, videoID, title, status string, progress int, length float64) func(ctx context.Context, path string, params url.Values) (*Response, error) {
	t.Helper()
	return func(ctx context.Context, path string, params url.Values) (*Response, error) {
		expectedPath := "/" + videoID
		if path != expectedPath {
			t.Errorf("expected GET path %s, got %s", expectedPath, path)
		}
		if params.Get("fields") != videoFields {
			t.Errorf("expected fields %q, got %q", videoFields, params.Get("fields"))
		}
		body, _ := json.Marshal(map[string]any{
			"id":    videoID,
			"title": title,
			"status": map[string]any{
				"video_status":        status,
				"processing_progress": progress,
			},
			"length": length,
		})
		return &Response{Body: body, StatusCode: 200}, nil
	}
}

func TestUploadVideo_Success(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			if path != "/act_123456/advideos" {
				t.Errorf("expected path /act_123456/advideos, got %s", path)
			}
			if filename != "product.mp4" {
				t.Errorf("expected filename product.mp4, got %s", filename)
			}
			if size != 1024 {
				t.Errorf("expected size 1024, got %d", size)
			}
			if params["title"] != "My Video" {
				t.Errorf("expected title 'My Video', got %s", params["title"])
			}
			// Meta's /advideos endpoint only returns the new video's id.
			return &Response{Body: []byte(`{"id":"99887766"}`), StatusCode: 200}, nil
		},
		GetFn: videoGetFn(t, "99887766", "My Video", "processing", 0, 12.5),
	}

	result, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123456",
		File:      strings.NewReader("fake video data"),
		Filename:  "product.mp4",
		FileSize:  1024,
		Title:     "My Video",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "99887766" {
		t.Errorf("expected ID 99887766, got %s", result.ID)
	}
	if result.Title != "My Video" {
		t.Errorf("expected Title 'My Video', got %s", result.Title)
	}
	if result.Status.VideoStatus != "processing" {
		t.Errorf("expected Status.VideoStatus processing, got %s", result.Status.VideoStatus)
	}
	if result.Length != 12.5 {
		t.Errorf("expected Length 12.5, got %f", result.Length)
	}
}

func TestUploadVideo_FetchesFullRecordAfterUpload(t *testing.T) {
	// Regression: Meta's POST /advideos returns only {"id": ...}. UploadVideo
	// must issue a follow-up GET so callers get the full Video record, not an
	// object with empty title/status/length fields.
	var getCalled bool
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{"id":"vid_abc"}`), StatusCode: 200}, nil
		},
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			getCalled = true
			if path != "/vid_abc" {
				t.Errorf("expected GET path /vid_abc, got %s", path)
			}
			body := `{"id":"vid_abc","title":"Follow-up Title","status":{"video_status":"processing","processing_progress":0},"length":115.066}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
	}

	result, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "clip.mp4",
		FileSize:  4,
		Title:     "Follow-up Title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !getCalled {
		t.Fatal("expected UploadVideo to issue a follow-up GET for full metadata")
	}
	if result.Title != "Follow-up Title" {
		t.Errorf("expected Title 'Follow-up Title', got %q", result.Title)
	}
	if result.Status.VideoStatus != "processing" {
		t.Errorf("expected Status.VideoStatus processing, got %q", result.Status.VideoStatus)
	}
	if result.Length != 115.066 {
		t.Errorf("expected Length 115.066, got %f", result.Length)
	}
}

func TestUploadVideo_CorrectPathAndParams(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string
	var capturedFilename string
	var capturedSize int64

	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			capturedFilename = filename
			capturedSize = size
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: videoGetFn(t, "1", "Test Title", "processing", 0, 0),
	}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "555",
		File:      strings.NewReader("data"),
		Filename:  "vid.mp4",
		FileSize:  2048,
		Title:     "Test Title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_555/advideos" {
		t.Errorf("expected path /act_555/advideos, got %s", capturedPath)
	}
	if capturedFilename != "vid.mp4" {
		t.Errorf("expected filename vid.mp4, got %s", capturedFilename)
	}
	if capturedSize != 2048 {
		t.Errorf("expected size 2048, got %d", capturedSize)
	}
	if capturedParams["title"] != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", capturedParams["title"])
	}
}

func TestUploadVideo_NoTitle(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: videoGetFn(t, "1", "", "processing", 0, 0),
	}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "vid.mp4",
		FileSize:  512,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := capturedParams["title"]; ok {
		t.Error("expected no title param when Title is empty")
	}
}

func TestUploadVideo_NormalizesAccountID(t *testing.T) {
	var capturedPath string

	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			capturedPath = path
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: videoGetFn(t, "1", "v.mp4", "processing", 0, 0),
	}

	// User includes "act_" prefix — should be normalized
	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "act_999",
		File:      strings.NewReader("data"),
		Filename:  "v.mp4",
		FileSize:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_999/advideos" {
		t.Errorf("expected path /act_999/advideos, got %s", capturedPath)
	}
}

func TestUploadVideo_MissingAccountID(t *testing.T) {
	mock := &MockClient{}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		File:     strings.NewReader("data"),
		Filename: "v.mp4",
		FileSize: 100,
	})
	if err == nil {
		t.Fatal("expected validation error for missing AccountID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestUploadVideo_MissingFile(t *testing.T) {
	mock := &MockClient{}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		Filename:  "v.mp4",
		FileSize:  100,
	})
	if err == nil {
		t.Fatal("expected validation error for missing File")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestUploadVideo_MissingFilename(t *testing.T) {
	mock := &MockClient{}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		FileSize:  100,
	})
	if err == nil {
		t.Fatal("expected validation error for missing Filename")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestUploadVideo_ClientError(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return nil, fmt.Errorf("upload failed: connection reset")
		},
	}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "v.mp4",
		FileSize:  100,
	})
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("expected original error to be propagated, got: %v", err)
	}
}

func TestUploadVideo_MissingIDInUploadResponse(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{}`), StatusCode: 200}, nil
		},
	}

	_, err := UploadVideo(context.Background(), mock, UploadVideoParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "v.mp4",
		FileSize:  100,
	})
	if err == nil {
		t.Fatal("expected error when upload response omits id")
	}
	if !strings.Contains(err.Error(), "video id") {
		t.Errorf("expected error to mention missing video id, got: %v", err)
	}
}

// standardImageResponse returns the canonical Meta response body for a
// successful /adimages upload, keyed by filename.
func standardImageResponse(filename, hash string) []byte {
	body, _ := json.Marshal(map[string]any{
		"images": map[string]any{
			filename: map[string]any{
				"hash":    hash,
				"url":     "https://scontent.xx.fbcdn.net/v/full.jpg",
				"url_128": "https://scontent.xx.fbcdn.net/v/thumb.jpg",
				"width":   1200,
				"height":  628,
				"name":    filename,
			},
		},
	})
	return body
}

func TestUploadImage_Success(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			if path != "/act_123456/adimages" {
				t.Errorf("expected path /act_123456/adimages, got %s", path)
			}
			if filename != "photo.jpg" {
				t.Errorf("expected filename photo.jpg, got %s", filename)
			}
			if size != 2048 {
				t.Errorf("expected size 2048, got %d", size)
			}
			return &Response{Body: standardImageResponse("photo.jpg", "abc123def456"), StatusCode: 200}, nil
		},
	}

	result, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "123456",
		File:      strings.NewReader("fake image data"),
		Filename:  "photo.jpg",
		FileSize:  2048,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hash != "abc123def456" {
		t.Errorf("expected Hash abc123def456, got %s", result.Hash)
	}
	if result.Name != "photo.jpg" {
		t.Errorf("expected Name photo.jpg, got %s", result.Name)
	}
	if result.Width != 1200 {
		t.Errorf("expected Width 1200, got %d", result.Width)
	}
	if result.Height != 628 {
		t.Errorf("expected Height 628, got %d", result.Height)
	}
	if result.URL == "" {
		t.Error("expected URL to be populated")
	}
	if result.URL128 == "" {
		t.Error("expected URL128 to be populated")
	}
}

func TestUploadImage_CorrectPathAndParams(t *testing.T) {
	var capturedPath string
	var capturedFilename string
	var capturedSize int64
	var capturedParams map[string]string

	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedFilename = filename
			capturedSize = size
			capturedParams = params
			return &Response{Body: standardImageResponse("banner.png", "hash_xyz"), StatusCode: 200}, nil
		},
	}

	_, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "789",
		File:      strings.NewReader("img bytes"),
		Filename:  "banner.png",
		FileSize:  4096,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_789/adimages" {
		t.Errorf("expected path /act_789/adimages, got %s", capturedPath)
	}
	if capturedFilename != "banner.png" {
		t.Errorf("expected filename banner.png, got %s", capturedFilename)
	}
	if capturedSize != 4096 {
		t.Errorf("expected size 4096, got %d", capturedSize)
	}
	if len(capturedParams) != 0 {
		t.Errorf("expected no extra params, got %v", capturedParams)
	}
}

func TestUploadImage_NormalizesAccountID(t *testing.T) {
	paths := make(map[string]string)

	for _, accID := range []string{"act_999", "999"} {
		key := accID
		mock := &MockClient{
			UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
				paths[key] = path
				return &Response{Body: standardImageResponse("i.jpg", "h"), StatusCode: 200}, nil
			},
		}

		_, err := UploadImage(context.Background(), mock, UploadImageParams{
			AccountID: accID,
			File:      strings.NewReader("data"),
			Filename:  "i.jpg",
			FileSize:  64,
		})
		if err != nil {
			t.Fatalf("unexpected error for AccountID %q: %v", accID, err)
		}
	}

	if paths["act_999"] != paths["999"] {
		t.Errorf("expected normalized path to match: act_999=%q, 999=%q", paths["act_999"], paths["999"])
	}
	if paths["act_999"] != "/act_999/adimages" {
		t.Errorf("expected path /act_999/adimages, got %s", paths["act_999"])
	}
}

func TestUploadImage_ValidationErrors(t *testing.T) {
	// These mocks must never be called; validation must fail before upload.
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn should not be called when validation fails")
			return nil, nil
		},
	}

	tests := []struct {
		name   string
		params UploadImageParams
	}{
		{
			name: "missing AccountID",
			params: UploadImageParams{
				File:     strings.NewReader("data"),
				Filename: "i.jpg",
				FileSize: 10,
			},
		},
		{
			name: "missing File",
			params: UploadImageParams{
				AccountID: "123",
				Filename:  "i.jpg",
				FileSize:  10,
			},
		},
		{
			name: "missing Filename",
			params: UploadImageParams{
				AccountID: "123",
				File:      strings.NewReader("data"),
				FileSize:  10,
			},
		},
		{
			name: "zero FileSize",
			params: UploadImageParams{
				AccountID: "123",
				File:      strings.NewReader("data"),
				Filename:  "i.jpg",
				FileSize:  0,
			},
		},
		{
			name: "negative FileSize",
			params: UploadImageParams{
				AccountID: "123",
				File:      strings.NewReader("data"),
				Filename:  "i.jpg",
				FileSize:  -1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UploadImage(context.Background(), mock, tc.params)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), "validation") {
				t.Errorf("expected validation error, got: %v", err)
			}
		})
	}
}

func TestUploadImage_FilenameRewrittenByMeta(t *testing.T) {
	// Meta sometimes returns the entry under a rewritten key (e.g. without
	// the extension). Fall back to the first entry and propagate the Name.
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			body, _ := json.Marshal(map[string]any{
				"images": map[string]any{
					"photo": map[string]any{ // key without extension
						"hash": "rewritten_hash",
						"url":  "https://scontent.xx.fbcdn.net/v/x.jpg",
					},
				},
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	result, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "photo.jpg",
		FileSize:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hash != "rewritten_hash" {
		t.Errorf("expected Hash rewritten_hash, got %s", result.Hash)
	}
	// Name should be enriched from the request filename.
	if result.Name != "photo.jpg" {
		t.Errorf("expected Name photo.jpg, got %s", result.Name)
	}
}

func TestUploadImage_ResponseMissingImage(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{"images":{}}`), StatusCode: 200}, nil
		},
	}

	_, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "i.jpg",
		FileSize:  10,
	})
	if err == nil {
		t.Fatal("expected error when response contains no images")
	}
	if !strings.Contains(err.Error(), "no images") {
		t.Errorf("expected meaningful error mentioning missing images, got: %v", err)
	}
}

func TestUploadImage_ClientError(t *testing.T) {
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return nil, fmt.Errorf("upload failed: connection reset")
		},
	}

	_, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "i.jpg",
		FileSize:  10,
	})
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("expected original error to be propagated, got: %v", err)
	}
}

func TestUploadImage_GraphError(t *testing.T) {
	graphErr := &GraphError{
		Message: "Invalid image file",
		Type:    "OAuthException",
		Code:    100,
	}
	mock := &MockClient{
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return &Response{
				Body:       []byte(`{"error":{"message":"Invalid image file","type":"OAuthException","code":100}}`),
				StatusCode: 400,
			}, graphErr
		},
	}

	_, err := UploadImage(context.Background(), mock, UploadImageParams{
		AccountID: "123",
		File:      strings.NewReader("data"),
		Filename:  "i.jpg",
		FileSize:  10,
	})
	if err == nil {
		t.Fatal("expected GraphError to be propagated")
	}
	ge, ok := err.(*GraphError)
	if !ok {
		t.Fatalf("expected *GraphError, got %T: %v", err, err)
	}
	if ge.Code != 100 {
		t.Errorf("expected GraphError code 100, got %d", ge.Code)
	}
}

func TestVideoStatus_Processing(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			if path != "/999888" {
				t.Errorf("expected path /999888, got %s", path)
			}
			fields := params.Get("fields")
			if fields != videoFields {
				t.Errorf("expected fields %q, got %q", videoFields, fields)
			}
			body := `{
				"id": "999888",
				"title": "test-video.mp4",
				"status": {
					"video_status": "processing",
					"processing_progress": 42
				},
				"length": 0
			}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
	}

	result, err := GetVideoStatus(context.Background(), mock, VideoStatusParams{
		VideoID: "999888",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "999888" {
		t.Errorf("expected ID 999888, got %s", result.ID)
	}
	if result.Title != "test-video.mp4" {
		t.Errorf("expected Title test-video.mp4, got %s", result.Title)
	}
	if result.Status.VideoStatus != "processing" {
		t.Errorf("expected VideoStatus processing, got %s", result.Status.VideoStatus)
	}
	if result.Status.ProcessingProgress != 42 {
		t.Errorf("expected ProcessingProgress 42, got %d", result.Status.ProcessingProgress)
	}
	if result.Length != 0 {
		t.Errorf("expected Length 0, got %f", result.Length)
	}
}

func TestVideoStatus_Ready(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			body, _ := json.Marshal(map[string]interface{}{
				"id":    "111222",
				"title": "finished.mp4",
				"status": map[string]interface{}{
					"video_status":        "ready",
					"processing_progress": 100,
				},
				"length": 30.5,
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	result, err := GetVideoStatus(context.Background(), mock, VideoStatusParams{
		VideoID: "111222",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "111222" {
		t.Errorf("expected ID 111222, got %s", result.ID)
	}
	if result.Status.VideoStatus != "ready" {
		t.Errorf("expected VideoStatus ready, got %s", result.Status.VideoStatus)
	}
	if result.Status.ProcessingProgress != 100 {
		t.Errorf("expected ProcessingProgress 100, got %d", result.Status.ProcessingProgress)
	}
	if result.Length != 30.5 {
		t.Errorf("expected Length 30.5, got %f", result.Length)
	}
}

func TestVideoStatus_MissingVideoID(t *testing.T) {
	mock := &MockClient{}

	_, err := GetVideoStatus(context.Background(), mock, VideoStatusParams{})
	if err == nil {
		t.Fatal("expected validation error for missing VideoID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestVideoStatus_ClientError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return nil, fmt.Errorf("server error: 500")
		},
	}

	_, err := GetVideoStatus(context.Background(), mock, VideoStatusParams{
		VideoID: "123",
	})
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("expected original error to be propagated, got: %v", err)
	}
}

// TestEnsureVideoThumbnail_Passthrough asserts the fast path: when the caller
// already has an image_hash, the helper returns it without touching the Graph
// API or the HTTP CDN transport. This is load-bearing because CreateCreative
// unconditionally routes through ensureVideoThumbnail.
func TestEnsureVideoThumbnail_Passthrough(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			t.Fatal("GetFn must not be called when imageHash is already set")
			return nil, nil
		},
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn must not be called when imageHash is already set")
			return nil, nil
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpDoer must not be called when imageHash is already set")
			return nil, nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	got, err := ensureVideoThumbnail(context.Background(), mock, "123", "vid_456", "existing_hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "existing_hash" {
		t.Errorf("expected existing_hash, got %q", got)
	}
	if fake.calls != 0 {
		t.Errorf("expected zero HTTP calls, got %d", fake.calls)
	}
}

// TestEnsureVideoThumbnail_FetchesAndUploads is the happy-path regression
// test: the helper must GET the video record to discover Video.Picture,
// download the bytes over plain HTTP, and POST them back to /adimages to
// obtain a new image_hash.
func TestEnsureVideoThumbnail_FetchesAndUploads(t *testing.T) {
	const (
		accountID   = "123456"
		videoID     = "vid_thumb_happy"
		pictureURL  = "https://scontent.xx.fbcdn.net/v/t15/auto.jpg"
		expectHash  = "thumb_hash_xyz"
		jpegPayload = "fake-jpeg-bytes"
	)

	var events []string
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			events = append(events, "get:"+path)
			if path != "/"+videoID {
				t.Errorf("expected GET path /%s, got %s", videoID, path)
			}
			if params.Get("fields") != videoFields {
				t.Errorf("expected fields %q, got %q", videoFields, params.Get("fields"))
			}
			body, _ := json.Marshal(map[string]any{
				"id":      videoID,
				"title":   "Ready Video",
				"length":  12.3,
				"picture": pictureURL,
				"status": map[string]any{
					"video_status":        "ready",
					"processing_progress": 100,
				},
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			events = append(events, "upload:"+path)
			wantPath := "/act_" + accountID + "/adimages"
			if path != wantPath {
				t.Errorf("expected Upload path %s, got %s", wantPath, path)
			}
			wantName := "thumb_" + videoID + ".jpg"
			if filename != wantName {
				t.Errorf("expected filename %s, got %s", wantName, filename)
			}
			data, readErr := io.ReadAll(file)
			if readErr != nil {
				t.Fatalf("reading upload file: %v", readErr)
			}
			if string(data) != jpegPayload {
				t.Errorf("expected upload body %q, got %q", jpegPayload, string(data))
			}
			if size != int64(len(jpegPayload)) {
				t.Errorf("expected upload size %d, got %d", len(jpegPayload), size)
			}
			return &Response{
				Body:       standardImageResponse(wantName, expectHash),
				StatusCode: 200,
			}, nil
		},
	}

	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			events = append(events, "http:"+req.URL.String())
			if req.URL.String() != pictureURL {
				t.Errorf("expected HTTP GET %s, got %s", pictureURL, req.URL.String())
			}
			if req.Method != http.MethodGet {
				t.Errorf("expected HTTP method GET, got %s", req.Method)
			}
			if req.Context() == nil {
				t.Error("expected request to carry a context")
			}
			return newFakeHTTPResponse(200, []byte(jpegPayload)), nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	got, err := ensureVideoThumbnail(context.Background(), mock, accountID, videoID, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectHash {
		t.Errorf("expected hash %q, got %q", expectHash, got)
	}
	wantOrder := []string{
		"get:/" + videoID,
		"http:" + pictureURL,
		"upload:/act_" + accountID + "/adimages",
	}
	if len(events) != len(wantOrder) {
		t.Fatalf("expected %d events, got %d: %v", len(wantOrder), len(events), events)
	}
	for i, want := range wantOrder {
		if events[i] != want {
			t.Errorf("event %d: expected %q, got %q", i, want, events[i])
		}
	}
}

// TestEnsureVideoThumbnail_VideoMissingPicture asserts we fail loudly with an
// actionable message when Meta returns a video record that does not yet have
// a generated thumbnail (e.g. still encoding).
func TestEnsureVideoThumbnail_VideoMissingPicture(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			body, _ := json.Marshal(map[string]any{
				"id":    "vid_no_pic",
				"title": "Encoding",
				"status": map[string]any{
					"video_status":        "processing",
					"processing_progress": 42,
				},
				"length": 0,
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			t.Fatal("UploadFn must not be called when Picture is empty")
			return nil, nil
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpDoer must not be called when Picture is empty")
			return nil, nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	_, err := ensureVideoThumbnail(context.Background(), mock, "acct_1", "vid_no_pic", "")
	if err == nil {
		t.Fatal("expected error for video with empty picture")
	}
	if !strings.Contains(err.Error(), "thumbnail picture") {
		t.Errorf("expected error to mention missing thumbnail picture, got: %v", err)
	}
	if !strings.Contains(err.Error(), "vid_no_pic") {
		t.Errorf("expected error to mention the video id, got: %v", err)
	}
}

// TestEnsureVideoThumbnail_DownloadFails covers both transport-level errors
// and non-2xx responses from the CDN — both must be wrapped with context.
func TestEnsureVideoThumbnail_DownloadFails(t *testing.T) {
	videoResponseBody, _ := json.Marshal(map[string]any{
		"id":      "vid_dl_fail",
		"title":   "T",
		"picture": "https://scontent.xx.fbcdn.net/v/missing.jpg",
		"status": map[string]any{
			"video_status":        "ready",
			"processing_progress": 100,
		},
		"length": 1.0,
	})
	newMock := func() *MockClient {
		return &MockClient{
			GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
				return &Response{Body: videoResponseBody, StatusCode: 200}, nil
			},
			UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
				t.Fatal("UploadFn must not be called when thumbnail download fails")
				return nil, nil
			},
		}
	}

	t.Run("transport error", func(t *testing.T) {
		fake := &fakeHTTPDoer{
			fn: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("connection reset by peer")
			},
		}
		restore := setThumbnailHTTPClient(fake)
		defer restore()

		_, err := ensureVideoThumbnail(context.Background(), newMock(), "123", "vid_dl_fail", "")
		if err == nil {
			t.Fatal("expected error on transport failure")
		}
		if !strings.Contains(err.Error(), "downloading video thumbnail") {
			t.Errorf("expected error wrapped with 'downloading video thumbnail', got: %v", err)
		}
		if !strings.Contains(err.Error(), "connection reset by peer") {
			t.Errorf("expected original error to be preserved, got: %v", err)
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		fake := &fakeHTTPDoer{
			fn: func(req *http.Request) (*http.Response, error) {
				return newFakeHTTPResponse(404, []byte("not found")), nil
			},
		}
		restore := setThumbnailHTTPClient(fake)
		defer restore()

		_, err := ensureVideoThumbnail(context.Background(), newMock(), "123", "vid_dl_fail", "")
		if err == nil {
			t.Fatal("expected error on non-2xx status")
		}
		if !strings.Contains(err.Error(), "downloading video thumbnail") {
			t.Errorf("expected error wrapped with 'downloading video thumbnail', got: %v", err)
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("expected error to mention status code, got: %v", err)
		}
	})
}

// TestEnsureVideoThumbnail_UploadFails asserts UploadImage errors propagate
// with wrapping context so the caller can tell which phase failed.
func TestEnsureVideoThumbnail_UploadFails(t *testing.T) {
	videoResponseBody, _ := json.Marshal(map[string]any{
		"id":      "vid_up_fail",
		"title":   "T",
		"picture": "https://scontent.xx.fbcdn.net/v/up.jpg",
		"status": map[string]any{
			"video_status":        "ready",
			"processing_progress": 100,
		},
		"length": 1.0,
	})

	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return &Response{Body: videoResponseBody, StatusCode: 200}, nil
		},
		UploadFn: func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
			return nil, fmt.Errorf("invalid image format")
		},
	}
	fake := &fakeHTTPDoer{
		fn: func(req *http.Request) (*http.Response, error) {
			return newFakeHTTPResponse(200, []byte("jpeg-data")), nil
		},
	}
	restore := setThumbnailHTTPClient(fake)
	defer restore()

	_, err := ensureVideoThumbnail(context.Background(), mock, "123", "vid_up_fail", "")
	if err == nil {
		t.Fatal("expected error on upload failure")
	}
	if !strings.Contains(err.Error(), "uploading video thumbnail") {
		t.Errorf("expected error wrapped with 'uploading video thumbnail', got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid image format") {
		t.Errorf("expected original error preserved, got: %v", err)
	}
}
