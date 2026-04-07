package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestUpdater wires an Updater at the given test servers without any
// network calls leaking to real GitHub.
func newTestUpdater(apiURL, rawURL string) *Updater {
	return New(Config{
		Owner:      "enriquefft",
		Repo:       "meta-cli",
		APIBaseURL: apiURL,
		RawBaseURL: rawURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
}

func TestLatest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/enriquefft/meta-cli/releases/latest" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("User-Agent header must be set")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/enriquefft/meta-cli/releases/tag/v0.3.0","published_at":"2026-04-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.TagName != "v0.3.0" {
		t.Errorf("TagName = %q", rel.TagName)
	}
	if rel.HTMLURL == "" {
		t.Error("HTMLURL should be populated")
	}
	if rel.PublishedAt == "" {
		t.Error("PublishedAt should be populated")
	}
}

func TestLatest_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	_, err := u.Latest(context.Background())
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status 404, got: %v", err)
	}
}

func TestLatest_RespectsContext(t *testing.T) {
	// A handler that sleeps long enough for the context to cancel first.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := u.Latest(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestLatest_MissingTagName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	_, err := u.Latest(context.Background())
	if err == nil {
		t.Fatal("expected error when tag_name is missing")
	}
}

func TestCheck_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.1"}`))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	status, err := u.Check(context.Background(), "0.2.1")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !status.UpToDate {
		t.Errorf("expected UpToDate=true, got false (current=%q latest=%q)", status.Current, status.Latest)
	}
	if status.Current != "0.2.1" {
		t.Errorf("Current = %q, want 0.2.1", status.Current)
	}
	if status.Latest != "0.2.1" {
		t.Errorf("Latest = %q, want 0.2.1", status.Latest)
	}
}

func TestCheck_Outdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"0.2.1"}`))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	status, err := u.Check(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status.UpToDate {
		t.Errorf("expected UpToDate=false, got true (current=%q latest=%q)", status.Current, status.Latest)
	}
	if status.Current != "0.2.0" {
		t.Errorf("Current = %q, want 0.2.0", status.Current)
	}
	if status.Latest != "0.2.1" {
		t.Errorf("Latest = %q, want 0.2.1", status.Latest)
	}
}

func TestCheck_StripsLeadingV(t *testing.T) {
	cases := []struct {
		name    string
		current string
		tag     string
		equal   bool
	}{
		{"both bare", "0.3.0", "0.3.0", true},
		{"both prefixed", "v0.3.0", "v0.3.0", true},
		{"current prefixed", "v0.3.0", "0.3.0", true},
		{"latest prefixed", "0.3.0", "v0.3.0", true},
		{"different", "v0.3.0", "v0.3.1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"tag_name":%q}`, tc.tag)
			}))
			defer srv.Close()

			u := newTestUpdater(srv.URL, srv.URL)
			status, err := u.Check(context.Background(), tc.current)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if status.UpToDate != tc.equal {
				t.Errorf("UpToDate = %v, want %v (current=%q latest=%q)", status.UpToDate, tc.equal, status.Current, status.Latest)
			}
		})
	}
}

func TestDetectInstallMethod_Windows(t *testing.T) {
	orig := goosFunc
	goosFunc = func() string { return "windows" }
	defer func() { goosFunc = orig }()

	if got := DetectInstallMethod(""); got != MethodWindows {
		t.Errorf("DetectInstallMethod = %v, want MethodWindows", got)
	}
}

func TestDetectInstallMethod_GoInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows path short-circuits before build info")
	}
	origGoos := goosFunc
	goosFunc = func() string { return "linux" }
	defer func() { goosFunc = origGoos }()

	origBuild := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.2.1"}}, true
	}
	defer func() { readBuildInfo = origBuild }()

	if got := DetectInstallMethod(""); got != MethodGoInstall {
		t.Errorf("DetectInstallMethod = %v, want MethodGoInstall", got)
	}
}

func TestDetectInstallMethod_Script(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows path short-circuits before build info")
	}
	origGoos := goosFunc
	goosFunc = func() string { return "linux" }
	defer func() { goosFunc = origGoos }()

	origBuild := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
	}
	defer func() { readBuildInfo = origBuild }()

	if got := DetectInstallMethod(""); got != MethodScript {
		t.Errorf("DetectInstallMethod = %v, want MethodScript", got)
	}
}

func TestDetectInstallMethod_NoBuildInfo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows path short-circuits before build info")
	}
	origGoos := goosFunc
	goosFunc = func() string { return "linux" }
	defer func() { goosFunc = origGoos }()

	origBuild := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	defer func() { readBuildInfo = origBuild }()

	if got := DetectInstallMethod(""); got != MethodScript {
		t.Errorf("DetectInstallMethod = %v, want MethodScript", got)
	}
}

