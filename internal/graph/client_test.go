package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestClientServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	ts := httptest.NewServer(handler)
	c := NewClient(ClientConfig{
		AccessToken: "test-token",
		APIVersion:  "v21.0",
	})
	c.baseURL = ts.URL
	c.httpClient = ts.Client()
	return ts, c
}

func TestGet_IncludesAccessToken(t *testing.T) {
	var gotToken string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("access_token")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"me"}`))
	})
	defer srv.Close()

	_, err := client.Get(context.Background(), "/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "test-token" {
		t.Errorf("expected access_token in query, got %s", gotToken)
	}
}

func TestGet_IncludesAPIVersion(t *testing.T) {
	c := NewClient(ClientConfig{AccessToken: "tok", APIVersion: "v21.0"})
	if !strings.Contains(c.baseURL, "/v21.0") {
		t.Errorf("expected baseURL to include /v21.0, got %s", c.baseURL)
	}
}

func TestGet_PassesParams(t *testing.T) {
	var gotFields string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		gotFields = r.URL.Query().Get("fields")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	params := url.Values{"fields": {"id,name"}}
	_, err := client.Get(context.Background(), "/me/adaccounts", params)
	if err != nil {
		t.Fatal(err)
	}
	if gotFields != "id,name" {
		t.Errorf("expected fields=id,name, got %s", gotFields)
	}
}

func TestPost_IncludesAccessToken(t *testing.T) {
	var gotToken string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotToken = r.FormValue("access_token")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	defer srv.Close()

	_, err := client.Post(context.Background(), "/act_123/campaigns", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "test-token" {
		t.Errorf("expected access_token in form, got %s", gotToken)
	}
}

func TestPost_DryRunInjectsValidateOnly(t *testing.T) {
	var gotExec string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotExec = r.FormValue("execution_options")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	defer srv.Close()

	client.SetDryRun(true)
	_, err := client.Post(context.Background(), "/act_123/campaigns", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if gotExec != `["validate_only"]` {
		t.Errorf("expected validate_only execution_options, got %s", gotExec)
	}
}

func TestPost_ReturnsResponse(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"120212345678901234"}`))
	})
	defer srv.Close()

	resp, err := client.Post(context.Background(), "/act_123/campaigns", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != "120212345678901234" {
		t.Errorf("expected id 120212345678901234, got %v", result["id"])
	}
}

func TestGet_APIError(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":467}}`))
	})
	defer srv.Close()

	resp, err := client.Get(context.Background(), "/me", nil)
	if err == nil {
		t.Fatal("expected error on 401 response")
	}
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUpload_Multipart(t *testing.T) {
	var gotFilename string
	var gotContentType string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_ = r.ParseMultipartForm(10 << 20)
		file, hdr, err := r.FormFile("source")
		if err != nil {
			t.Errorf("failed to get source file: %v", err)
			w.WriteHeader(500)
			return
		}
		defer func() { _ = file.Close() }()
		gotFilename = hdr.Filename
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"vid123"}`))
	})
	defer srv.Close()

	fileContent := strings.NewReader("fake video content")
	resp, err := client.Upload(context.Background(), "/act_123/advideos", fileContent, "video.mp4", 19, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if gotFilename != "video.mp4" {
		t.Errorf("expected filename video.mp4, got %s", gotFilename)
	}
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("expected multipart content type, got %s", gotContentType)
	}
}

func TestUpload_PassesParams(t *testing.T) {
	var gotTitle string
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		gotTitle = r.FormValue("title")
		_, _, _ = r.FormFile("source")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"vid123"}`))
	})
	defer srv.Close()

	fileContent := strings.NewReader("fake video")
	resp, err := client.Upload(context.Background(), "/act_123/advideos", fileContent, "video.mp4", 10, map[string]string{"title": "My Video"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTitle != "My Video" {
		t.Errorf("expected title 'My Video', got %s", gotTitle)
	}
	_ = resp
}

func TestGet_CancelledContext(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Get(ctx, "/me", nil)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestNewClient_SetsBaseURL(t *testing.T) {
	c := NewClient(ClientConfig{AccessToken: "tok", APIVersion: "v21.0"})
	if c.baseURL != "https://graph.facebook.com/v21.0" {
		t.Errorf("expected baseURL with version, got %s", c.baseURL)
	}
}

func TestPaginate_ReturnsIterator(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":[{"id":"1"}]}`))
	})
	defer srv.Close()

	it := client.Paginate(context.Background(), "/me/adaccounts", nil)
	if it == nil {
		t.Error("expected non-nil iterator")
	}
}

