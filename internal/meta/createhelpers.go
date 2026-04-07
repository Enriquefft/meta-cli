package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// createResponse is the thin envelope Meta returns from every POST that
// creates a resource under an ad account (/campaigns, /adsets, /adcreatives,
// /ads, /advideos, ...). The response only carries the new object's ID; the
// full record must be fetched separately via GET /{id}?fields=...
type createResponse struct {
	ID string `json:"id"`
}

// fetchAfterCreate is the single source of truth for the "POST returned just
// an id, now fetch the full record" pattern shared by every Create<Resource>
// function in this package (CreateCampaign, CreateAdSet, CreateCreative,
// CreateAd, UploadVideo). It exists so that the chained GET behaviour lives
// in exactly one place: callers wire up their own POST / Upload call, hand
// the raw response body here, and receive a fully populated resource decoded
// into dst.
//
// The resourceKind string is used to produce consistent, human-readable
// error messages (e.g. "create response missing campaign id"). It should be
// the singular noun for the resource ("campaign", "ad set", "creative",
// "ad", "video") — it is NOT a URL path component.
//
// fields is the canonical Graph API field list to request during the
// follow-up GET (e.g. campaignFields). Each resource declares its own
// <resource>Fields constant so that Create<Resource> and any future
// Get<Resource> share the same shape — single source of truth for what the
// domain exposes about a resource.
//
// dst must be a non-nil pointer to a struct whose JSON tags match the
// Graph API field names.
func fetchAfterCreate(
	ctx context.Context,
	client Client,
	postBody []byte,
	resourceKind string,
	fields string,
	dst any,
) error {
	var created createResponse
	if err := json.Unmarshal(postBody, &created); err != nil {
		return fmt.Errorf("parsing create response: %w", err)
	}
	if created.ID == "" {
		return fmt.Errorf("create response missing %s id", resourceKind)
	}

	path := "/" + created.ID
	queryParams := url.Values{}
	queryParams.Set("fields", fields)

	resp, err := client.Get(ctx, path, queryParams)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(resp.Body, dst); err != nil {
		return fmt.Errorf("parsing %s response: %w", resourceKind, err)
	}
	return nil
}
