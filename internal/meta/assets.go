package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// videoFields is the canonical set of fields requested from the Graph API
// for a video resource. Both UploadVideo and GetVideoStatus use it so that
// the two flows return identical shapes. The picture field is included so
// ensureVideoThumbnail can reach the auto-generated thumbnail URL without
// a second round-trip.
const videoFields = "id,title,status,length,picture"

// UploadVideoParams contains the parameters for uploading a video to an ad account.
type UploadVideoParams struct {
	AccountID string
	File      io.Reader
	Filename  string
	FileSize  int64
	Title     string
}

// VideoStatusParams contains the parameters for checking video encoding status.
type VideoStatusParams struct {
	VideoID string
}

// Video is the canonical representation of a Meta video resource, shared by
// UploadVideo and GetVideoStatus. It is the single source of truth for the
// shape of a video returned to callers.
//
// Picture is Meta's auto-generated thumbnail URL, served from the FB CDN.
// It is populated once the video has finished encoding and is the source
// used by ensureVideoThumbnail to avoid forcing users to manually supply a
// thumbnail image when creating video creatives.
type Video struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Status  VideoStatus `json:"status"`
	Length  float64     `json:"length"`
	Picture string      `json:"picture,omitempty"`
}

// VideoStatus represents the encoding status of a video.
type VideoStatus struct {
	VideoStatus        string `json:"video_status"`
	ProcessingProgress int    `json:"processing_progress"`
}

// normalizeAccountID strips an optional "act_" prefix and returns the numeric ID.
func normalizeAccountID(id string) string {
	return strings.TrimPrefix(id, "act_")
}

// UploadVideo uploads a video file to the specified ad account and returns
// the full video record (id, title, status, length). Because Meta's upload
// endpoint only returns the new video's id, this function issues a follow-up
// GET against the same path used by GetVideoStatus so callers always receive
// a fully populated Video regardless of which code path produced it.
func UploadVideo(ctx context.Context, client Client, params UploadVideoParams) (*Video, error) {
	if params.AccountID == "" {
		return nil, fmt.Errorf("validation: AccountID is required")
	}
	if params.File == nil {
		return nil, fmt.Errorf("validation: File is required")
	}
	if params.Filename == "" {
		return nil, fmt.Errorf("validation: Filename is required")
	}

	accountID := normalizeAccountID(params.AccountID)
	path := fmt.Sprintf("/act_%s/advideos", accountID)

	uploadParams := make(map[string]string)
	if params.Title != "" {
		uploadParams["title"] = params.Title
	}

	resp, err := client.Upload(ctx, path, params.File, params.Filename, params.FileSize, uploadParams)
	if err != nil {
		return nil, err
	}

	// Meta's POST /act_*/advideos endpoint (and the resumable finish phase)
	// only returns the new video's id. Chain a follow-up GET via the shared
	// fetchAfterCreate helper so callers always receive a fully populated
	// Video record regardless of which upload code path produced it.
	var video Video
	if err := fetchAfterCreate(ctx, client, resp.Body, "video", videoFields, &video); err != nil {
		return nil, err
	}
	return &video, nil
}

// UploadImageParams contains parameters for uploading an image to an ad account.
type UploadImageParams struct {
	AccountID string
	File      io.Reader
	Filename  string
	FileSize  int64
}

