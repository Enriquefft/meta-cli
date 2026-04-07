package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/enriquefft/meta-cli/internal/updater"
)

// withVersion temporarily swaps cli.Version for the duration of a test.
func withVersion(t *testing.T, v string) {
	t.Helper()
	orig := Version
	Version = v
	t.Cleanup(func() { Version = orig })
}

// withDetectMethod swaps the install-method detector. Used to simulate
// Windows, go-install, and script-installed binaries on any host.
func withDetectMethod(t *testing.T, m updater.InstallMethod) {
	t.Helper()
	orig := updateDetectMethod
	updateDetectMethod = func(string) updater.InstallMethod { return m }
	t.Cleanup(func() { updateDetectMethod = orig })
}

// withExitCapture redirects osExit to a capture variable and returns a
// pointer the caller can inspect after executing the command.
func withExitCapture(t *testing.T) *int {
	t.Helper()
	var code int
	orig := osExit
	osExit = func(c int) { code = c }
	t.Cleanup(func() { osExit = orig })
	return &code
}

// withExecutable swaps updateExecutable so the update command resolves to a
// caller-controlled path instead of the test binary's real location.
func withExecutable(t *testing.T, path string) {
	t.Helper()
	orig := updateExecutable
	updateExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { updateExecutable = orig })
}

// withEvalSymlinks swaps updateEvalSymlinks so tests can force the
// resolved-vs-unresolved path without creating real symlinks on disk.
func withEvalSymlinks(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := updateEvalSymlinks
	updateEvalSymlinks = fn
	t.Cleanup(func() { updateEvalSymlinks = orig })
}

func TestUpdateCommand_Registered(t *testing.T) {
	mc := &mockClient{}
	deps := testDeps(mc)

	cmd := NewUpdateCommand(deps)
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	if cmd.Use != "update" {
		t.Errorf("Use = %q, want %q", cmd.Use, "update")
	}
}

func TestUpdateCommand_Check_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.1"}`))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	stdout, stderr, err := executeCommand(cmd,
		"--check",
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v\nstderr: %s", err, stderr)
	}

	var status updater.Status
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("decode stdout: %v\nstdout: %s", err, stdout)
	}
	if !status.UpToDate {
		t.Errorf("UpToDate = false, want true (stdout: %s)", stdout)
	}
	if status.Current != "0.2.1" {
		t.Errorf("Current = %q", status.Current)
	}
	if status.Latest != "0.2.1" {
		t.Errorf("Latest = %q", status.Latest)
	}
}

func TestUpdateCommand_Check_Outdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0"}`))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	stdout, stderr, err := executeCommand(cmd,
		"--check",
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v\nstderr: %s", err, stderr)
	}

	var status updater.Status
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("decode stdout: %v\nstdout: %s", err, stdout)
	}
	if status.UpToDate {
		t.Errorf("UpToDate = true, want false (stdout: %s)", stdout)
	}
	if status.Current != "0.2.1" {
		t.Errorf("Current = %q", status.Current)
	}
	if status.Latest != "0.3.0" {
		t.Errorf("Latest = %q", status.Latest)
	}
}

func TestUpdateCommand_GoInstall_PrintsHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0"}`))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")
	withDetectMethod(t, updater.MethodGoInstall)

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	stdout, stderr, err := executeCommand(cmd,
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "go install github.com/enriquefft/meta-cli/cmd/meta@latest") {
		t.Errorf("stdout missing go install hint, got: %q", stdout)
	}
	// install.sh must not have been invoked in this path.
	if strings.Contains(stdout, "Updating meta to") {
		t.Errorf("stdout should not contain install.sh progress: %q", stdout)
	}
}

func TestUpdateCommand_Windows_Refuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0"}`))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")
	withDetectMethod(t, updater.MethodWindows)
	exitCode := withExitCapture(t)

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	_, stderr, err := executeCommand(cmd,
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v", err)
	}
	if !strings.Contains(stderr, "not supported on Windows") {
		t.Errorf("stderr should mention Windows refusal, got: %q", stderr)
	}
	if *exitCode != meta.ExitConfigError {
		t.Errorf("exit code = %d, want %d", *exitCode, meta.ExitConfigError)
	}
}

func TestUpdateCommand_CheckWithVersion_Mutex(t *testing.T) {
	// --check and --version express opposing intents (one is "no install,
	// just look", the other is "install this exact tag"). Combining them
	// must produce an immediate parse-time error rather than silently
	// installing the pinned version.
	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	_, _, err := executeCommand(cmd, "--check", "--version", "0.3.0")
	if err == nil {
		t.Fatal("expected error when combining --check and --version, got nil")
	}
	if !strings.Contains(err.Error(), "check") || !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention both flags, got: %v", err)
	}
}

