package graph

import (
	"encoding/json"
	"net/http"
	"time"
)

type RateLimit struct {
	Usage                int
	CallCount            int
	TotalTime            int
	TotalCPUTime         int
	EstimatedTimeToReset int
}

func ParseRateLimit(header http.Header) *RateLimit {
	val := header.Get("X-Business-Use-Case-Usage")
	if val == "" {
		return nil
	}

	var accounts map[string][]struct {
		CallCount            int `json:"call_count"`
		TotalCPUTime         int `json:"total_cputime"`
		TotalTime            int `json:"total_time"`
		EstimatedTimeToReset int `json:"estimated_time_to_reset"`
	}

	if err := json.Unmarshal([]byte(val), &accounts); err != nil {
		return nil
	}

	rl := &RateLimit{}
	for _, entries := range accounts {
		for _, e := range entries {
			if e.CallCount > rl.Usage {
				rl.Usage = e.CallCount
			}
			if e.TotalCPUTime > rl.TotalCPUTime {
				rl.TotalCPUTime = e.TotalCPUTime
			}
			if e.TotalTime > rl.TotalTime {
				rl.TotalTime = e.TotalTime
			}
			if e.CallCount > rl.CallCount {
				rl.CallCount = e.CallCount
			}
			if e.EstimatedTimeToReset > rl.EstimatedTimeToReset {
				rl.EstimatedTimeToReset = e.EstimatedTimeToReset
			}
		}
	}

	return rl
}

func BackoffDuration(usage int) time.Duration {
	switch {
	case usage >= 100:
		return 60 * time.Second
	case usage >= 75:
		return 10 * time.Second
	default:
		return 0
	}
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
	}
}

func (r RetryConfig) DelayForAttempt(attempt int) time.Duration {
	if attempt > 30 {
		attempt = 30
	}
	secs := r.BaseDelay.Seconds() * float64(uint(1)<<attempt)
	return time.Duration(secs * float64(time.Second))
}