// Image is the canonical representation of a Meta ad image. Single source of
// truth for the shape returned by both UploadImage and any future GetImage call.
//
// The Hash field is the canonical identifier consumed by ad creatives as
// image_hash (including as the thumbnail hash for video creatives).
type Image struct {
	Hash   string `json:"hash"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	URL128 string `json:"url_128,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// uploadImageResponse is the envelope Meta returns from POST /act_{id}/adimages.
// The response is a map keyed by the uploaded filename (minus extension in some
// cases); it always contains exactly one entry per single-file upload.
//
// Meta documentation and observed response shape:
//
//	{
//	  "images": {
//	    "myphoto.jpg": {
//	      "hash": "abc123...",
//	      "url": "https://scontent...",
//	      "url_128": "https://scontent...",
//	      "width": 1200,
//	      "height": 628,
//	      "name": "myphoto.jpg"
//	    }
//	  }
//	}
type uploadImageResponse struct {
	Images map[string]Image `json:"images"`
}

// UploadImage uploads an image file to the specified ad account and returns
// the resulting Image record (including the hash, which is the canonical
// identifier consumed by creatives as image_hash).
//
// Meta endpoint: POST /act_{account_id}/adimages
// The multipart upload is delegated to the shared Client.Upload primitive.
// The response is a map keyed by filename; this function looks up the entry
// matching the upload's filename and falls back to the first (and only) entry
// when Meta rewrites the key (e.g. stripping the file extension).
func UploadImage(ctx context.Context, client Client, params UploadImageParams) (*Image, error) {
	if params.AccountID == "" {
		return nil, fmt.Errorf("validation: AccountID is required")
	}
	if params.File == nil {
		return nil, fmt.Errorf("validation: File is required")
	}
	if params.Filename == "" {
		return nil, fmt.Errorf("validation: Filename is required")
	}
	if params.FileSize <= 0 {
		return nil, fmt.Errorf("validation: FileSize must be greater than zero")
	}

	accountID := normalizeAccountID(params.AccountID)
	path := fmt.Sprintf("/act_%s/adimages", accountID)

	resp, err := client.Upload(ctx, path, params.File, params.Filename, params.FileSize, nil)
	if err != nil {
		return nil, err
	}

	var decoded uploadImageResponse
	if err := json.Unmarshal(resp.Body, &decoded); err != nil {
		return nil, fmt.Errorf("parsing upload response: %w", err)
	}
	if len(decoded.Images) == 0 {
		return nil, fmt.Errorf("upload response contained no images")
	}

	img, ok := decoded.Images[params.Filename]
	if !ok {
		// Meta may rewrite the key (e.g. strip the extension); there is always
		// exactly one entry for a single-file upload, so fall back to it and
		// report which keys were present if we somehow end up with zero.
		for _, v := range decoded.Images {
			img = v
			ok = true
			break
		}
		if !ok {
			keys := make([]string, 0, len(decoded.Images))
			for k := range decoded.Images {
				keys = append(keys, k)
			}
			return nil, fmt.Errorf("upload response missing image entry for %q (got keys: %v)", params.Filename, keys)
		}
	}

	if img.Name == "" {
		img.Name = params.Filename
	}

	return &img, nil
}

// httpDoer is the minimal subset of *http.Client used by ensureVideoThumbnail
// to download Meta's CDN-hosted video thumbnails. It exists so tests can
// inject a fake response without hitting the real network; production code
// always uses the package-level thumbnailHTTPClient below.
//
// This is intentionally a private interface: "download an arbitrary URL" is
// not a Graph API primitive and must not be promoted to the Client interface
// (which exclusively models Graph calls). Keeping the abstraction local to
// this file preserves the deep-module boundary of internal/graph.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// thumbnailHTTPClient is the HTTP transport used by ensureVideoThumbnail to
// download video thumbnail bytes from Meta's CDN. It is a package-level var
// purely so tests can temporarily swap it via setThumbnailHTTPClient; no
// production code path should read or modify it, and callers of
// ensureVideoThumbnail treat it as an implementation detail.
var thumbnailHTTPClient httpDoer = http.DefaultClient

// setThumbnailHTTPClient swaps the HTTP transport used by ensureVideoThumbnail
// and returns a restore function. It is package-private and intended
// exclusively for tests — never call it from production code.
func setThumbnailHTTPClient(c httpDoer) func() {
	prev := thumbnailHTTPClient
	thumbnailHTTPClient = c
	return func() { thumbnailHTTPClient = prev }
}

// ensureVideoThumbnail is the single source of truth for "given a video id,
// give me an image_hash my creative's video_data block can use". Meta's
// adcreatives endpoint rejects video creatives that do not specify an
// image_hash or image_url inside video_data (error subcode 1443226), even
// though every uploaded video already has an auto-generated thumbnail at
// Video.Picture. Users should never have to download, re-upload, and pass
// that hash by hand — this helper automates the full dance so CreateCreative
// (and any future code path that needs a thumbnail hash) can stay a
// one-liner.
//
// Behavior:
//
//   - If imageHash is non-empty it is returned as-is, no HTTP work is done.
//     This is the hot path when the user explicitly provided one and allows
//     callers to unconditionally route through this helper.
//   - Otherwise accountID and videoID are validated, the video record is
//     fetched via GetVideoStatus to obtain Video.Picture, the picture bytes
//     are downloaded over plain HTTP from the (signed, short-lived) FB CDN
//     URL, and UploadImage is invoked against the same ad account so the
//     resulting Image.Hash is suitable for use as a creative thumbnail hash.
//
// Errors are wrapped with context at every step rather than swallowed, and
// a video that has not finished encoding (empty Picture) produces a clear,
// actionable error rather than a later Meta-side failure.
func ensureVideoThumbnail(ctx context.Context, client Client, accountID, videoID, imageHash string) (string, error) {
	if imageHash != "" {
		return imageHash, nil
	}
	if accountID == "" {
		return "", fmt.Errorf("validation: AccountID is required")
	}
	if videoID == "" {
		return "", fmt.Errorf("validation: VideoID is required")
	}

	video, err := GetVideoStatus(ctx, client, VideoStatusParams{VideoID: videoID})
	if err != nil {
		return "", fmt.Errorf("fetching video thumbnail: %w", err)
	}
	if video.Picture == "" {
		return "", fmt.Errorf(
			"video %s has no thumbnail picture yet; wait for encoding to finish "+
				"or provide --image-hash / --image-url explicitly",
			videoID,
		)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, video.Picture, nil)
	if err != nil {
		return "", fmt.Errorf("building video thumbnail request: %w", err)
	}
	resp, err := thumbnailHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading video thumbnail: %w", err)
	}
	defer func() {
		// A failure closing the CDN response body has no meaningful recovery
		// path: we have already decided whether to return the thumbnail hash
		// or an error. Logging is not available here without violating the
		// deep-module boundary; dropping the close error is the correct
		// trade-off for a one-shot CDN GET.
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"downloading video thumbnail: unexpected status %d from %s",
			resp.StatusCode, video.Picture,
		)
	}

	pictureBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading video thumbnail body: %w", err)
	}
	if len(pictureBytes) == 0 {
		return "", fmt.Errorf("downloading video thumbnail: empty response body from %s", video.Picture)
	}

	// The /picture endpoint on Meta's CDN always serves JPEG, so a fixed
	// extension is correct and avoids speculative content sniffing.
	filename := fmt.Sprintf("thumb_%s.jpg", videoID)
	image, err := UploadImage(ctx, client, UploadImageParams{
		AccountID: accountID,
		File:      bytes.NewReader(pictureBytes),
		Filename:  filename,
		FileSize:  int64(len(pictureBytes)),
	})
	if err != nil {
		return "", fmt.Errorf("uploading video thumbnail: %w", err)
	}
	if image.Hash == "" {
		return "", fmt.Errorf("uploading video thumbnail: upload response missing hash")
	}
	return image.Hash, nil
}

// GetVideoStatus fetches the current metadata and encoding status of a video.
func GetVideoStatus(ctx context.Context, client Client, params VideoStatusParams) (*Video, error) {
	if params.VideoID == "" {
		return nil, fmt.Errorf("validation: VideoID is required")
	}

	path := fmt.Sprintf("/%s", params.VideoID)
	queryParams := url.Values{}
	queryParams.Set("fields", videoFields)

	resp, err := client.Get(ctx, path, queryParams)
	if err != nil {
		return nil, err
	}

	var result Video
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parsing video response: %w", err)
	}

	return &result, nil
}
