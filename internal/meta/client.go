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
}

type PageIterator struct {
	client  Client
	path    string
	params  url.Values
	current *Response
	err     error
	hasNext bool
	nextURL string
}

func NewPageIterator(client Client, ctx context.Context, path string, params url.Values) *PageIterator {
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

	var resp *Response
	var err error

	if it.nextURL != "" {
		u, _ := url.Parse(it.nextURL)
		resp, err = it.client.Get(ctx, it.path, u.Query())
	} else {
		resp, err = it.client.Get(ctx, it.path, it.params)
	}

	if err != nil {
		it.err = err
		it.hasNext = false
		return false
	}

	it.current = resp

	var page struct {
		Paging struct {
			Next string `json:"next"`
		} `json:"paging"`
	}
	if json.Unmarshal(resp.Body, &page) == nil && page.Paging.Next != "" {
		it.nextURL = page.Paging.Next
		it.hasNext = true
	} else {
		it.hasNext = false
	}

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
