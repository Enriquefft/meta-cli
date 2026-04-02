package graph

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUpload_StreamsFile(t *testing.T) {
	var gotContent string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		file, _, err := r.FormFile("source")
		if err != nil {
			t.Errorf("failed to get source file: %v", err)
			w.WriteHeader(500)
			return
		}
		defer file.Close()
		b, _ := io.ReadAll(file)
		gotContent = string(b)
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"vid123"}`))
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
		r.ParseMultipartForm(10 << 20)
		gotTitle = r.FormValue("title")
		gotToken = r.FormValue("access_token")
		_, _, _ = r.FormFile("source")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"vid123"}`))
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
		w.Write([]byte(`{"error":{"message":"Invalid param","type":"OAuthException","code":100}}`))
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
