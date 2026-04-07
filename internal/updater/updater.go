// Package updater handles self-upgrades of the meta-cli binary by delegating
// to the official install.sh script pinned to a specific release tag.
//
// The package deliberately does NOT reimplement any installer logic (platform
// detection, archive download, checksum verification, archive layout). That
// logic lives in install.sh at the repository root and is treated as the
// single source of truth — this package's only job is:
//
//  1. Query the GitHub API for the latest release.
//  2. Compare it against the currently running binary's version.
//  3. Download install.sh pinned to the target tag.
//  4. Execute it with META_CLI_VERSION and META_CLI_INSTALL_DIR set.
//
// Pinning the install.sh URL to the target tag (not main) is a hard
// requirement: the upgrade procedure must be reproducible against a specific
// commit, so a user who upgrades to v0.3.0 gets the install.sh that shipped
// with v0.3.0, not whatever happens to be on main today.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// goosFunc returns runtime.GOOS. Swapped out in tests so we can exercise
// the Windows branch of DetectInstallMethod on non-Windows hosts.
var goosFunc = func() string { return runtime.GOOS }

// readBuildInfo returns debug.ReadBuildInfo. Swapped out in tests so we can
// simulate both "go install" and "goreleaser ldflags" builds deterministically.
var readBuildInfo = debug.ReadBuildInfo

// Release mirrors the subset of the GitHub "latest release" response we care
// about.
type Release struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// Status is the result of comparing the currently installed version to the
// latest published release.
type Status struct {
	Current  string  `json:"current"`
	Latest   string  `json:"latest"`
	UpToDate bool    `json:"up_to_date"`
	Release  Release `json:"release"`
}

// InstallMethod classifies how the running meta-cli binary was installed.
// This is the signal that decides whether `meta update` should re-run
// install.sh, print a `go install` hint, or refuse (Windows).
type InstallMethod int

const (
	// MethodUnknown means classification failed or the binary was built in a
	// way we don't specifically recognize. Treated as MethodScript by the
	// update command because install.sh is the lowest-surprise upgrade path
	// on Linux and macOS.
	MethodUnknown InstallMethod = iota

	// MethodScript is a binary produced by goreleaser and installed either
	// via install.sh or a manual download. These binaries have
	// BuildInfo.Main.Version == "(devel)" because goreleaser builds from a
	// local checkout and stamps the version through -ldflags.
	MethodScript

	// MethodGoInstall is a binary produced by `go install module@version`.
	// These binaries have BuildInfo.Main.Version set to a concrete semver.
	MethodGoInstall

	// MethodWindows indicates the binary is running on Windows. install.sh
	// refuses Windows, so `meta update` cannot delegate to it and must tell
	// the user to download a release manually.
	MethodWindows
)

// String returns the lowercase name used in error messages and JSON output.
func (m InstallMethod) String() string {
	switch m {
	case MethodScript:
		return "script"
	case MethodGoInstall:
		return "go-install"
	case MethodWindows:
		return "windows"
	default:
		return "unknown"
	}
}

// Config configures an Updater. Zero values fall back to production defaults
// in New, so callers only need to set fields they actually want to override
// (the CLI's hidden test flags use this to redirect HTTP calls to httptest).
type Config struct {
	Owner      string
	Repo       string
	APIBaseURL string
	RawBaseURL string
	HTTPClient *http.Client
	UserAgent  string
}

// Updater is the entry point for the self-update flow. All state is immutable
// after New returns, so a single Updater is safe to share across goroutines.
type Updater struct {
	owner      string
	repo       string
	apiBaseURL string
	rawBaseURL string
	httpClient *http.Client
	userAgent  string
}

// defaults used when Config fields are left at their zero value.
const (
	defaultOwner       = "enriquefft"
	defaultRepo        = "meta-cli"
	defaultAPIBaseURL  = "https://api.github.com"
	defaultRawBaseURL  = "https://raw.githubusercontent.com"
	defaultUserAgent   = "meta-cli-updater"
	defaultHTTPTimeout = 30 * time.Second

	// maxReleaseBodyBytes caps the GitHub API response we're willing to read.
	// The real payload is a few KiB; 1 MiB is a comfortable safety margin.
	maxReleaseBodyBytes = 1 << 20

	// maxInstallScriptBytes caps the install.sh download. The real script is
	// ~10 KiB; 1 MiB is a comfortable safety margin.
	maxInstallScriptBytes = 1 << 20
)

