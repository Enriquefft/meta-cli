package meta

import (
	"context"
	"encoding/json"
	"fmt"
)

// adFields is the canonical set of fields requested from the Graph API for
// an ad resource. CreateAd issues a follow-up GET with this field list
// after the POST returns only {"id": ...}, so the caller always receives a
// fully populated Ad. It is the single source of truth for the shape
// meta-cli exposes for an ad.
//
// Note: the creative sub-object is requested via the nested field syntax
// "creative{id}" so the GET returns {"creative": {"id": ...}} matching the
// AdCreative type. Requesting "creative" alone yields an object with every
// ad creative field, which bloats the response and does not align with the
// minimal AdCreative struct.
const adFields = "id,name,adset_id,status,creative{id}"

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

	// Meta's POST /act_*/ads endpoint only returns the new ad's id. Chain a
	// follow-up GET via the shared fetchAfterCreate helper so callers
	// receive a fully populated Ad instead of an object with empty
	// name/adset_id/creative/status fields.
	var ad Ad
	if err := fetchAfterCreate(ctx, client, resp.Body, "ad", adFields, &ad); err != nil {
		return nil, err
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