func TestGet_RetriesOn500(t *testing.T) {
	calls := 0
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":{"message":"temp","type":"OAuthException","code":1}}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"me"}`))
	})
	defer srv.Close()

	client.retry.BaseDelay = 1 * time.Millisecond
	resp, err := client.Get(context.Background(), "/me", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestGet_RetriesOn429(t *testing.T) {
	calls := 0
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"OAuthException","code":17}}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"me"}`))
	})
	defer srv.Close()

	client.retry.BaseDelay = 1 * time.Millisecond
	resp, err := client.Get(context.Background(), "/me", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPost_RetriesOn502(t *testing.T) {
	calls := 0
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(502)
			_, _ = w.Write([]byte(`bad gateway`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	defer srv.Close()

	client.retry.BaseDelay = 1 * time.Millisecond
	resp, err := client.Post(context.Background(), "/path", map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPost_DoesNotMutateCallerParams(t *testing.T) {
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	defer srv.Close()

	params := map[string]string{"name": "original"}
	_, err := client.Post(context.Background(), "/path", params)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := params["access_token"]; ok {
		t.Error("Post mutated caller's params map by adding access_token")
	}
	if _, ok := params["execution_options"]; ok {
		t.Error("Post mutated caller's params map by adding execution_options")
	}
	if params["name"] != "original" {
		t.Errorf("Post mutated caller's params, name = %q", params["name"])
	}
}

func TestRedactURL(t *testing.T) {
	original := "https://graph.facebook.com/v21.0/me?access_token=secret123&fields=id"
	redacted := redactURL(original)
	if strings.Contains(redacted, "secret123") {
		t.Errorf("URL not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "REDACTED") {
		t.Errorf("expected REDACTED in URL: %s", redacted)
	}
	if !strings.Contains(redacted, "fields=id") {
		t.Errorf("expected fields param preserved: %s", redacted)
	}
}

func TestRedactParams(t *testing.T) {
	original := url.Values{}
	original.Set("access_token", "secret123")
	original.Set("name", "test")
	redacted := redactParams(original.Encode())
	if strings.Contains(redacted, "secret123") {
		t.Errorf("params not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "REDACTED") {
		t.Errorf("expected REDACTED in params: %s", redacted)
	}
	if !strings.Contains(redacted, "name=test") {
		t.Errorf("expected name param preserved: %s", redacted)
	}
}

func TestPost_NoRetryOnAuthError(t *testing.T) {
	calls := 0
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190,"error_subcode":467}}`))
	})
	defer srv.Close()

	client.retry.BaseDelay = 1 * time.Millisecond
	_, err := client.Post(context.Background(), "/path", map[string]string{"name": "test"})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if calls != 1 {
		t.Errorf("expected no retries on 401, got %d calls", calls)
	}
}

func TestGet_RateLimitBackoff(t *testing.T) {
	calls := 0
	srv, client := newTestClientServer(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("X-Business-Use-Case-Usage", `{"acc":[{"call_count":80,"total_cputime":50,"total_time":60,"estimated_time_to_reset":300}]}`)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"me"}`))
	})
	defer srv.Close()

	backoffCalled := false
	client.backoffFn = func(usage int) time.Duration {
		backoffCalled = true
		if usage < 75 {
			t.Errorf("expected usage >= 75, got %d", usage)
		}
		return 1 * time.Millisecond
	}

	_, err := client.Get(context.Background(), "/me", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Get(context.Background(), "/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !backoffCalled {
		t.Error("expected backoff function to be called on second request")
	}
}
