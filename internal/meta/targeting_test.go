package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestSearchTargeting_Success(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			if path != "/search" {
				t.Errorf("expected path /search, got %s", path)
			}
			if params.Get("type") != "interests" {
				t.Errorf("expected type interests, got %s", params.Get("type"))
			}
			if params.Get("q") != "e-commerce" {
				t.Errorf("expected q 'e-commerce', got %s", params.Get("q"))
			}
			if params.Get("limit") != "10" {
				t.Errorf("expected limit 10, got %s", params.Get("limit"))
			}

			body, _ := json.Marshal(map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                        "6003276047589",
						"name":                      "E-commerce",
						"type":                      "interests",
						"audience_size_lower_bound": 500000000,
						"audience_size_upper_bound": 600000000,
						"path":                      []string{"Interests", "Shopping and fashion", "E-commerce"},
					},
					{
						"id":                        "6003276047590",
						"name":                      "Online shopping",
						"type":                      "interests",
						"audience_size_lower_bound": 300000000,
						"audience_size_upper_bound": 400000000,
						"path":                      []string{"Interests", "Shopping and fashion", "Online shopping"},
					},
				},
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	result, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type:  "interests",
		Query: "e-commerce",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Data))
	}

	first := result.Data[0]
	if first.ID != "6003276047589" {
		t.Errorf("expected ID 6003276047589, got %s", first.ID)
	}
	if first.Name != "E-commerce" {
		t.Errorf("expected Name 'E-commerce', got %s", first.Name)
	}
	if first.Type != "interests" {
		t.Errorf("expected Type interests, got %s", first.Type)
	}
	if first.AudienceSizeLowerBound != 500000000 {
		t.Errorf("expected AudienceSizeLowerBound 500000000, got %d", first.AudienceSizeLowerBound)
	}
	if first.AudienceSizeUpperBound != 600000000 {
		t.Errorf("expected AudienceSizeUpperBound 600000000, got %d", first.AudienceSizeUpperBound)
	}
	if len(first.Path) != 3 || first.Path[2] != "E-commerce" {
		t.Errorf("expected Path [Interests, Shopping and fashion, E-commerce], got %v", first.Path)
	}

	second := result.Data[1]
	if second.ID != "6003276047590" {
		t.Errorf("expected second ID 6003276047590, got %s", second.ID)
	}
	if second.Name != "Online shopping" {
		t.Errorf("expected second Name 'Online shopping', got %s", second.Name)
	}
}

func TestSearchTargeting_EmptyResults(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			body, _ := json.Marshal(map[string]interface{}{
				"data": []interface{}{},
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	result, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type:  "interests",
		Query: "xyznonexistent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Data))
	}
}

func TestSearchTargeting_DefaultLimit(t *testing.T) {
	var capturedParams url.Values

	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			capturedParams = params
			body, _ := json.Marshal(map[string]interface{}{
				"data": []interface{}{},
			})
			return &Response{Body: body, StatusCode: 200}, nil
		},
	}

	_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type:  "interests",
		Query: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams.Get("limit") != "25" {
		t.Errorf("expected default limit 25, got %s", capturedParams.Get("limit"))
	}
}

func TestSearchTargeting_MissingType(t *testing.T) {
	mock := &MockClient{}

	_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Query: "test",
	})
	if err == nil {
		t.Fatal("expected validation error for missing Type")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestSearchTargeting_InvalidType(t *testing.T) {
	mock := &MockClient{}

	_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type:  "invalid_type",
		Query: "test",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid Type")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid_type") {
		t.Errorf("expected error to mention the invalid type, got: %v", err)
	}
}

func TestSearchTargeting_MissingQuery(t *testing.T) {
	mock := &MockClient{}

	_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type: "interests",
	})
	if err == nil {
		t.Fatal("expected validation error for missing Query")
	}
	if !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestSearchTargeting_ClientError(t *testing.T) {
	mock := &MockClient{
		GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
			return nil, fmt.Errorf("rate limit exceeded")
		},
	}

	_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
		Type:  "interests",
		Query: "test",
	})
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected original error to be propagated, got: %v", err)
	}
}

func TestSearchTargeting_AllValidTypes(t *testing.T) {
	validTypes := []string{
		"interests",
		"behaviors",
		"demographics",
		"education_schools",
		"education_majors",
		"work_employers",
		"work_positions",
	}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			mock := &MockClient{
				GetFn: func(ctx context.Context, path string, params url.Values) (*Response, error) {
					if params.Get("type") != typ {
						t.Errorf("expected type %s, got %s", typ, params.Get("type"))
					}
					body, _ := json.Marshal(map[string]interface{}{
						"data": []interface{}{},
					})
					return &Response{Body: body, StatusCode: 200}, nil
				},
			}

			_, err := SearchTargeting(context.Background(), mock, SearchTargetingParams{
				Type:  typ,
				Query: "test",
			})
			if err != nil {
				t.Fatalf("unexpected error for valid type %s: %v", typ, err)
			}
		})
	}
}
