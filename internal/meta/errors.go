package meta

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
)

type GraphError struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	Code        int    `json:"code"`
	Subcode     int    `json:"error_subcode"`
	TraceID     string `json:"fbtrace_id"`
	IsRetryable bool
}

const (
	ExitSuccess         = 0
	ExitAPIError        = 1
	ExitAuthError       = 2
	ExitValidationError = 3
	ExitConfigError     = 4
	ExitNetworkError    = 5
)

func (e *GraphError) Error() string {
	return fmt.Sprintf("meta api error: %s (code=%d, subcode=%d)", e.Message, e.Code, e.Subcode)
}

func ParseGraphError(body []byte) *GraphError {
	var wrapper struct {
		Error *GraphError `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil
	}
	if wrapper.Error == nil {
		return nil
	}
	wrapper.Error.IsRetryable = isRetryable(wrapper.Error.Code)
	return wrapper.Error
}

func ClassifyError(err error) int {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ExitNetworkError
	}
	var ge *GraphError
	if errors.As(err, &ge) {
		switch {
		case ge.Code == 190 || ge.Subcode == 467:
			return ExitAuthError
		case ge.Code == 100 || ge.Code == 192:
			return ExitValidationError
		default:
			return ExitAPIError
		}
	}
	return ExitAPIError
}

func isRetryable(code int) bool {
	return code == 1 || code == 2 || code == 4 || code == 17 || code == 341
}

func DollarsToCents(d float64) (int64, error) {
	if d < 0 || math.IsNaN(d) || math.IsInf(d, 0) {
		return 0, fmt.Errorf("invalid dollar amount: %v", d)
	}
	return int64(math.Round(d * 100)), nil
}
