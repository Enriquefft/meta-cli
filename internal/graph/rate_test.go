package graph

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimit_ValidHeader(t *testing.T) {
	h := http.Header{}
	h.Set("X-Business-Use-Case-Usage", `{"acc123":[{"call_count":80,"total_cputime":50,"total_time":60,"estimated_time_to_reset":300}]}`)

	rl := ParseRateLimit(h)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Usage != 80 {
		t.Errorf("expected Usage 80, got %d", rl.Usage)
	}
	if rl.TotalCPUTime != 50 {
		t.Errorf("expected TotalCPUTime 50, got %d", rl.TotalCPUTime)
	}
	if rl.TotalTime != 60 {
		t.Errorf("expected TotalTime 60, got %d", rl.TotalTime)
	}
	if rl.EstimatedTimeToReset != 300 {
		t.Errorf("expected EstimatedTimeToReset 300, got %d", rl.EstimatedTimeToReset)
	}
}

func TestParseRateLimit_MissingHeader(t *testing.T) {
	h := http.Header{}
	rl := ParseRateLimit(h)
	if rl != nil {
		t.Error("expected nil for missing header")
	}
}

func TestParseRateLimit_MalformedJSON(t *testing.T) {
	h := http.Header{}
	h.Set("X-Business-Use-Case-Usage", `not json`)
	rl := ParseRateLimit(h)
	if rl != nil {
		t.Error("expected nil for malformed JSON")
	}
}

func TestParseRateLimit_EmptyJSON(t *testing.T) {
	h := http.Header{}
	h.Set("X-Business-Use-Case-Usage", `{}`)
	rl := ParseRateLimit(h)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit for empty accounts map")
	}
	if rl.Usage != 0 {
		t.Errorf("expected Usage 0, got %d", rl.Usage)
	}
}

func TestParseRateLimit_MultipleAccounts(t *testing.T) {
	h := http.Header{}
	h.Set("X-Business-Use-Case-Usage", `{
		"acc1":[{"call_count":40,"total_cputime":20,"total_time":30,"estimated_time_to_reset":100}],
		"acc2":[{"call_count":90,"total_cputime":70,"total_time":80,"estimated_time_to_reset":500}]
	}`)

	rl := ParseRateLimit(h)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Usage != 90 {
		t.Errorf("expected Usage 90 (max across accounts), got %d", rl.Usage)
	}
	if rl.TotalCPUTime != 70 {
		t.Errorf("expected TotalCPUTime 70 (max), got %d", rl.TotalCPUTime)
	}
	if rl.TotalTime != 80 {
		t.Errorf("expected TotalTime 80 (max), got %d", rl.TotalTime)
	}
	if rl.EstimatedTimeToReset != 500 {
		t.Errorf("expected EstimatedTimeToReset 500 (max), got %d", rl.EstimatedTimeToReset)
	}
}

func TestParseRateLimit_MultipleEntriesPerAccount(t *testing.T) {
	h := http.Header{}
	h.Set("X-Business-Use-Case-Usage", `{"acc1":[{"call_count":30,"total_cputime":10,"total_time":20,"estimated_time_to_reset":60},{"call_count":85,"total_cputime":60,"total_time":70,"estimated_time_to_reset":200}]}`)

	rl := ParseRateLimit(h)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Usage != 85 {
		t.Errorf("expected Usage 85 (max across entries), got %d", rl.Usage)
	}
}

func TestBackoffDuration(t *testing.T) {
	tests := []struct {
		usage  int
		expect time.Duration
	}{
		{0, 0},
		{50, 0},
		{74, 0},
		{75, 10 * time.Second},
		{80, 10 * time.Second},
		{99, 10 * time.Second},
		{100, 60 * time.Second},
		{150, 60 * time.Second},
	}
	for _, tt := range tests {
		got := BackoffDuration(tt.usage)
		if got != tt.expect {
			t.Errorf("BackoffDuration(%d) = %v, want %v", tt.usage, got, tt.expect)
		}
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay 1s, got %v", cfg.BaseDelay)
	}
}

func TestDelayForAttempt_ExponentialBackoff(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 1 * time.Second}

	tests := []struct {
		attempt int
		expect  time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
	}
	for _, tt := range tests {
		got := cfg.DelayForAttempt(tt.attempt)
		if got != tt.expect {
			t.Errorf("DelayForAttempt(%d) = %v, want %v", tt.attempt, got, tt.expect)
		}
	}
}

func TestDelayForAttempt_CapsAt30(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 1 * time.Second}
	d30 := cfg.DelayForAttempt(30)
	d50 := cfg.DelayForAttempt(50)
	if d30 != d50 {
		t.Errorf("expected attempt 50 capped to same as 30, got %v vs %v", d50, d30)
	}
	if d30 != maxBackoff {
		t.Errorf("expected capped delay %v, got %v", maxBackoff, d30)
	}
}

func TestDelayForAttempt_CappedAtMaxBackoff(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 1 * time.Second}
	// attempt 5 = 32s which equals maxBackoff
	d5 := cfg.DelayForAttempt(5)
	if d5 != maxBackoff {
		t.Errorf("expected DelayForAttempt(5) = %v, got %v", maxBackoff, d5)
	}
	// attempt 10 would be 1024s without cap, must be 32s
	d10 := cfg.DelayForAttempt(10)
	if d10 != maxBackoff {
		t.Errorf("expected DelayForAttempt(10) capped to %v, got %v", maxBackoff, d10)
	}
}

func TestDelayForAttempt_RespectsBaseDelay(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 500 * time.Millisecond}
	got := cfg.DelayForAttempt(0)
	if got != 500*time.Millisecond {
		t.Errorf("DelayForAttempt(0) with 500ms base = %v, want 500ms", got)
	}
	got = cfg.DelayForAttempt(1)
	if got != 1*time.Second {
		t.Errorf("DelayForAttempt(1) with 500ms base = %v, want 1s", got)
	}
}
