package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
)

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
			body := `{"id":"99887766","title":"product.mp4","upload_status":"processing"}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
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
	if result.Title != "product.mp4" {
		t.Errorf("expected Title product.mp4, got %s", result.Title)
	}
	if result.UploadStatus != "processing" {
		t.Errorf("expected UploadStatus processing, got %s", result.UploadStatus)
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
			return &Response{
				Body:       []byte(`{"id":"1","title":"vid.mp4","upload_status":"processing"}`),
				StatusCode: 200,
			}, nil
		},
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
			return &Response{
				Body:       []byte(`{"id":"1","title":"vid.mp4","upload_status":"processing"}`),
				StatusCode: 200,
			}, nil
		},
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
			return &Response{
				Body:       []byte(`{"id":"1","title":"v.mp4","upload_status":"processing"}`),
				StatusCode: 200,
			}, nil
		},
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

func TestVideoStatus_Processing(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			if path != "/999888" {
				t.Errorf("expected path /999888, got %s", path)
			}
			fields := params.Get("fields")
			if fields != "id,title,status,length" {
				t.Errorf("expected fields id,title,status,length, got %s", fields)
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
