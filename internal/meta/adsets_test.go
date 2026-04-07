package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// adSetGetFn returns a GetFn suitable for the MockClient that answers the
// follow-up GET issued by CreateAdSet after the POST returns only
// {"id": ...}. The returned body mirrors the shape the real Graph API sends
// for the adSetFields field list.
func adSetGetFn(t *testing.T, adset AdSet) func(ctx context.Context, path string, params url.Values) (*Response, error) {
	t.Helper()
	return func(_ context.Context, path string, params url.Values) (*Response, error) {
		expectedPath := "/" + adset.ID
		if path != expectedPath {
			t.Errorf("expected GET path %s, got %s", expectedPath, path)
		}
		if params.Get("fields") != adSetFields {
			t.Errorf("expected fields %q, got %q", adSetFields, params.Get("fields"))
		}
		body, _ := json.Marshal(adset)
		return &Response{Body: body, StatusCode: 200}, nil
	}
}

func TestCreateAdSet_Success(t *testing.T) {
	var capturedPath string
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, params map[string]string) (*Response, error) {
			capturedPath = path
			capturedParams = params
			return &Response{Body: []byte(`{"id":"111222"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{
			ID:          "111222",
			Name:        "Test AdSet",
			CampaignID:  "camp_123",
			Status:      "PAUSED",
			DailyBudget: "5000",
		}),
	}

	params := CreateAdSetParams{
		AccountID:        "987654",
		Name:             "Test AdSet",
		CampaignID:       "camp_123",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US", "CA"},
	}

	adset, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/adsets" {
		t.Errorf("expected path /act_987654/adsets, got %s", capturedPath)
	}
	if capturedParams["name"] != "Test AdSet" {
		t.Errorf("expected name 'Test AdSet', got %s", capturedParams["name"])
	}
	if capturedParams["campaign_id"] != "camp_123" {
		t.Errorf("expected campaign_id 'camp_123', got %s", capturedParams["campaign_id"])
	}
	if capturedParams["optimization_goal"] != "CONVERSIONS" {
		t.Errorf("expected optimization_goal 'CONVERSIONS', got %s", capturedParams["optimization_goal"])
	}
	if capturedParams["billing_event"] != "IMPRESSIONS" {
		t.Errorf("expected default billing_event 'IMPRESSIONS', got %s", capturedParams["billing_event"])
	}
	if capturedParams["daily_budget"] != "5000" {
		t.Errorf("expected daily_budget '5000', got %s", capturedParams["daily_budget"])
	}
	if capturedParams["status"] != "PAUSED" {
		t.Errorf("expected default status 'PAUSED', got %s", capturedParams["status"])
	}

	if adset.ID != "111222" {
		t.Errorf("expected adset ID '111222', got %s", adset.ID)
	}
	if adset.Name != "Test AdSet" {
		t.Errorf("expected adset Name 'Test AdSet', got %s", adset.Name)
	}
}

func TestCreateAdSet_TargetingSpec(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US", "GB"},
		AgeMin:           25,
		AgeMax:           55,
		Genders:          []int{1},
		Interests:        []string{"6003139266461", "6003397425735"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	targetingJSON := capturedParams["targeting"]
	if targetingJSON == "" {
		t.Fatal("expected targeting param to be set")
	}

	var targeting map[string]interface{}
	if err := json.Unmarshal([]byte(targetingJSON), &targeting); err != nil {
		t.Fatalf("targeting is not valid JSON: %v", err)
	}

	geoLocations, ok := targeting["geo_locations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected geo_locations in targeting")
	}
	countries, ok := geoLocations["countries"].([]interface{})
	if !ok {
		t.Fatal("expected countries in geo_locations")
	}
	if len(countries) != 2 {
		t.Errorf("expected 2 countries, got %d", len(countries))
	}
	if countries[0] != "US" || countries[1] != "GB" {
		t.Errorf("expected countries [US, GB], got %v", countries)
	}

	ageMin, ok := targeting["age_min"].(float64)
	if !ok || int(ageMin) != 25 {
		t.Errorf("expected age_min 25, got %v", targeting["age_min"])
	}
	ageMax, ok := targeting["age_max"].(float64)
	if !ok || int(ageMax) != 55 {
		t.Errorf("expected age_max 55, got %v", targeting["age_max"])
	}

	genders, ok := targeting["genders"].([]interface{})
	if !ok || len(genders) != 1 {
		t.Fatalf("expected genders [1], got %v", targeting["genders"])
	}
	if int(genders[0].(float64)) != 1 {
		t.Errorf("expected gender 1, got %v", genders[0])
	}

	interests, ok := targeting["interests"].([]interface{})
	if !ok || len(interests) != 2 {
		t.Fatalf("expected 2 interests, got %v", targeting["interests"])
	}
	interest0, ok := interests[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected interest to be an object")
	}
	if interest0["id"] != "6003139266461" {
		t.Errorf("expected first interest id '6003139266461', got %v", interest0["id"])
	}
}

func TestCreateAdSet_DefaultAgesAndGenders(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var targeting map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["targeting"]), &targeting); err != nil {
		t.Fatalf("targeting is not valid JSON: %v", err)
	}

	ageMin := int(targeting["age_min"].(float64))
	if ageMin != 18 {
		t.Errorf("expected default age_min 18, got %d", ageMin)
	}
	ageMax := int(targeting["age_max"].(float64))
	if ageMax != 65 {
		t.Errorf("expected default age_max 65, got %d", ageMax)
	}

	genders := targeting["genders"].([]interface{})
	if len(genders) != 2 {
		t.Fatalf("expected default genders [1,2], got %v", genders)
	}
	if int(genders[0].(float64)) != 1 || int(genders[1].(float64)) != 2 {
		t.Errorf("expected genders [1,2], got %v", genders)
	}
}

func TestCreateAdSet_LifetimeBudget(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:           "123",
		Name:                "Test",
		CampaignID:          "c1",
		LifetimeBudgetCents: 50000,
		OptimizationGoal:    "CONVERSIONS",
		Countries:           []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["lifetime_budget"] != "50000" {
		t.Errorf("expected lifetime_budget '50000', got %s", capturedParams["lifetime_budget"])
	}
	if _, ok := capturedParams["daily_budget"]; ok {
		t.Error("daily_budget should not be set when lifetime_budget is used")
	}
}

func TestCreateAdSet_AccountIDNormalization(t *testing.T) {
	var capturedPath string

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			capturedPath = path
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:        "act_987654",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/act_987654/adsets" {
		t.Errorf("expected path /act_987654/adsets, got %s", capturedPath)
	}
}

func TestCreateAdSet_MissingAccountID(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing AccountID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_MissingName(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		AccountID:        "123",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Name")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_MissingCampaignID(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing CampaignID")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_MissingOptimizationGoal(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		Countries:        []string{"US"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing OptimizationGoal")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_MissingCountries(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for missing Countries")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_EmptyCountries(t *testing.T) {
	mock := &MockClient{}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err == nil {
		t.Fatal("expected validation error for empty Countries")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestCreateAdSet_OptionalFields(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		BillingEvent:     "CLICKS",
		BidAmountCents:   150,
		Countries:        []string{"US"},
		PixelID:          "px_123",
		CustomEventType:  "PURCHASE",
		StartTime:        "2024-01-01T00:00:00+0000",
		EndTime:          "2024-12-31T23:59:59+0000",
		Status:           "ACTIVE",
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams["billing_event"] != "CLICKS" {
		t.Errorf("expected billing_event 'CLICKS', got %s", capturedParams["billing_event"])
	}
	if capturedParams["bid_amount"] != "150" {
		t.Errorf("expected bid_amount '150', got %s", capturedParams["bid_amount"])
	}
	if capturedParams["promoted_object"] == "" {
		t.Error("expected promoted_object to be set when PixelID is provided")
	}
	if capturedParams["start_time"] != "2024-01-01T00:00:00+0000" {
		t.Errorf("expected start_time, got %s", capturedParams["start_time"])
	}
	if capturedParams["end_time"] != "2024-12-31T23:59:59+0000" {
		t.Errorf("expected end_time, got %s", capturedParams["end_time"])
	}
	if capturedParams["status"] != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got %s", capturedParams["status"])
	}
}

func TestCreateAdSet_ExcludedCountriesAndPlatforms(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:          "123",
		Name:               "Test",
		CampaignID:         "c1",
		DailyBudgetCents:   5000,
		OptimizationGoal:   "CONVERSIONS",
		Countries:          []string{"US"},
		ExcludedCountries:  []string{"CN"},
		PublisherPlatforms: []string{"facebook", "instagram"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var targeting map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["targeting"]), &targeting); err != nil {
		t.Fatalf("targeting is not valid JSON: %v", err)
	}

	excludedGeo, ok := targeting["excluded_geo_locations"].(map[string]interface{})
	if !ok {
		t.Fatal("expected excluded_geo_locations in targeting")
	}
	excludedCountries := excludedGeo["countries"].([]interface{})
	if len(excludedCountries) != 1 || excludedCountries[0] != "CN" {
		t.Errorf("expected excluded countries [CN], got %v", excludedCountries)
	}

	platforms := targeting["publisher_platforms"].([]interface{})
	if len(platforms) != 2 {
		t.Errorf("expected 2 publisher_platforms, got %d", len(platforms))
	}
}

func TestCreateAdSet_BehaviorsAndCustomAudiences(t *testing.T) {
	var capturedParams map[string]string

	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, params map[string]string) (*Response, error) {
			capturedParams = params
			return &Response{Body: []byte(`{"id":"1"}`), StatusCode: 200}, nil
		},
		GetFn: adSetGetFn(t, AdSet{ID: "1"}),
	}

	params := CreateAdSetParams{
		AccountID:        "123",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 5000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
		Behaviors:        []string{"beh_1", "beh_2"},
		CustomAudiences:  []string{"aud_1"},
	}

	_, err := CreateAdSet(context.Background(), mock, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var targeting map[string]interface{}
	if err := json.Unmarshal([]byte(capturedParams["targeting"]), &targeting); err != nil {
		t.Fatalf("targeting is not valid JSON: %v", err)
	}

	behaviors, ok := targeting["behaviors"].([]interface{})
	if !ok || len(behaviors) != 2 {
		t.Fatalf("expected 2 behaviors, got %v", targeting["behaviors"])
	}
	beh0 := behaviors[0].(map[string]interface{})
	if beh0["id"] != "beh_1" {
		t.Errorf("expected first behavior id 'beh_1', got %v", beh0["id"])
	}

	audiences, ok := targeting["custom_audiences"].([]interface{})
	if !ok || len(audiences) != 1 {
		t.Fatalf("expected 1 custom audience, got %v", targeting["custom_audiences"])
	}
	aud0 := audiences[0].(map[string]interface{})
	if aud0["id"] != "aud_1" {
		t.Errorf("expected custom audience id 'aud_1', got %v", aud0["id"])
	}
}

// TestCreateAdSet_FetchesFullRecordAfterCreate is the regression test for
// the empty-fields bug: Meta's POST /adsets returns only {"id": ...}, so
// CreateAdSet must issue a follow-up GET to enrich the returned AdSet with
// name, campaign_id, status, and budget fields.
func TestCreateAdSet_FetchesFullRecordAfterCreate(t *testing.T) {
	var postCalled, getCalled bool

	mock := &MockClient{
		PostFn: func(_ context.Context, path string, _ map[string]string) (*Response, error) {
			postCalled = true
			if path != "/act_77/adsets" {
				t.Errorf("expected POST path /act_77/adsets, got %s", path)
			}
			return &Response{Body: []byte(`{"id":"as_full"}`), StatusCode: 200}, nil
		},
		GetFn: func(_ context.Context, path string, params url.Values) (*Response, error) {
			getCalled = true
			if path != "/as_full" {
				t.Errorf("expected GET path /as_full, got %s", path)
			}
			if params.Get("fields") != adSetFields {
				t.Errorf("expected fields %q, got %q", adSetFields, params.Get("fields"))
			}
			body := `{"id":"as_full","name":"Enriched AS","campaign_id":"camp_77","status":"PAUSED","daily_budget":"2000"}`
			return &Response{Body: []byte(body), StatusCode: 200}, nil
		},
	}

	result, err := CreateAdSet(context.Background(), mock, CreateAdSetParams{
		AccountID:        "77",
		Name:             "Enriched AS",
		CampaignID:       "camp_77",
		DailyBudgetCents: 2000,
		OptimizationGoal: "LINK_CLICKS",
		Countries:        []string{"US"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Fatal("expected CreateAdSet to POST")
	}
	if !getCalled {
		t.Fatal("expected CreateAdSet to issue a follow-up GET for full metadata")
	}
	if result.ID != "as_full" {
		t.Errorf("expected ID as_full, got %q", result.ID)
	}
	if result.Name != "Enriched AS" {
		t.Errorf("expected Name Enriched AS, got %q", result.Name)
	}
	if result.CampaignID != "camp_77" {
		t.Errorf("expected CampaignID camp_77, got %q", result.CampaignID)
	}
	if result.Status != "PAUSED" {
		t.Errorf("expected Status PAUSED, got %q", result.Status)
	}
	if result.DailyBudget != "2000" {
		t.Errorf("expected DailyBudget 2000, got %q", result.DailyBudget)
	}
}

// TestCreateAdSet_MissingIDInPostResponse asserts a defensive error is
// returned when the POST response lacks an id.
func TestCreateAdSet_MissingIDInPostResponse(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return &Response{Body: []byte(`{}`), StatusCode: 200}, nil
		},
	}

	_, err := CreateAdSet(context.Background(), mock, CreateAdSetParams{
		AccountID:        "1",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 1000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	})
	if err == nil {
		t.Fatal("expected error when POST response omits id")
	}
	if !strings.Contains(err.Error(), "ad set id") {
		t.Errorf("expected error to mention missing ad set id, got: %v", err)
	}
}

// TestCreateAdSet_PostErrorPropagated asserts POST-level transport errors
// short-circuit the Create flow and are surfaced to the caller.
func TestCreateAdSet_PostErrorPropagated(t *testing.T) {
	mock := &MockClient{
		PostFn: func(_ context.Context, _ string, _ map[string]string) (*Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	_, err := CreateAdSet(context.Background(), mock, CreateAdSetParams{
		AccountID:        "1",
		Name:             "Test",
		CampaignID:       "c1",
		DailyBudgetCents: 1000,
		OptimizationGoal: "CONVERSIONS",
		Countries:        []string{"US"},
	})
	if err == nil {
		t.Fatal("expected POST error to be propagated")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected original error in chain, got: %v", err)
	}
}
