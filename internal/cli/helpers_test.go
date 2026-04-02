package cli

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/enriquefft/meta-cli/internal/config"
	"github.com/enriquefft/meta-cli/internal/meta"
	"github.com/spf13/cobra"
)

// mockClient implements meta.Client for CLI tests.
type mockClient struct {
	mu       sync.Mutex
	getFn    func(ctx context.Context, path string, params url.Values) (*meta.Response, error)
	postFn   func(ctx context.Context, path string, params map[string]string) (*meta.Response, error)
	uploadFn func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error)
	pagFn    func(ctx context.Context, path string, params url.Values) *meta.PageIterator
	dryRun   bool
	verbose  bool
}

var _ meta.Client = (*mockClient)(nil)

func (m *mockClient) Get(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
	m.mu.Lock()
	fn := m.getFn
	m.mu.Unlock()
	if fn == nil {
		panic("mockClient.Get not set")
	}
	return fn(ctx, path, params)
}

func (m *mockClient) Post(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
	m.mu.Lock()
	fn := m.postFn
	m.mu.Unlock()
	if fn == nil {
		panic("mockClient.Post not set")
	}
	return fn(ctx, path, params)
}

func (m *mockClient) Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
	m.mu.Lock()
	fn := m.uploadFn
	m.mu.Unlock()
	if fn == nil {
		panic("mockClient.Upload not set")
	}
	return fn(ctx, path, file, filename, size, params)
}

func (m *mockClient) Paginate(ctx context.Context, path string, params url.Values) *meta.PageIterator {
	m.mu.Lock()
	fn := m.pagFn
	m.mu.Unlock()
	if fn == nil {
		panic("mockClient.Paginate not set")
	}
	return fn(ctx, path, params)
}

func (m *mockClient) SetDryRun(v bool)  { m.dryRun = v }
func (m *mockClient) SetVerbose(v bool) { m.verbose = v }

// testDeps creates a Dependencies with a mock client and default config.
func testDeps(mc *mockClient) *Dependencies {
	return &Dependencies{
		Client: mc,
		Config: &config.Config{
			DefaultAccount: "act_123456",
			AccessToken:    "test-token",
			APIVersion:     "v21.0",
			OutputFormat:   "json",
		},
		Format: "json",
	}
}

// testStore creates a temporary ConfigStore for testing.
func testStore() (*config.ConfigStore, func()) {
	dir, err := os.MkdirTemp("", "meta-cli-test-*")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "config.yaml")
	store := config.NewStore(path)
	cleanup := func() { _ = os.RemoveAll(dir) }
	return store, cleanup
}

// executeCommand runs a cobra command with the given arguments and captures output.
// Returns stdout, stderr, and any error.
func executeCommand(root *cobra.Command, args ...string) (stdout string, stderr string, err error) {
	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)
	root.SetOut(bufOut)
	root.SetErr(bufErr)
	root.SetArgs(args)

	err = root.Execute()

	return bufOut.String(), bufErr.String(), err
}

// executeCommandWithContext runs a cobra command with context and captures output.
func executeCommandWithContext(ctx context.Context, root *cobra.Command, args ...string) (stdout string, stderr string, err error) {
	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)
	root.SetOut(bufOut)
	root.SetErr(bufErr)
	root.SetArgs(args)

	err = root.ExecuteContext(ctx)

	return bufOut.String(), bufErr.String(), err
}
