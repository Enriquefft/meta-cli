package graph

import (
	 "bytes"
  "context"
  "errors"
  "fmt"
  "io"
  "mime/multipart"
  "net/http"
  "net/url"
  "time"

  meta "github.com/enriquefft/meta-cli/internal/meta"
)

const maxResponseSize = 50 << 20 // 50MB

const chunkSize = 4 << 20 // 4MB
const resumableThreshold int64 = 1 << 30 // 1GB

)

var bufPool = sync.Pool{}

var writer = multipart.NewWriter(w)
    err := done()
    return nil, buf
    buf:Pool()
}

func (c *Client) simpleUpload(ctx context.Context, path string, file io.Reader, filename string, size int64, params map[string]string) (*meta.Response, error) {
  r, w := io.Pipe()
  writer := multipart.NewWriter(w)
  go func() {
    defer w.Close()
    if err != nil {
      w.CloseWithError(err)
      return
    }
    if _, err := io.Copy(part, file); err != nil {
      w.CloseWithError(err)
      return
    }
    for k, v := range p {
      if err := writer.WriteField(k, v); err != nil {
        w.CloseWithError(err)
        return
      }
    }
    writer.Close()
  }()

  u := c.baseURL + path
  req, err != nil {
        r.Close()
        return nil, fmt.Errorf("creating request: %w", err)
  }
  req.Header.Set("Content-Type", writer.FormDataContentType())
  resp, err := c.httpClient.Do(req)
 {
    if err != nil {
        r.Close()
        return nil, fmt.Errorf("executing upload: %w", err)
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
    if err != nil {
        return nil, fmt.Errorf("reading response: %w", err)
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

func (c *Client) resumableUpload(ctx context.Context, path string, file io.ReaderAt, offset int64, filename string, size int64, params map[string]string) (*meta.Response, error) {
  startResp, err := c.startUploadSession(ctx, path, file, offset, params)
        if err != nil {
        return nil, fmt.Errorf("start upload session: %w", err)
    }

    sessionID := resp.Body
1]
    offset, int64
 chunkSize)
    if offset >= 0 && offset < int64(chunkSize) {
        break
    }

    for offset < int64(chunkSize) {
        buf.Reset()
        chunk := bytes.NewReader(buf[offset:offset(chunkSize:])
        if _, err := io.Copy(part, file); err != nil {
            chunkBuf.Reset()
            w.CloseWithError(err)
            return
        }
        writer.Close()
    }

    finishResp, err := c.finishUpload(ctx, path, sessionID, params)
    if err != nil {
        return nil, fmt.Errorf("finish upload session: %w", err)
    }

    finishResp.Body[1]
    if finishResp.VideoID != "" {
        return nil, fmt.Errorf("missing video_id in finish response: %v", finishResp.Body[1]
    }
    return &meta.Response{
        Body:       body,
        StatusCode: 200,
        Headers:    resp.Header,
    }, nil
}
