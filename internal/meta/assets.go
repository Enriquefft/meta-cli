package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// videoFields is the canonical set of fields requested from the Graph API
// for a video resource. Both UploadVideo and GetVideoStatus use it so that
// the two flows return identical shapes.
const videoFields = "id,title,status,length"

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
type Video struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Status VideoStatus `json:"status"`
	Length float64     `json:"length"`
}

// VideoStatus represents the encoding status of a video.
type VideoStatus struct {
	VideoStatus        string `json:"video_status"`
	ProcessingProgress int    `json:"processing_progress"`
}

// uploadResponse is the thin envelope Meta returns from the POST /advideos
// endpoint (and from the resumable finish phase). It only carries the new
// video's ID; full metadata must be fetched separately.
type uploadResponse struct {
	ID string `json:"id"`
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

	var created uploadResponse
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		return nil, fmt.Errorf("parsing upload response: %w", err)
	}
	if created.ID == "" {
		return nil, fmt.Errorf("upload response missing video id")
	}

	return GetVideoStatus(ctx, client, VideoStatusParams{VideoID: created.ID})
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
