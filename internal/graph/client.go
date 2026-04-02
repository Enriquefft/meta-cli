package graph

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	meta "github.com/enriquefft/meta-cli/internal/meta"
)

const maxResponseSize = 50 << 20

type ClientConfig struct {
	AccessToken string
	APIVersion  string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	dryRun     bool
	verbose    bool
	retry      RetryConfig

	rateMu    sync.Mutex
	rateLimit *RateLimit
}

func NewClient(cfg ClientConfig) *Client {
	baseURL := fmt.Sprintf("https://graph.facebook.com/%s", cfg.APIVersion)
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    baseURL,
		token:      cfg.AccessToken,
		retry:      DefaultRetryConfig(),
	}
}

func (c *Client) SetDryRun(v bool)  { c.dryRun = v }
func (c *Client) SetVerbose(v bool) { c.verbose = v }

func (c *Client) preRequestBackoff(ctx context.Context) {
	c.rateMu.Lock()
	rl := c.rateLimit
	c.rateMu.Unlock()

	if rl == nil || rl.Usage < 75 {
		return
	}

	backoff := BackoffDuration(rl.Usage)
	if backoff <= 0 {
		return
	}

	if c.verbose {
		slog.Info("rate limit backoff", "usage", rl.Usage, "backoff", backoff)
	}

	select {
	case <-ctx.Done():
	case <-time.After(backoff):
	}
}

func (c *Client) updateRateLimit(header http.Header) {
	if rl := ParseRateLimit(header); rl != nil {
		c.rateMu.Lock()
		c.rateLimit = rl
		c.rateMu.Unlock()
	}
}

func (c *Client) Get(ctx context.Context, path string, params url.Values) (*meta.Response, error) {
	c.preRequestBackoff(ctx)

	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", c.token)

	u := c.baseURL + path + "?" + params.Encode()

	if c.verbose {
		slog.Info("GET", "url", redactURL(u))
	}

	var resp *http.Response
	var body []byte
	var err error

	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retry.DelayForAttempt(attempt - 1)
			if c.verbose {
				slog.Info("retrying", "attempt", attempt, "delay", delay)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetError(err) {
				continue
			}
			return nil, fmt.Errorf("executing request: %w", err)
		}

		body, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		c.updateRateLimit(resp.Header)

		if !IsRetryableHTTP(resp.StatusCode) {
			break
		}
	}

	if c.verbose {
		slog.Info("response", "status", resp.StatusCode, "body", string(body))
	}

	result := &meta.Response{
		Body:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}

	if err := CheckResponse(result); err != nil {
		return result, err
	}

	return result, nil
}

func (c *Client) Post(ctx context.Context, path string, params map[string]string) (*meta.Response, error) {
	c.preRequestBackoff(ctx)

	p := make(map[string]string, len(params)+2)
	for k, v := range params {
		p[k] = v
	}
	p["access_token"] = c.token

	if c.dryRun {
		p["execution_options"] = `["validate_only"]`
	}

	form := url.Values{}
	for k, v := range p {
		form.Set(k, v)
	}

	u := c.baseURL + path

	if c.verbose {
		slog.Info("POST", "url", u, "params", redactParams(form.Encode()))
	}

	var resp *http.Response
	var body []byte
	var err error

	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retry.DelayForAttempt(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetError(err) {
				continue
			}
			return nil, fmt.Errorf("executing request: %w", err)
		}

		body, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		c.updateRateLimit(resp.Header)

		if !IsRetryableHTTP(resp.StatusCode) {
			break
		}
	}

	if c.verbose {
		slog.Info("response", "status", resp.StatusCode, "body", string(body))
	}

	result := &meta.Response{
		Body:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}

	if err := CheckResponse(result); err != nil {
		return result, err
	}

	return result, nil
}

func (c *Client) Paginate(ctx context.Context, path string, params url.Values) *meta.PageIterator {
	return meta.NewPageIterator(c, ctx, path, params)
}

func redactURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	q := parsed.Query()
	if q.Get("access_token") != "" {
		q.Set("access_token", "REDACTED")
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}

func redactParams(encoded string) string {
	vals, err := url.ParseQuery(encoded)
	if err != nil {
		return encoded
	}
	if vals.Get("access_token") != "" {
		vals.Set("access_token", "REDACTED")
	}
	return vals.Encode()
}

func isRetryableNetError(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		err = urlErr.Err
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return !dnsErr.IsNotFound
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return false
}
