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
