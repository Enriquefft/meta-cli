package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// adSetFields is the canonical set of fields requested from the Graph API
// for an ad set resource. CreateAdSet issues a follow-up GET with this
// field list after the POST returns only {"id": ...}, so the caller always
// receives a fully populated AdSet. It is the single source of truth for
// the shape meta-cli exposes for an ad set.
const adSetFields = "id,name,campaign_id,status,daily_budget,lifetime_budget,optimization_goal,billing_event"

// CreateAdSetParams holds the input for creating a Meta ad set.
type CreateAdSetParams struct {
	AccountID           string
	Name                string
	CampaignID          string
	DailyBudgetCents    int64
	LifetimeBudgetCents int64
	OptimizationGoal    string
	BillingEvent        string // default: IMPRESSIONS
	BidAmountCents      int64
	Countries           []string // required, non-empty
	AgeMin              int      // default: 18
	AgeMax              int      // default: 65
	Genders             []int    // default: [1,2]
	Interests           []string // interest IDs
	Behaviors           []string
	CustomAudiences     []string
	ExcludedCountries   []string
	PublisherPlatforms  []string
	PixelID             string
	CustomEventType     string
	StartTime           string
	EndTime             string
	AdvantageAudience   bool   // default: false (explicit targeting)
	Status              string // default: PAUSED
}

// AdSet represents a Meta ad set returned by the API.
type AdSet struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CampaignID     string `json:"campaign_id"`
	Status         string `json:"status"`
	DailyBudget    string `json:"daily_budget,omitempty"`
	LifetimeBudget string `json:"lifetime_budget,omitempty"`
}

// CreateAdSet creates a new ad set under the given ad account.
func CreateAdSet(ctx context.Context, client Client, params CreateAdSetParams) (*AdSet, error) {
	if err := validateAdSetParams(params); err != nil {
		return nil, err
	}

	accountID := normalizeAccountID(params.AccountID)
	path := "/act_" + accountID + "/adsets"

	// Apply defaults.
	status := params.Status
	if status == "" {
		status = "PAUSED"
	}
	billingEvent := params.BillingEvent
	if billingEvent == "" {
		billingEvent = "IMPRESSIONS"
	}

	targetingJSON, err := buildTargetingSpec(params)
	if err != nil {
		return nil, fmt.Errorf("building targeting spec: %w", err)
	}

	body := map[string]string{
		"name":              params.Name,
		"campaign_id":       params.CampaignID,
		"optimization_goal": params.OptimizationGoal,
		"billing_event":     billingEvent,
		"status":            status,
		"targeting":         targetingJSON,
	}

	if params.DailyBudgetCents > 0 {
		body["daily_budget"] = strconv.FormatInt(params.DailyBudgetCents, 10)
	}
	if params.LifetimeBudgetCents > 0 {
		body["lifetime_budget"] = strconv.FormatInt(params.LifetimeBudgetCents, 10)
	}
	if params.BidAmountCents > 0 {
		body["bid_amount"] = strconv.FormatInt(params.BidAmountCents, 10)
	}
	if params.StartTime != "" {
		body["start_time"] = params.StartTime
	}
	if params.EndTime != "" {
		body["end_time"] = params.EndTime
	}
	if params.PixelID != "" {
		promotedObj := map[string]string{"pixel_id": params.PixelID}
		if params.CustomEventType != "" {
			promotedObj["custom_event_type"] = params.CustomEventType
		}
		poJSON, err := json.Marshal(promotedObj)
		if err != nil {
			return nil, fmt.Errorf("marshalling promoted_object: %w", err)
		}
		body["promoted_object"] = string(poJSON)
	}

	resp, err := client.Post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("creating ad set: %w", err)
	}

	// Meta's POST /act_*/adsets endpoint only returns the new ad set's id.
	// Chain a follow-up GET via the shared fetchAfterCreate helper so
	// callers receive a fully populated AdSet instead of an object with
	// empty name/status fields.
	var adset AdSet
	if err := fetchAfterCreate(ctx, client, resp.Body, "ad set", adSetFields, &adset); err != nil {
		return nil, err
	}

	return &adset, nil
}

// buildTargetingSpec constructs the targeting JSON object for the Meta API.
func buildTargetingSpec(params CreateAdSetParams) (string, error) {
	ageMin := params.AgeMin
	if ageMin == 0 {
		ageMin = 18
	}
	ageMax := params.AgeMax
	if ageMax == 0 {
		ageMax = 65
	}
	genders := params.Genders
	if len(genders) == 0 {
		genders = []int{1, 2}
	}

	targeting := map[string]interface{}{
		"geo_locations": map[string]interface{}{
			"countries": params.Countries,
		},
		"age_min": ageMin,
		"age_max": ageMax,
		"genders": genders,
	}

	if len(params.Interests) > 0 {
		interests := make([]map[string]string, len(params.Interests))
		for i, id := range params.Interests {
			interests[i] = map[string]string{"id": id}
		}
		targeting["interests"] = interests
	}

	if len(params.Behaviors) > 0 {
		behaviors := make([]map[string]string, len(params.Behaviors))
		for i, id := range params.Behaviors {
			behaviors[i] = map[string]string{"id": id}
		}
		targeting["behaviors"] = behaviors
	}

	if len(params.CustomAudiences) > 0 {
		audiences := make([]map[string]string, len(params.CustomAudiences))
		for i, id := range params.CustomAudiences {
			audiences[i] = map[string]string{"id": id}
		}
		targeting["custom_audiences"] = audiences
	}

	if len(params.ExcludedCountries) > 0 {
		targeting["excluded_geo_locations"] = map[string]interface{}{
			"countries": params.ExcludedCountries,
		}
	}

	if len(params.PublisherPlatforms) > 0 {
		targeting["publisher_platforms"] = params.PublisherPlatforms
	}

	advantageFlag := 0
	if params.AdvantageAudience {
		advantageFlag = 1
	}
	targeting["targeting_automation"] = map[string]interface{}{
		"advantage_audience": advantageFlag,
	}

	b, err := json.Marshal(targeting)
	if err != nil {
		return "", fmt.Errorf("marshalling targeting: %w", err)
	}

	return string(b), nil
}

func validateAdSetParams(p CreateAdSetParams) error {
	if p.AccountID == "" {
		return fmt.Errorf("validation: AccountID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("validation: Name is required")
	}
	if p.CampaignID == "" {
		return fmt.Errorf("validation: CampaignID is required")
	}
	if p.OptimizationGoal == "" {
		return fmt.Errorf("validation: OptimizationGoal is required")
	}
	if len(p.Countries) == 0 {
		return fmt.Errorf("validation: Countries is required and must not be empty")
	}
	return nil
}
