package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetManagementUsageSummaryAggregatesUsageTotals(t *testing.T) {
	now := time.Now().In(time.Local).Round(0)
	earlier := now.Add(-45 * time.Minute)
	older := now.Add(-24 * time.Hour)
	todayKey := now.Format("2006-01-02")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/management/usage":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"failed_requests": 4,
				"usage": map[string]any{
					"total_requests": 12,
					"success_count":  8,
					"failure_count":  4,
					"total_tokens":   1234,
					"apis": map[string]any{
						"key-a": map[string]any{
							"models": map[string]any{
								"gpt-5.4": map[string]any{
									"details": []any{
										map[string]any{
											"timestamp": earlier.Format(time.RFC3339Nano),
											"tokens": map[string]any{
												"input_tokens":     400,
												"output_tokens":    100,
												"reasoning_tokens": 20,
												"cached_tokens":    50,
												"total_tokens":     500,
											},
											"failed": false,
										},
										map[string]any{
											"timestamp": now.Format(time.RFC3339Nano),
											"tokens": map[string]any{
												"input_tokens":     600,
												"output_tokens":    80,
												"reasoning_tokens": 10,
												"cached_tokens":    44,
												"total_tokens":     690,
											},
											"failed": false,
										},
									},
								},
								"gpt-5.1-codex": map[string]any{
									"details": []any{
										map[string]any{
											"timestamp": older.Format(time.RFC3339Nano),
											"tokens": map[string]any{
												"input_tokens":     0,
												"output_tokens":    0,
												"reasoning_tokens": 0,
												"cached_tokens":    0,
												"total_tokens":     0,
											},
											"failed": true,
										},
									},
								},
							},
						},
					},
					"requests_by_day": map[string]any{
						todayKey: 11,
					},
					"tokens_by_day": map[string]any{
						todayKey: 1200,
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	summary, err := service.GetManagementUsageSummary()
	if err != nil {
		t.Fatalf("GetManagementUsageSummary: %v", err)
	}

	if summary.TotalRequests != 12 || summary.SuccessCount != 8 || summary.FailureCount != 4 || summary.FailedRequests != 4 {
		t.Fatalf("unexpected request summary: %+v", summary)
	}
	if summary.TodayRequests != 11 || summary.TodayTokens != 1200 {
		t.Fatalf("unexpected today summary: %+v", summary)
	}
	if summary.APIKeyCount != 1 || summary.ModelCount != 2 {
		t.Fatalf("unexpected API/model counts: %+v", summary)
	}
	if summary.Tokens.TotalTokens != 1234 {
		t.Fatalf("unexpected total token count: %+v", summary.Tokens)
	}
	if summary.Tokens.InputTokens != 1000 || summary.Tokens.OutputTokens != 180 || summary.Tokens.ReasoningTokens != 30 || summary.Tokens.CachedTokens != 94 {
		t.Fatalf("unexpected token breakdown: %+v", summary.Tokens)
	}

	parsedLastRequest, err := time.Parse(time.RFC3339Nano, summary.LastRequestAt)
	if err != nil {
		t.Fatalf("parse LastRequestAt: %v", err)
	}
	if !parsedLastRequest.Equal(now) {
		t.Fatalf("unexpected last request time: got %s want %s", parsedLastRequest.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	}
	if summary.FetchedAt == "" {
		t.Fatalf("expected fetchedAt to be set")
	}
}
