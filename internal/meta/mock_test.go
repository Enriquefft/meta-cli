package meta

import (
	"context"
	"io"
	"net/url"
)

type MockClient struct {
	GetFn      func(ctx context.Context, path string, params url.Values) (*Response, error)
	PostFn     func(ctx context.Context, path string, params map[string]string) (*Response, error)
	UploadFn   func(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error)
	PaginateFn func(ctx context.Context, path string, params url.Values) *PageIterator
	DryRun     bool
	Verbose    bool
}

var _ Client = (*MockClient)(nil)

func (m *MockClient) Get(ctx context.Context, path string, params url.Values) (*Response, error) {
	if m.GetFn == nil {
		panic("MockClient.Get not implemented")
	}
	return m.GetFn(ctx, path, params)
}

func (m *MockClient) Post(ctx context.Context, path string, params map[string]string) (*Response, error) {
	if m.PostFn == nil {
		panic("MockClient.Post not implemented")
	}
	return m.PostFn(ctx, path, params)
}

func (m *MockClient) Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error) {
	if m.UploadFn == nil {
		panic("MockClient.Upload not implemented")
	}
	return m.UploadFn(ctx, path, file, filename, size, params)
}

func (m *MockClient) Paginate(ctx context.Context, path string, params url.Values) *PageIterator {
	if m.PaginateFn == nil {
		panic("MockClient.Paginate not implemented")
	}
	return m.PaginateFn(ctx, path, params)
}

func (m *MockClient) SetDryRun(v bool)  { m.DryRun = v }
func (m *MockClient) SetVerbose(v bool) { m.Verbose = v }
