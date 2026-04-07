package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"

	meta "github.com/enriquefft/meta-cli/internal/meta"
)

const resumableThreshold int64 = 1 << 30
const chunkSize = 4 << 20
const maxUploadIterations = 10_000

func (c *Client) Upload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
	p := make(map[string]string, len(params)+1)
	for k, v := range params {
		p[k] = v
	}
	p["access_token"] = c.token

	if size >= resumableThreshold {
		if ra, ok := file.(io.ReaderAt); ok {
			return c.resumableUpload(ctx, path, ra, filename, size, p)
		}
	}

	return c.simpleUpload(ctx, path, file, filename, p)
}

func (c *Client) simpleUpload(ctx context.Context, path string, file io.Reader, filename string, params map[string]string) (*meta.Response, error) {
	c.preRequestBackoff(ctx)

	r, w := io.Pipe()
	mpW := multipart.NewWriter(w)

	pipeErrCh := make(chan error, 1)
	go func() {
		defer func() { _ = w.Close() }()
		part, err := mpW.CreateFormFile("source", filename)
		if err != nil {
			pipeErrCh <- err
			_ = w.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			pipeErrCh <- err
			_ = w.CloseWithError(err)
			return
		}
		for k, v := range params {
			if err := mpW.WriteField(k, v); err != nil {
				pipeErrCh <- err
				_ = w.CloseWithError(err)
				return
			}
		}
		pipeErrCh <- mpW.Close()
	}()

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("creating upload request: %w", err)
	}
	req.Header.Set("Content-Type", mpW.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if pipeErr := <-pipeErrCh; pipeErr != nil {
			return nil, pipeErr
		}
		return nil, fmt.Errorf("executing upload: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading upload response: %w", err)
	}

	c.updateRateLimit(resp.Header)

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

type uploadSessionResponse struct {
	VideoUploadSessionID string `json:"video_upload_session_id"`
	VideoID              string `json:"video_id"`
	StartOffset          string `json:"start_offset"`
	EndOffset            string `json:"end_offset"`
}

type finishUploadResponse struct {
	Success bool   `json:"success"`
	VideoID string `json:"video_id"`
}

func (c *Client) startUploadSession(ctx context.Context, path string, params map[string]string) (*uploadSessionResponse, error) {
	c.preRequestBackoff(ctx)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("upload_phase", "start")
	encoded := form.Encode()

	resp, body, err := c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("start session failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var session uploadSessionResponse
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("parsing start session response: %w", err)
	}

	return &session, nil
}

var chunkBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, chunkSize)
		return &buf
	},
}

func (c *Client) transferChunk(ctx context.Context, path string, sessionID string, file io.ReaderAt, offset int64, accessToken string) (string, error) {
	c.preRequestBackoff(ctx)

	bufPtr := chunkBufPool.Get().(*[]byte)
	defer chunkBufPool.Put(bufPtr)
	buf := *bufPtr

	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("reading chunk at offset %d: %w", offset, err)
	}
	if n == 0 {
		return "", nil
	}

	r, w := io.Pipe()
	mpW := multipart.NewWriter(w)

	go func() {
		defer func() { _ = w.Close() }()
		part, err := mpW.CreateFormFile("video_file_chunk", "chunk")
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if _, err := part.Write(buf[:n]); err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if err := mpW.WriteField("access_token", accessToken); err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if err := mpW.WriteField("upload_phase", "transfer"); err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if err := mpW.WriteField("upload_session_id", sessionID); err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if err := mpW.WriteField("start_offset", fmt.Sprintf("%d", offset)); err != nil {
			_ = w.CloseWithError(err)
			return
		}
		_ = mpW.Close()
	}()

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		_ = r.Close()
		return "", fmt.Errorf("creating transfer request: %w", err)
	}
	req.Header.Set("Content-Type", mpW.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing transfer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("reading transfer response: %w", err)
	}

	c.updateRateLimit(resp.Header)

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("transfer failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var result struct {
		StartOffset string `json:"start_offset"`
		EndOffset   string `json:"end_offset"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing transfer response: %w", err)
	}

	return result.StartOffset, nil
}

func (c *Client) finishUpload(ctx context.Context, path string, sessionID string, params map[string]string) (*finishUploadResponse, error) {
	c.preRequestBackoff(ctx)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("upload_phase", "finish")
	form.Set("upload_session_id", sessionID)
	encoded := form.Encode()

	resp, body, err := c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("finish upload: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("finish failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var fin finishUploadResponse
	if err := json.Unmarshal(body, &fin); err != nil {
		return nil, fmt.Errorf("parsing finish response: %w", err)
	}

	return &fin, nil
}

func (c *Client) resumableUpload(ctx context.Context, path string, file io.ReaderAt, filename string, size int64, params map[string]string) (*meta.Response, error) {
	params["file_size"] = fmt.Sprintf("%d", size)
	params["file_name"] = filename

	session, err := c.startUploadSession(ctx, path, params)
	if err != nil {
		return nil, err
	}

	var offset int64
	for i := 0; offset < size; i++ {
		if i >= maxUploadIterations {
			return nil, fmt.Errorf("upload exceeded maximum iterations (%d) at offset %d/%d: possible server-side stall", maxUploadIterations, offset, size)
		}

		nextOffset, err := c.transferChunk(ctx, path, session.VideoUploadSessionID, file, offset, params["access_token"])
		if err != nil {
			return nil, fmt.Errorf("transferring chunk at offset %d: %w", offset, err)
		}

		var newOffset int64
		if nextOffset == "" {
			newOffset = offset + int64(chunkSize)
		} else {
			var parsed int64
			if _, err := fmt.Sscanf(nextOffset, "%d", &parsed); err != nil {
				return nil, fmt.Errorf("invalid start_offset %q from API: %w", nextOffset, err)
			}
			if parsed <= offset {
				newOffset = offset + int64(chunkSize)
			} else {
				newOffset = parsed
			}
		}

		if newOffset <= offset {
			return nil, fmt.Errorf("upload stalled: offset did not advance at %d (API returned non-progressing offset)", offset)
		}
		if newOffset > size+int64(chunkSize) {
			return nil, fmt.Errorf("upload offset %d exceeds file size %d by more than one chunk", newOffset, size)
		}
		offset = newOffset
	}

	fin, err := c.finishUpload(ctx, path, session.VideoUploadSessionID, params)
	if err != nil {
		return nil, err
	}

	// Normalize the resumable finish envelope (which uses video_id) to the
	// same {"id": "..."} shape returned by the simple /advideos upload so the
	// meta layer can decode either response uniformly.
	resultBody, err := json.Marshal(map[string]string{"id": fin.VideoID})
	if err != nil {
		return nil, fmt.Errorf("encoding resumable upload result: %w", err)
	}

	return &meta.Response{
		Body:       resultBody,
		StatusCode: 200,
	}, nil
}
