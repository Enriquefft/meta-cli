package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
)

func TestAssetsCommand_VideoStatus(t *testing.T) {
	mc := &mockClient{}
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"vid_456","title":"Test Video","status":{"video_status":"ready","processing_progress":100},"length":5.5}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAssetsCommand(deps)
	stdout, _, err := executeCommand(cmd, "video-status", "vid_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.Video
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.ID != "vid_456" {
		t.Errorf("expected video ID 'vid_456', got %q", result.ID)
	}
	if result.Status.VideoStatus != "ready" {
		t.Errorf("expected video status 'ready', got %q", result.Status.VideoStatus)
	}
}

func TestAssetsCommand_VideoStatus_MissingArg(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewAssetsCommand(deps)
	_, _, err := executeCommand(cmd, "video-status")
	if err == nil {
		t.Error("expected error for missing video ID arg")
	}
}

func TestAssetsCommand_UploadVideo_MissingFile(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	var exitCode int
	originalExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = originalExit }()

	cmd := NewAssetsCommand(deps)
	_, _, _ = executeCommand(cmd, "upload-video")

	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestAssetsCommand_UploadImage_Success(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "banner.jpg")
	if err := os.WriteFile(imagePath, []byte("fake image data"), 0o644); err != nil {
		t.Fatalf("creating temp file: %v", err)
	}

	mc := &mockClient{}
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		if path != "/act_123456/adimages" {
			t.Errorf("expected path /act_123456/adimages, got %s", path)
		}
		if filename != "banner.jpg" {
			t.Errorf("expected filename banner.jpg, got %s", filename)
		}
		if len(params) != 0 {
			t.Errorf("expected no extra params, got %v", params)
		}
		return &meta.Response{
			Body:       []byte(`{"images":{"banner.jpg":{"hash":"img_hash_abc","url":"https://scontent.xx.fbcdn.net/v/full.jpg","url_128":"https://scontent.xx.fbcdn.net/v/thumb.jpg","width":1200,"height":628,"name":"banner.jpg"}}}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAssetsCommand(deps)
	stdout, _, err := executeCommand(cmd, "upload-image", "--file", imagePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.Image
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.Hash != "img_hash_abc" {
		t.Errorf("expected hash 'img_hash_abc', got %q", result.Hash)
	}
	if result.Name != "banner.jpg" {
		t.Errorf("expected name 'banner.jpg', got %q", result.Name)
	}
	if result.Width != 1200 {
		t.Errorf("expected width 1200, got %d", result.Width)
	}
	if result.Height != 628 {
		t.Errorf("expected height 628, got %d", result.Height)
	}
}

func TestAssetsCommand_UploadImage_MissingFile(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	var exitCode int
	originalExit := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = originalExit }()

	cmd := NewAssetsCommand(deps)
	_, _, _ = executeCommand(cmd, "upload-image")

	if exitCode != meta.ExitValidationError {
		t.Errorf("expected exit code %d, got %d", meta.ExitValidationError, exitCode)
	}
}

func TestAssetsCommand_UploadVideo_Success(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "test.mp4")
	if err := os.WriteFile(videoPath, []byte("fake video data"), 0o644); err != nil {
		t.Fatalf("creating temp file: %v", err)
	}

	mc := &mockClient{}
	mc.uploadFn = func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
		// Meta's /advideos POST returns only {"id": ...}.
		return &meta.Response{
			Body:       []byte(`{"id":"vid_new"}`),
			StatusCode: 200,
		}, nil
	}
	// UploadVideo must chain a GET to return the full video record.
	mc.getFn = func(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
		return &meta.Response{
			Body:       []byte(`{"id":"vid_new","title":"Test Upload","status":{"video_status":"processing","processing_progress":0},"length":12.5}`),
			StatusCode: 200,
		}, nil
	}

	deps := testDeps(mc)
	cmd := NewAssetsCommand(deps)
	stdout, _, err := executeCommand(cmd, "upload-video", "--file", videoPath, "--title", "Test Upload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result meta.Video
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result.ID != "vid_new" {
		t.Errorf("expected video ID 'vid_new', got %q", result.ID)
	}
	if result.Title != "Test Upload" {
		t.Errorf("expected title 'Test Upload', got %q", result.Title)
	}
	if result.Status.VideoStatus != "processing" {
		t.Errorf("expected status 'processing', got %q", result.Status.VideoStatus)
	}
	if result.Length != 12.5 {
		t.Errorf("expected length 12.5, got %f", result.Length)
	}
}
