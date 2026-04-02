package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Response struct {
	Body       []byte
	StatusCode int
	Headers    http.Header
}

type Client interface {
	Get(ctx context.Context, path string, params url.Values) (*Response, error)
	Post(ctx context.Context, path string, params map[string]string) (*Response, error)
	Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*Response, error)
	Paginate(ctx context.Context, path string, params url.Values) *PageIterator
	SetDryRun(v bool)
	SetVerbose(v bool)
}

type PageIterator struct {
	client  Client
	path    string
	params  url.Values
	current *Response
	err     error
	hasNext bool
}

func NewPageIterator(client Client, path string, params url.Values) *PageIterator {
	if params == nil {
		params = url.Values{}
	}
	return &PageIterator{
		client:  client,
		path:    path,
		params:  params,
		hasNext: true,
	}
}

func (it *PageIterator) Next(ctx context.Context) bool {
	if !it.hasNext {
		return false
	}

	resp, err := it.client.Get(ctx, it.path, it.params)
	if err != nil {
		it.err = err
		it.hasNext = false
		return false
	}

	it.current = resp

	var page struct {
		Paging struct {
			Cursors struct {
				Before string `json:"before"`
				After  string `json:"after"`
			} `json:"cursors"`
			Next string `json:"next"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(resp.Body, &page); err != nil {
		it.err = fmt.Errorf("parsing pagination metadata: %w", err)
		it.hasNext = false
		return true
	}
	if page.Paging.Cursors.After == "" {
		it.hasNext = false
		return true
	}

	if page.Paging.Next == "" {
		it.hasNext = false
		return true
	}

	nextParams := make(url.Values, len(it.params))
	for k, v := range it.params {
		if k != "after" {
			nextParams[k] = v
		}
	}
	nextParams.Set("after", page.Paging.Cursors.After)
	it.params = nextParams
	it.hasNext = true

	return true
}

func (it *PageIterator) Page() (*Response, error) {
	if it.current == nil {
		return nil, fmt.Errorf("no page available: call Next() first")
	}
	return it.current, it.err
}

func (it *PageIterator) Err() error {
	return it.err
}
