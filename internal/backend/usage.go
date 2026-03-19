package backend

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (b *Backend) GetManagementUsageSummary() (ManagementUsageSummary, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return ManagementUsageSummary{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return ManagementUsageSummary{}, err
	}

	payload, err := b.client.FetchManagementUsage(context.Background(), settings)
	if err != nil {
		return ManagementUsageSummary{}, err
	}

	summary, err := summarizeManagementUsage(payload, time.Now())
	if err != nil {
		return ManagementUsageSummary{}, err
	}
	return summary, nil
}

func summarizeManagementUsage(payload map[string]any, now time.Time) (ManagementUsageSummary, error) {
	usage := objectFromAny(payload["usage"])
	if len(usage) == 0 {
		return ManagementUsageSummary{}, errors.New("management usage response missing usage summary")
	}

	summary := ManagementUsageSummary{
		FetchedAt:      nowISO(),
		TotalRequests:  usageIntValue(usage["total_requests"]),
		SuccessCount:   usageIntValue(usage["success_count"]),
		FailureCount:   usageIntValue(usage["failure_count"]),
		FailedRequests: usageIntValue(payload["failed_requests"]),
		Tokens: UsageTokenTotals{
			TotalTokens: usageInt64Value(usage["total_tokens"]),
		},
	}
	topLevelTotalTokensProvided := summary.Tokens.TotalTokens > 0

	todayKey := now.In(time.Local).Format("2006-01-02")
	summary.TodayRequests = usageIntValue(objectFromAny(usage["requests_by_day"])[todayKey])
	summary.TodayTokens = usageInt64Value(objectFromAny(usage["tokens_by_day"])[todayKey])

	apis := objectFromAny(usage["apis"])
	summary.APIKeyCount = len(apis)
	models := make(map[string]struct{})
	var latestRequestAt time.Time
	var latestRequestSet bool

	for _, apiValue := range apis {
		apiEntry := objectFromAny(apiValue)
		for modelName, modelValue := range objectFromAny(apiEntry["models"]) {
			trimmedModelName := strings.TrimSpace(modelName)
			if trimmedModelName != "" {
				models[trimmedModelName] = struct{}{}
			}
			modelEntry := objectFromAny(modelValue)
			details, _ := modelEntry["details"].([]any)
			for _, detailValue := range details {
				detail := objectFromAny(detailValue)
				tokens := objectFromAny(detail["tokens"])
				summary.Tokens.InputTokens += usageInt64Value(tokens["input_tokens"])
				summary.Tokens.OutputTokens += usageInt64Value(tokens["output_tokens"])
				summary.Tokens.ReasoningTokens += usageInt64Value(tokens["reasoning_tokens"])
				summary.Tokens.CachedTokens += usageInt64Value(tokens["cached_tokens"])
				if !topLevelTotalTokensProvided {
					summary.Tokens.TotalTokens += usageInt64Value(tokens["total_tokens"])
				}

				timestamp := strings.TrimSpace(stringValue(detail["timestamp"]))
				if timestamp == "" {
					continue
				}
				parsed, err := time.Parse(time.RFC3339Nano, timestamp)
				if err != nil {
					continue
				}
				if !latestRequestSet || parsed.After(latestRequestAt) {
					latestRequestAt = parsed
					latestRequestSet = true
				}
			}
		}
	}

	summary.ModelCount = len(models)
	if latestRequestSet {
		summary.LastRequestAt = latestRequestAt.Format(time.RFC3339Nano)
	}
	if summary.FailedRequests == 0 && summary.FailureCount > 0 {
		summary.FailedRequests = summary.FailureCount
	}
	if summary.TodayTokens == 0 && summary.Tokens.TotalTokens > 0 && summary.TodayRequests == summary.TotalRequests {
		summary.TodayTokens = summary.Tokens.TotalTokens
	}

	return summary, nil
}

func usageIntValue(value any) int {
	parsed, ok := intValueFromAny(value)
	if !ok {
		return 0
	}
	return parsed
}

func usageInt64Value(value any) int64 {
	parsed, ok := int64ValueFromAny(value)
	if !ok {
		return 0
	}
	return parsed
}
