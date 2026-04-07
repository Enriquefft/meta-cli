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
