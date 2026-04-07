package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpload_StreamsFile(t *testing.T) {
	var gotContent string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		file, _, err := r.FormFile("source")
		if err != nil {
			t.Errorf("failed to get source file: %v", err)
			w.WriteHeader(500)
			return
		}
		defer func() { _ = file.Close() }()
		b, _ := io.ReadAll(file)
		gotContent = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"vid123"}`))
	})
	defer srv.Close()

	content := "fake video content for streaming test"
	resp, err := client.Upload(context.Background(), "/act_123/advideos", strings.NewReader(content), "video.mp4", int64(len(content)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if gotContent != content {
		t.Errorf("expected file content %q, got %q", content, gotContent)
	}
}

func TestUpload_StreamsParams(t *testing.T) {
	var gotTitle, gotToken string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		gotTitle = r.FormValue("title")
		gotToken = r.FormValue("access_token")
		_, _, _ = r.FormFile("source")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"vid123"}`))
	})
	defer srv.Close()

	resp, err := client.Upload(context.Background(), "/act_123/advideos", strings.NewReader("data"), "video.mp4", 4, map[string]string{"title": "My Video"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTitle != "My Video" {
		t.Errorf("expected title 'My Video', got %s", gotTitle)
	}
	if gotToken != "test-token" {
		t.Errorf("expected access_token 'test-token', got %s", gotToken)
	}
	_ = resp
}

func TestUpload_ReturnsErrorOnFailure(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid param","type":"OAuthException","code":100}}`))
	})
	defer srv.Close()

	resp, err := client.Upload(context.Background(), "/act_123/advideos", strings.NewReader("data"), "video.mp4", 4, nil)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if resp == nil {
		t.Fatal("expected non-nil response even on error")
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// resumableReader wraps a byte slice to implement io.Reader + io.ReaderAt,
// triggering the resumable upload path when size >= resumableThreshold.
type resumableReader struct {
	*bytes.Reader
}

func newResumableUploadServer(t *testing.T, totalSize int64) (*httptest.Server, *Client) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		uploadPhase := r.FormValue("upload_phase")

		switch uploadPhase {
		case "start":
			fileSize := r.FormValue("file_size")
			if fileSize == "" {
				t.Error("start phase missing file_size")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"video_upload_session_id": "session_abc",
				"start_offset":            "0",
				"end_offset":              fmt.Sprintf("%d", chunkSize),
			})

		case "transfer":
			sessionID := r.FormValue("upload_session_id")
			if sessionID != "session_abc" {
				t.Errorf("expected session_id session_abc, got %s", sessionID)
			}

			file, _, err := r.FormFile("video_file_chunk")
			if err != nil {
				t.Errorf("transfer phase missing video_file_chunk: %v", err)
				w.WriteHeader(400)
				return
			}
			chunk, _ := io.ReadAll(file)
			_ = file.Close()

			startOffset := r.FormValue("start_offset")
			nextOffset := fmt.Sprintf("%d", totalSize)
			if startOffset == "0" && totalSize > int64(chunkSize) {
				nextOffset = fmt.Sprintf("%d", len(chunk))
			}

			_ = json.NewEncoder(w).Encode(map[string]string{
				"start_offset": nextOffset,
				"end_offset":   fmt.Sprintf("%d", totalSize),
			})

		case "finish":
			sessionID := r.FormValue("upload_session_id")
			if sessionID != "session_abc" {
				t.Errorf("finish phase expected session_abc, got %s", sessionID)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":  true,
				"video_id": "vid_resumable_123",
			})

		default:
			t.Errorf("unexpected upload_phase: %q", uploadPhase)
			w.WriteHeader(400)
		}
	}))

	c := NewClient(ClientConfig{AccessToken: "test-token", APIVersion: "v21.0"})
	c.baseURL = ts.URL
	c.httpClient = ts.Client()
	return ts, c
}

func TestResumableUpload_SmallFile(t *testing.T) {
	// File just at the resumable threshold: should trigger resumable path
	size := resumableThreshold
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	srv, client := newResumableUploadServer(t, size)
	defer srv.Close()

	reader := &resumableReader{bytes.NewReader(data)}
	resp, err := client.Upload(context.Background(), "/act_123/advideos", reader, "big_video.mp4", size, nil)
	if err != nil {
		t.Fatalf("resumable upload failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["id"] != "vid_resumable_123" {
		t.Errorf("expected video id vid_resumable_123, got %s", result["id"])
	}
}

// plainReader wraps an io.Reader to strip any io.ReaderAt interface.
type plainReader struct {
	io.Reader
}

func TestResumableUpload_FallsBackWithoutReaderAt(t *testing.T) {
	// A plain io.Reader (no ReaderAt) should fall back to simple upload even for large size
	var gotContentType string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_ = r.ParseMultipartForm(10 << 20)
		_, _, _ = r.FormFile("source")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"vid_simple"}`))
	})
	defer srv.Close()

	reader := &plainReader{strings.NewReader("small content")}
	resp, err := client.Upload(context.Background(), "/act_123/advideos", reader, "video.mp4", resumableThreshold, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("expected simple multipart upload, got content-type: %s", gotContentType)
	}
}

func TestResumableUpload_StartSessionError(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request","code":100}}`))
	})
	defer srv.Close()

	data := make([]byte, resumableThreshold)
	reader := &resumableReader{bytes.NewReader(data)}
	_, err := client.Upload(context.Background(), "/act_123/advideos", reader, "video.mp4", resumableThreshold, nil)
	if err == nil {
		t.Fatal("expected error when start session fails")
	}
}

func TestResumableUpload_FinishIncludesVideoID(t *testing.T) {
	size := resumableThreshold
	data := make([]byte, size)

	srv, client := newResumableUploadServer(t, size)
	defer srv.Close()

	reader := &resumableReader{bytes.NewReader(data)}
	resp, err := client.Upload(context.Background(), "/act_123/advideos", reader, "video.mp4", size, map[string]string{"title": "Test"})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	// The resumable upload transport normalizes the finish envelope to the
	// same {"id": "..."} shape returned by the simple upload path. The
	// meta.UploadVideo domain function is responsible for fetching the full
	// video record; the transport only reports the new video id.
	var result map[string]string
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["id"] != "vid_resumable_123" {
		t.Errorf("expected id 'vid_resumable_123', got %q", result["id"])
	}
	if _, ok := result["upload_status"]; ok {
		t.Error("transport must not fabricate upload_status; domain layer owns the full video shape")
	}
}
