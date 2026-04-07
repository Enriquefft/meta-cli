package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// campaignFields is the canonical set of fields requested from the Graph API
// for a campaign resource. CreateCampaign issues a follow-up GET with this
// field list after the POST returns only {"id": ...}, so the caller always
// receives a fully populated Campaign. It is the single source of truth for
// the shape meta-cli exposes for a campaign.
const campaignFields = "id,name,objective,status,daily_budget,lifetime_budget,bid_strategy,special_ad_categories"

// CreateCampaignParams holds the input for creating a Meta campaign.
type CreateCampaignParams struct {
	AccountID           string
	Name                string
	Objective           string // OUTCOME_SALES, OUTCOME_TRAFFIC, etc.
	DailyBudgetCents    int64  // mutually exclusive with LifetimeBudgetCents
	LifetimeBudgetCents int64
	BidStrategy         string // default: LOWEST_COST_WITHOUT_CAP
	Status              string // default: PAUSED
	SpecialAdCategory   string // default: NONE
}

// Campaign represents a Meta campaign returned by the API.
type Campaign struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Objective      string `json:"objective"`
	Status         string `json:"status"`
	DailyBudget    string `json:"daily_budget,omitempty"`
	LifetimeBudget string `json:"lifetime_budget,omitempty"`
}

// CreateCampaign creates a new campaign under the given ad account.
func CreateCampaign(ctx context.Context, client Client, params CreateCampaignParams) (*Campaign, error) {
	if err := validateCampaignParams(params); err != nil {
		return nil, err
	}

	accountID := normalizeAccountID(params.AccountID)
	path := "/act_" + accountID + "/campaigns"

	// Apply defaults.
	status := params.Status
	if status == "" {
		status = "PAUSED"
	}
	bidStrategy := params.BidStrategy
	if bidStrategy == "" {
		bidStrategy = "LOWEST_COST_WITHOUT_CAP"
	}
	specialAdCategory := params.SpecialAdCategory
	if specialAdCategory == "" {
		specialAdCategory = "NONE"
	}

	catJSON, err := json.Marshal([]string{specialAdCategory})
	if err != nil {
		return nil, fmt.Errorf("marshalling special_ad_categories: %w", err)
	}

	body := map[string]string{
		"name":                  params.Name,
		"objective":             params.Objective,
		"status":                status,
		"bid_strategy":          bidStrategy,
		"special_ad_categories": string(catJSON),
	}

	if params.DailyBudgetCents > 0 {
		body["daily_budget"] = strconv.FormatInt(params.DailyBudgetCents, 10)
	} else if params.LifetimeBudgetCents > 0 {
		body["lifetime_budget"] = strconv.FormatInt(params.LifetimeBudgetCents, 10)
	}

	resp, err := client.Post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("creating campaign: %w", err)
	}

	// Meta's POST /act_*/campaigns endpoint only returns the new campaign's
	// id. Chain a follow-up GET via the shared fetchAfterCreate helper so
	// callers receive a fully populated Campaign instead of an object with
	// empty name/objective/status fields.
	var campaign Campaign
	if err := fetchAfterCreate(ctx, client, resp.Body, "campaign", campaignFields, &campaign); err != nil {
		return nil, err
	}

	return &campaign, nil
}

func validateCampaignParams(p CreateCampaignParams) error {
	if p.AccountID == "" {
		return fmt.Errorf("validation: AccountID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("validation: Name is required")
	}
	if p.Objective == "" {
		return fmt.Errorf("validation: Objective is required")
	}
	if p.DailyBudgetCents == 0 && p.LifetimeBudgetCents == 0 {
		return fmt.Errorf("validation: either DailyBudgetCents or LifetimeBudgetCents is required")
	}
	if p.DailyBudgetCents > 0 && p.LifetimeBudgetCents > 0 {
		return fmt.Errorf("validation: DailyBudgetCents and LifetimeBudgetCents are mutually exclusive")
	}
	return nil
}
