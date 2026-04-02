package meta

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateCreativeParams holds the input for creating a Meta ad creative.
type CreateCreativeParams struct {
	AccountID          string
	Name               string
	PageID             string // required
	VideoID            string // one of VideoID/ImageHash/ImageURL required
	ImageHash          string
	ImageURL           string
	Message            string // required
	Headline           string
	Description        string
	CTA                string // default: SHOP_NOW
	Link               string // required
	InstagramAccountID string
}

// Creative represents a Meta ad creative returned by the API.
type Creative struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateCreative creates a new ad creative under the given ad account.
func CreateCreative(ctx context.Context, client Client, params CreateCreativeParams) (*Creative, error) {
	if err := validateCreativeParams(params); err != nil {
		return nil, err
	}

	accountID := normalizeAccountID(params.AccountID)
	path := "/act_" + accountID + "/adcreatives"

	cta := params.CTA
	if cta == "" {
		cta = "SHOP_NOW"
	}

	oss, err := buildObjectStorySpec(params, cta)
	if err != nil {
		return nil, err
	}

	body := map[string]string{
		"name":              params.Name,
		"object_story_spec": oss,
	}

	resp, err := client.Post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("creating creative: %w", err)
	}

	var creative Creative
	if err := json.Unmarshal(resp.Body, &creative); err != nil {
		return nil, fmt.Errorf("parsing creative response: %w", err)
	}

	return &creative, nil
}

// buildObjectStorySpec constructs the object_story_spec JSON for the Meta API.
func buildObjectStorySpec(params CreateCreativeParams, cta string) (string, error) {
	callToAction := map[string]interface{}{
		"type": cta,
		"value": map[string]string{
			"link": params.Link,
		},
	}

	spec := map[string]interface{}{
		"page_id": params.PageID,
	}

	if params.InstagramAccountID != "" {
		spec["instagram_actor_id"] = params.InstagramAccountID
	}

	if params.VideoID != "" {
		videoData := map[string]interface{}{
			"video_id":       params.VideoID,
			"message":        params.Message,
			"call_to_action": callToAction,
		}
		if params.ImageHash != "" {
			videoData["image_hash"] = params.ImageHash
		} else if params.ImageURL != "" {
			videoData["image_url"] = params.ImageURL
		}
		if params.Headline != "" {
			videoData["title"] = params.Headline
		}
		spec["video_data"] = videoData
	} else {
		linkData := map[string]interface{}{
			"message":        params.Message,
			"call_to_action": callToAction,
			"link":           params.Link,
		}
		if params.ImageHash != "" {
			linkData["image_hash"] = params.ImageHash
		} else {
			linkData["picture"] = params.ImageURL
		}
		if params.Headline != "" {
			linkData["name"] = params.Headline
		}
		if params.Description != "" {
			linkData["description"] = params.Description
		}
		spec["link_data"] = linkData
	}

	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshalling object_story_spec: %w", err)
	}

	return string(b), nil
}

func validateCreativeParams(p CreateCreativeParams) error {
	if p.AccountID == "" {
		return fmt.Errorf("validation: AccountID is required")
	}
	if p.PageID == "" {
		return fmt.Errorf("validation: PageID is required")
	}
	if p.Message == "" {
		return fmt.Errorf("validation: Message is required")
	}
	if p.Link == "" {
		return fmt.Errorf("validation: Link is required")
	}
	if p.VideoID == "" && p.ImageHash == "" && p.ImageURL == "" {
		return fmt.Errorf("validation: one of VideoID, ImageHash, or ImageURL is required")
	}
	return nil
}
