package meta

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateAdParams holds the input for creating a Meta ad.
type CreateAdParams struct {
	AccountID  string
	Name       string
	AdSetID    string
	CreativeID string
	Status     string // default: PAUSED
}

// Ad represents a Meta ad returned by the API.
type Ad struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	AdSetID  string     `json:"adset_id"`
	Creative AdCreative `json:"creative"`
	Status   string     `json:"status"`
}

// AdCreative holds the creative reference within an ad.
type AdCreative struct {
	ID string `json:"id"`
}

// CreateAd creates a new ad under the given ad account.
func CreateAd(ctx context.Context, client Client, params CreateAdParams) (*Ad, error) {
	if err := validateAdParams(params); err != nil {
		return nil, err
	}

	accountID := normalizeAccountID(params.AccountID)
	path := "/act_" + accountID + "/ads"

	status := params.Status
	if status == "" {
		status = "PAUSED"
	}

	creativeJSON, err := json.Marshal(map[string]string{
		"creative_id": params.CreativeID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling creative param: %w", err)
	}

	body := map[string]string{
		"name":     params.Name,
		"adset_id": params.AdSetID,
		"creative": string(creativeJSON),
		"status":   status,
	}

	resp, err := client.Post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("creating ad: %w", err)
	}

	var ad Ad
	if err := json.Unmarshal(resp.Body, &ad); err != nil {
		return nil, fmt.Errorf("parsing ad response: %w", err)
	}

	return &ad, nil
}

func validateAdParams(p CreateAdParams) error {
	if p.AccountID == "" {
		return fmt.Errorf("validation: AccountID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("validation: Name is required")
	}
	if p.AdSetID == "" {
		return fmt.Errorf("validation: AdSetID is required")
	}
	if p.CreativeID == "" {
		return fmt.Errorf("validation: CreativeID is required")
	}
	return nil
}
