package meta

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
)

// GraphError is the decoded representation of a Meta Graph API error response.
//
// Meta includes two developer-facing fields on error responses — error_user_title
// and error_user_msg — that describe exactly what is wrong and how to fix it.
// They are the difference between a generic "Invalid parameter" and an actionable
// diagnostic, so they are surfaced on this struct (and in the Error() string)
// rather than being dropped.
type GraphError struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	Code        int    `json:"code"`
	Subcode     int    `json:"error_subcode"`
	TraceID     string `json:"fbtrace_id"`
	UserTitle   string `json:"error_user_title,omitempty"`
	UserMessage string `json:"error_user_msg,omitempty"`
	IsRetryable bool   `json:"-"`
}

const (
	ExitSuccess         = 0
	ExitAPIError        = 1
	ExitAuthError       = 2
	ExitValidationError = 3
	ExitConfigError     = 4
	ExitNetworkError    = 5
)

// Error renders the error as a scannable single line. When Meta provides the
// developer-facing UserTitle/UserMessage fields they are included, because they
// are typically the most actionable part of the payload.
func (e *GraphError) Error() string {
	var b strings.Builder
	b.WriteString("meta api error: ")
	b.WriteString(e.Message)

	if detail := e.userDetail(); detail != "" {
		b.WriteString(" — ")
		b.WriteString(detail)
	}

	fmt.Fprintf(&b, " (code=%d, subcode=%d", e.Code, e.Subcode)
	if e.TraceID != "" {
		fmt.Fprintf(&b, ", trace=%s", e.TraceID)
	}
	b.WriteString(")")
	return b.String()
}

// userDetail joins UserTitle and UserMessage into a single human-readable
// fragment. Either, both, or neither may be present.
func (e *GraphError) userDetail() string {
	switch {
	case e.UserTitle != "" && e.UserMessage != "":
		return e.UserTitle + ": " + e.UserMessage
	case e.UserTitle != "":
		return e.UserTitle
	case e.UserMessage != "":
		return e.UserMessage
	default:
		return ""
	}
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