// New returns an Updater with the given configuration. Any zero-valued field
// is replaced with the production default so callers can pass Config{} for
// "just do the right thing."
func New(c Config) *Updater {
	u := &Updater{
		owner:      c.Owner,
		repo:       c.Repo,
		apiBaseURL: c.APIBaseURL,
		rawBaseURL: c.RawBaseURL,
		httpClient: c.HTTPClient,
		userAgent:  c.UserAgent,
	}
	if u.owner == "" {
		u.owner = defaultOwner
	}
	if u.repo == "" {
		u.repo = defaultRepo
	}
	if u.apiBaseURL == "" {
		u.apiBaseURL = defaultAPIBaseURL
	}
	if u.rawBaseURL == "" {
		u.rawBaseURL = defaultRawBaseURL
	}
	if u.userAgent == "" {
		u.userAgent = defaultUserAgent
	}
	if u.httpClient == nil {
		u.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	// Trim trailing slashes so URL composition is unambiguous.
	u.apiBaseURL = strings.TrimRight(u.apiBaseURL, "/")
	u.rawBaseURL = strings.TrimRight(u.rawBaseURL, "/")
	return u
}

// Latest fetches the "latest release" metadata for the configured repository.
//
// It uses the documented GitHub endpoint and the recommended
// Accept: application/vnd.github+json media type, with a User-Agent header
// because the GitHub API rejects anonymous requests without one.
func (u *Updater) Latest(ctx context.Context) (Release, error) {
	endpoint, err := url.JoinPath(u.apiBaseURL, "repos", u.owner, u.repo, "releases", "latest")
	if err != nil {
		return Release{}, fmt.Errorf("building release URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, fmt.Errorf("building release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", u.userAgent)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseBodyBytes))
	if err != nil {
		return Release{}, fmt.Errorf("reading release response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, snippet(body))
	}

	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, fmt.Errorf("decoding release response: %w", err)
	}
	if rel.TagName == "" {
		return Release{}, errors.New("github api response missing tag_name")
	}
	return rel, nil
}

// Check fetches the latest release and compares it against currentVersion.
//
// Versions are compared as strings after stripping a single leading 'v' from
// both sides. We intentionally avoid a full semver comparator because the
// only thing that matters for self-update is "is the user running the exact
// tag GitHub currently marks as latest?" — the install.sh script uses the
// same "latest-tag" contract, so matching its semantics keeps the two layers
// consistent. Prereleases and build metadata are handled by whatever the
// GitHub "latest" endpoint returns, which is the same source of truth
// install.sh consults.
func (u *Updater) Check(ctx context.Context, currentVersion string) (Status, error) {
	rel, err := u.Latest(ctx)
	if err != nil {
		return Status{}, err
	}
	current := StripLeadingV(currentVersion)
	latest := StripLeadingV(rel.TagName)
	return Status{
		Current:  current,
		Latest:   latest,
		UpToDate: current == latest,
		Release:  rel,
	}, nil
}

// RunInstallScript downloads the install.sh that shipped with v{version} and
// executes it with META_CLI_VERSION and META_CLI_INSTALL_DIR set, inheriting
// stdin/stdout/stderr so the user sees the script's live progress output.
//
// The URL is deliberately pinned to v{version} (not main): upgrading to
// v0.3.0 must use the install.sh that shipped with v0.3.0, so the upgrade
// procedure is reproducible against a specific commit.
//
// The script is written to a private temp file (mode 0700) rather than piped
// directly into `sh`. This is necessary because RunInstallScript needs to be
// interruptible via ctx, and `exec.CommandContext` can only cancel a child
// it spawned itself.
func (u *Updater) RunInstallScript(ctx context.Context, version, installDir string) error {
	return u.runInstallScript(ctx, version, installDir, os.Stdin, os.Stdout, os.Stderr)
}

// runInstallScript is the test seam for RunInstallScript. It accepts explicit
// stdio so tests can capture the executed script's output without touching
// the real process streams.
func (u *Updater) runInstallScript(ctx context.Context, version, installDir string, stdin io.Reader, stdout, stderr io.Writer) error {
	if version == "" {
		return errors.New("install script: version is required")
	}
	if installDir == "" {
		return errors.New("install script: install directory is required")
	}

	scriptURL, err := url.JoinPath(u.rawBaseURL, u.owner, u.repo, "v"+version, "install.sh")
	if err != nil {
		return fmt.Errorf("building install.sh URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		return fmt.Errorf("building install.sh request: %w", err)
	}
	req.Header.Set("User-Agent", u.userAgent)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading install.sh: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInstallScriptBytes))
	if err != nil {
		return fmt.Errorf("reading install.sh: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("downloading install.sh: status %d: %s", resp.StatusCode, snippet(body))
	}

	tmp, err := os.CreateTemp("", "meta-install-*.sh")
	if err != nil {
		return fmt.Errorf("creating install.sh temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// Restrict permissions before writing so there is no race between write
	// and chmod in which a world-readable file exists on disk.
	if err := os.Chmod(tmpPath, 0o700); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod install.sh temp file: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing install.sh temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing install.sh temp file: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", tmpPath)
	cmd.Env = append(os.Environ(),
		"META_CLI_INSTALL_DIR="+installDir,
		"META_CLI_VERSION="+version,
	)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running install.sh: %w", err)
	}
	return nil
}

// DetectInstallMethod classifies how the running binary was installed so the
// update command can pick the right upgrade path.
//
// Logic:
//
//  1. Windows is unconditionally MethodWindows because install.sh refuses it.
//  2. If runtime/debug reports a concrete module version on Main (anything
//     other than "" or "(devel)"), the binary was built by
//     `go install module@version`, which stamps the module version into
//     build info. Return MethodGoInstall so the command prints the
//     `go install ...@latest` hint.
//  3. Otherwise return MethodScript. This covers goreleaser-built binaries,
//     which leave Main.Version as "(devel)" because goreleaser builds from
//     a local checkout and injects the version via -ldflags.
//
// execPath is reserved for future heuristics (for example, comparing against
// $GOBIN to distinguish `go install` more reliably on exotic setups). It is
// kept in the signature now so the update command doesn't need to change
// when we extend detection later.
func DetectInstallMethod(execPath string) InstallMethod {
	_ = execPath
	if goosFunc() == "windows" {
		return MethodWindows
	}
	info, ok := readBuildInfo()
	if !ok {
		return MethodScript
	}
	v := info.Main.Version
	if v != "" && v != "(devel)" {
		return MethodGoInstall
	}
	return MethodScript
}

// StripLeadingV removes a single leading 'v' or 'V' so "v0.2.1" and "0.2.1"
// compare equal. This is the single source of truth for version normalization;
// CLI layers should call this rather than reimplementing it.
func StripLeadingV(v string) string {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}

// snippet returns a short prefix of body for inclusion in error messages,
// collapsing whitespace so the output stays on a single line.
func snippet(body []byte) string {
	const maxLen = 200
	s := strings.TrimSpace(string(body))
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
