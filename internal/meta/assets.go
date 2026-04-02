package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// UploadVideoParams contains the parameters for uploading a video to an ad account.
type UploadVideoParams struct {
	AccountID string
	File      io.Reader
	Filename  string
	FileSize  int64
	Title     string
}

// UploadVideoResult contains the response from a video upload operation.
type UploadVideoResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	UploadStatus string `json:"upload_status"`
}

// VideoStatusParams contains the parameters for checking video encoding status.
type VideoStatusParams struct {
	VideoID string
}

// VideoStatusResult contains the response from a video status check.
type VideoStatusResult struct {
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

// normalizeAccountID strips an optional "act_" prefix and returns the numeric ID.
func normalizeAccountID(id string) string {
	return strings.TrimPrefix(id, "act_")
}

// UploadVideo uploads a video file to the specified ad account.
func UploadVideo(ctx context.Context, client Client, params UploadVideoParams) (*UploadVideoResult, error) {
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

	var result UploadVideoResult
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parsing upload response: %w", err)
	}

	return &result, nil
}

// GetVideoStatus checks the encoding status of a previously uploaded video.
func GetVideoStatus(ctx context.Context, client Client, params VideoStatusParams) (*VideoStatusResult, error) {
	if params.VideoID == "" {
		return nil, fmt.Errorf("validation: VideoID is required")
	}

	path := fmt.Sprintf("/%s", params.VideoID)
	queryParams := url.Values{}
	queryParams.Set("fields", "id,title,status,length")

	resp, err := client.Get(ctx, path, queryParams)
	if err != nil {
		return nil, err
	}

	var result VideoStatusResult
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parsing video status response: %w", err)
	}

	return &result, nil
}
