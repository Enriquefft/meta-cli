package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// validTargetingTypes is the set of allowed targeting search types.
var validTargetingTypes = map[string]struct{}{
	"interests":         {},
	"behaviors":         {},
	"demographics":      {},
	"education_schools": {},
	"education_majors":  {},
	"work_employers":    {},
	"work_positions":    {},
}

const defaultTargetingLimit = 25

// SearchTargetingParams contains the parameters for searching targeting options.
type SearchTargetingParams struct {
	AccountID string
	Type      string
	Query     string
	Limit     int
}

// TargetingSuggestion represents a single targeting option returned by the API.
type TargetingSuggestion struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Type                   string   `json:"type"`
	AudienceSizeLowerBound int64    `json:"audience_size_lower_bound"`
	AudienceSizeUpperBound int64    `json:"audience_size_upper_bound"`
	Path                   []string `json:"path"`
}

// SearchTargetingResult contains the list of targeting suggestions from the API.
type SearchTargetingResult struct {
	Data []TargetingSuggestion `json:"data"`
}

// SearchTargeting searches for targeting options (interests, behaviors, demographics, etc.).
func SearchTargeting(ctx context.Context, client Client, params SearchTargetingParams) (*SearchTargetingResult, error) {
	if params.AccountID == "" {
		return nil, fmt.Errorf("validation: AccountID is required")
	}
	if params.Type == "" {
		return nil, fmt.Errorf("validation: Type is required")
	}
	if _, ok := validTargetingTypes[params.Type]; !ok {
		return nil, fmt.Errorf("validation: Type %q is not valid; must be one of: interests, behaviors, demographics, education_schools, education_majors, work_employers, work_positions", params.Type)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("validation: Query is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultTargetingLimit
	}

	accountID := normalizeAccountID(params.AccountID)
	path := "/act_" + accountID + "/targetingsearch"

	queryParams := url.Values{}
	queryParams.Set("type", params.Type)
	queryParams.Set("q", params.Query)
	queryParams.Set("limit", strconv.Itoa(limit))

	resp, err := client.Get(ctx, path, queryParams)
	if err != nil {
		return nil, err
	}

	var result SearchTargetingResult
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parsing targeting search response: %w", err)
	}

	// Ensure empty slice, never nil.
	if result.Data == nil {
		result.Data = []TargetingSuggestion{}
	}

	return &result, nil
}