func TestInstallMethod_String(t *testing.T) {
	cases := map[InstallMethod]string{
		MethodUnknown:   "unknown",
		MethodScript:    "script",
		MethodGoInstall: "go-install",
		MethodWindows:   "windows",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", m, got, want)
		}
	}
}

func TestRunInstallScript_PassesEnvAndExecutes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is POSIX-only; RunInstallScript refuses Windows higher up")
	}

	// Fake install.sh that simply prints the two env vars we care about.
	// The update command inherits stdout, so we capture it via the test
	// seam runInstallScript.
	const fakeScript = `#!/bin/sh
echo "DIR=${META_CLI_INSTALL_DIR}"
echo "VER=${META_CLI_VERSION}"
`

	var observedPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath.Store(r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fakeScript))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)

	tmpDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := u.runInstallScript(context.Background(), "0.2.1", tmpDir, nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runInstallScript: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "DIR="+tmpDir) {
		t.Errorf("stdout missing META_CLI_INSTALL_DIR export, got: %q", out)
	}
	if !strings.Contains(out, "VER=0.2.1") {
		t.Errorf("stdout missing META_CLI_VERSION export, got: %q", out)
	}

	got, _ := observedPath.Load().(string)
	if want := "/enriquefft/meta-cli/v0.2.1/install.sh"; got != want {
		t.Errorf("download path = %q, want %q", got, want)
	}
}

func TestRunInstallScript_RejectsURLDownloadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is POSIX-only")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	err := u.runInstallScript(context.Background(), "0.2.1", t.TempDir(), nil, os.Stdout, os.Stderr)
	if err == nil {
		t.Fatal("expected error when install.sh download returns 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestRunInstallScript_PinsToTagURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is POSIX-only")
	}
	var observedPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedPath.Store(r.URL.Path)
		// A no-op script so execution succeeds.
		_, _ = w.Write([]byte("#!/bin/sh\nexit 0\n"))
	}))
	defer srv.Close()

	u := newTestUpdater(srv.URL, srv.URL)
	if err := u.runInstallScript(context.Background(), "0.2.1", t.TempDir(), nil, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("runInstallScript: %v", err)
	}

	got, _ := observedPath.Load().(string)
	if strings.Contains(got, "/main/") {
		t.Errorf("install.sh URL must be pinned to the tag, not main; got path %q", got)
	}
	if !strings.HasSuffix(got, "/v0.2.1/install.sh") {
		t.Errorf("install.sh URL must end with /v0.2.1/install.sh, got %q", got)
	}
}

func TestRunInstallScript_RequiresVersionAndDir(t *testing.T) {
	u := New(Config{})
	if err := u.runInstallScript(context.Background(), "", "/tmp", nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Error("expected error for empty version")
	}
	if err := u.runInstallScript(context.Background(), "0.2.1", "", nil, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Error("expected error for empty install directory")
	}
}

func TestStripLeadingV(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"v":      "",
		"V0.1.0": "0.1.0",
		"v0.2.1": "0.2.1",
		"0.2.1":  "0.2.1",
	}
	for in, want := range cases {
		if got := StripLeadingV(in); got != want {
			t.Errorf("StripLeadingV(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	u := New(Config{})
	if u.owner != defaultOwner {
		t.Errorf("owner = %q, want %q", u.owner, defaultOwner)
	}
	if u.repo != defaultRepo {
		t.Errorf("repo = %q, want %q", u.repo, defaultRepo)
	}
	if u.apiBaseURL != defaultAPIBaseURL {
		t.Errorf("apiBaseURL = %q, want %q", u.apiBaseURL, defaultAPIBaseURL)
	}
	if u.rawBaseURL != defaultRawBaseURL {
		t.Errorf("rawBaseURL = %q, want %q", u.rawBaseURL, defaultRawBaseURL)
	}
	if u.userAgent != defaultUserAgent {
		t.Errorf("userAgent = %q, want %q", u.userAgent, defaultUserAgent)
	}
	if u.httpClient == nil {
		t.Error("httpClient must default to non-nil")
	}
}

func TestNew_TrimsTrailingSlashes(t *testing.T) {
	u := New(Config{
		APIBaseURL: "https://api.example.com/",
		RawBaseURL: "https://raw.example.com///",
	})
	if u.apiBaseURL != "https://api.example.com" {
		t.Errorf("apiBaseURL = %q", u.apiBaseURL)
	}
	if u.rawBaseURL != "https://raw.example.com" {
		t.Errorf("rawBaseURL = %q", u.rawBaseURL)
	}
}