func TestUpdateCommand_Network_Error(t *testing.T) {
	// Spin up a server just so we can capture its URL, then close it so
	// any request against that address fails with a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	withVersion(t, "0.2.1")
	exitCode := withExitCapture(t)

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	_, stderr, err := executeCommand(cmd,
		"--github-api-base-url="+closedURL,
		"--github-raw-base-url="+closedURL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v", err)
	}
	if stderr == "" {
		t.Error("expected error output on stderr")
	}
	if *exitCode != meta.ExitNetworkError {
		t.Errorf("exit code = %d, want %d", *exitCode, meta.ExitNetworkError)
	}
}

// TestUpdateCommand_Script_InvokesInstaller drives the full update flow end
// to end when the install method is MethodScript. The test stubs the
// executable path to a fake binary inside a temp dir, serves an install.sh
// that records its env into a receipt file, and asserts both the pinned URL
// and that META_CLI_INSTALL_DIR matches the resolved install directory.
//
// This is the only test that covers the CLI -> updater.RunInstallScript
// glue; lower-level RunInstallScript semantics are covered in the updater
// package tests.
func TestUpdateCommand_Script_InvokesInstaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is POSIX-only")
	}

	// Fake binary location: <tmp>/fakebin/meta. The command should resolve
	// installDir to <tmp>/fakebin via filepath.Dir.
	fakeDir := filepath.Join(t.TempDir(), "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatalf("mkdir fakedir: %v", err)
	}
	fakeBin := filepath.Join(fakeDir, "meta")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Receipt file captures what install.sh saw at runtime. Using a file
	// rather than a pipe avoids any ordering issues between the child
	// process and the test goroutine.
	receipt := filepath.Join(t.TempDir(), "receipt.txt")

	var observedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Distinguish between the GitHub API call and the install.sh fetch
		// by URL path. The API path is /repos/.../releases/latest; the
		// script path is /<owner>/<repo>/v<ver>/install.sh.
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			_, _ = w.Write([]byte(`{"tag_name":"v0.3.0"}`))
			return
		}
		observedPath = r.URL.Path
		script := fmt.Sprintf(`#!/bin/sh
printf 'DIR=%%s\nVER=%%s\n' "${META_CLI_INSTALL_DIR}" "${META_CLI_VERSION}" > %q
`, receipt)
		_, _ = w.Write([]byte(script))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")
	withDetectMethod(t, updater.MethodScript)
	withExecutable(t, fakeBin)
	// Passthrough EvalSymlinks so the resolved path ends up the same as
	// the fake binary path — on some platforms /tmp is itself a symlink
	// (e.g. macOS /tmp -> /private/tmp), so we normalize deterministically.
	withEvalSymlinks(t, func(p string) (string, error) { return p, nil })

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	stdout, stderr, err := executeCommand(cmd,
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v\nstderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "Updating meta to v0.3.0") {
		t.Errorf("stdout missing progress line, got: %q", stdout)
	}
	if !strings.Contains(stdout, "meta v0.3.0 installed") {
		t.Errorf("stdout missing success line, got: %q", stdout)
	}

	if observedPath != "/enriquefft/meta-cli/v0.3.0/install.sh" {
		t.Errorf("install.sh fetched from %q, want /enriquefft/meta-cli/v0.3.0/install.sh", observedPath)
	}

	data, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "DIR="+fakeDir+"\n") {
		t.Errorf("install.sh saw wrong META_CLI_INSTALL_DIR; receipt=%q fakeDir=%q", got, fakeDir)
	}
	if !strings.Contains(got, "VER=0.3.0\n") {
		t.Errorf("install.sh saw wrong META_CLI_VERSION; receipt=%q", got)
	}
}

func TestUpdateCommand_UpToDate_PrintsMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.1"}`))
	}))
	defer srv.Close()

	withVersion(t, "0.2.1")
	// Make sure the script branch would execute if we weren't short-circuited
	// by the UpToDate check. Any method works; MethodScript is the default.
	withDetectMethod(t, updater.MethodScript)

	mc := &mockClient{}
	deps := testDeps(mc)
	cmd := NewUpdateCommand(deps)

	stdout, stderr, err := executeCommand(cmd,
		"--github-api-base-url="+srv.URL,
		"--github-raw-base-url="+srv.URL,
	)
	if err != nil {
		t.Fatalf("executeCommand: %v\nstderr: %s", err, stderr)
	}
	want := fmt.Sprintf("meta %s is already the latest version", Version)
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout missing up-to-date message, got: %q", stdout)
	}
}
