package graph

import (
	"fmt"
	"net/http"

	meta "github.com/enriquefft/meta-cli/internal/meta"
)

func CheckResponse(resp *meta.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ge := meta.ParseGraphError(resp.Body)
		if ge != nil {
			return ge
		}
		return nil
	}

	ge := meta.ParseGraphError(resp.Body)
	if ge != nil {
		return ge
	}

	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(resp.Body))
}

func IsRetryableHTTP(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable
}
