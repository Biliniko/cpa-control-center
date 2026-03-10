package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	retryDelay time.Duration
}

type ProbeRetryEvent struct {
	AccountName    string
	RetryIndex     int
	MaxRetries     int
	ProbeErrorKind string
	ProbeErrorText string
	StatusCode     int
}

type ProbeRetryObserver func(ProbeRetryEvent)

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
		retryDelay: 150 * time.Millisecond,
	}
}

func (c *Client) TestConnection(ctx context.Context, settings AppSettings) (ConnectionResult, error) {
	accountCount, err := c.checkManagementAccess(ctx, settings)
	if err != nil {
		return ConnectionResult{
			OK:        false,
			Message:   err.Error(),
			CheckedAt: nowISO(),
		}, err
	}

	return ConnectionResult{
		OK:           true,
		Message:      msg(settings.Locale, "connection.success"),
		AccountCount: accountCount,
		CheckedAt:    nowISO(),
	}, nil
}

func (c *Client) checkManagementAccess(ctx context.Context, settings AppSettings) (int, error) {
	_, err := c.doRequest(ctx, settings, http.MethodGet, settings.BaseURL+"/v0/management/config", nil)
	if err == nil {
		return 0, nil
	}
	if !isManagementHTTPStatus(err, http.StatusNotFound) {
		return 0, err
	}

	files, fallbackErr := c.FetchAuthFiles(ctx, settings)
	if fallbackErr != nil {
		return 0, fallbackErr
	}
	return len(files), nil
}

func (c *Client) FetchAuthFiles(ctx context.Context, settings AppSettings) ([]map[string]any, error) {
	body, err := c.doRequest(ctx, settings, http.MethodGet, settings.BaseURL+"/v0/management/auth-files", nil)
	if err != nil {
		return nil, err
	}

	filesValue, ok := body["files"]
	if !ok {
		return nil, errors.New(msg(settings.Locale, "error.response_missing_files"))
	}

	fileItems, ok := filesValue.([]any)
	if !ok {
		return nil, errors.New(msg(settings.Locale, "error.response_files_not_list"))
	}

	files := make([]map[string]any, 0, len(fileItems))
	for _, item := range fileItems {
		asMap, ok := item.(map[string]any)
		if ok {
			files = append(files, asMap)
		}
	}
	return files, nil
}

func (c *Client) BuildAccountRecord(item map[string]any, previous *AccountRecord, timestamp string) AccountRecord {
	record := AccountRecord{
		Name:             strings.TrimSpace(stringValue(item["name"])),
		AuthIndex:        strings.TrimSpace(stringValue(item["auth_index"])),
		Email:            strings.TrimSpace(stringValue(item["email"])),
		Provider:         stringOr(stringValue(item["provider"]), stringValue(item["type"])),
		Type:             stringOr(stringValue(item["type"]), stringValue(item["provider"])),
		Account:          stringOr(stringValue(item["account"]), stringValue(item["email"])),
		Source:           strings.TrimSpace(stringValue(item["source"])),
		Status:           strings.TrimSpace(stringValue(item["status"])),
		StatusMessage:    normalizeText(stringValue(item["status_message"]), 1200),
		Disabled:         boolValueFromAny(item["disabled"]),
		Unavailable:      boolValueFromAny(item["unavailable"]),
		RuntimeOnly:      boolValueFromAny(item["runtime_only"]),
		ManagedReason:    "",
		LastAction:       "",
		LastActionStatus: "",
		LastActionError:  "",
		LastSeenAt:       timestamp,
		LastProbedAt:     "",
		UpdatedAt:        timestamp,
		ChatGPTAccountID: extractChatGPTAccountID(item),
		IDTokenPlanType:  extractIDTokenPlanType(item),
		AuthUpdatedAt:    strings.TrimSpace(stringValue(item["updated_at"])),
		AuthModTime:      strings.TrimSpace(stringValue(item["modtime"])),
		AuthLastRefresh:  strings.TrimSpace(stringValue(item["last_refresh"])),
		State:            stateUntracked,
		StateKey:         stateUntracked,
	}
	record.PlanType = record.IDTokenPlanType
	if record.Name == "" {
		record.Name = strings.TrimSpace(stringValue(item["id"]))
	}
	if previous != nil {
		record.ManagedReason = previous.ManagedReason
		record.LastAction = previous.LastAction
		record.LastActionStatus = previous.LastActionStatus
		record.LastActionError = previous.LastActionError
	}
	return record
}

func (c *Client) ProbeUsage(ctx context.Context, settings AppSettings, record AccountRecord, retryObservers ...ProbeRetryObserver) AccountRecord {
	if strings.TrimSpace(record.ChatGPTAccountID) == "" {
		record = resetProbeState(record)
		record.ProbeErrorKind = "missing_chatgpt_account_id"
		record.ProbeErrorText = msg(settings.Locale, "error.missing_chatgpt_account_id")
		return classifyAccountState(record)
	}

	attempts := settings.Retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var onRetry ProbeRetryObserver
	if len(retryObservers) > 0 {
		onRetry = retryObservers[0]
	}

	var probed AccountRecord
	for attempt := 1; attempt <= attempts; attempt++ {
		probed = c.probeUsageOnce(ctx, settings, record)
		if !shouldRetryProbeResult(probed) || attempt == attempts || ctx.Err() != nil {
			return probed
		}
		if onRetry != nil {
			onRetry(ProbeRetryEvent{
				AccountName:    record.Name,
				RetryIndex:     attempt,
				MaxRetries:     attempts - 1,
				ProbeErrorKind: probed.ProbeErrorKind,
				ProbeErrorText: probed.ProbeErrorText,
				StatusCode:     intValue(probed.APIStatusCode),
			})
		}
		if err := waitForRetry(ctx, c.retryDelay*time.Duration(attempt)); err != nil {
			return probed
		}
	}

	return probed
}

func (c *Client) probeUsageOnce(ctx context.Context, settings AppSettings, record AccountRecord) AccountRecord {
	record = resetProbeState(record)

	payload := map[string]any{
		"authIndex": record.AuthIndex,
		"method":    http.MethodGet,
		"url":       whamUsageURL,
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"Content-Type":       "application/json",
			"User-Agent":         settings.UserAgent,
			"Chatgpt-Account-Id": record.ChatGPTAccountID,
		},
	}

	body, err := c.doRequest(ctx, settings, http.MethodPost, settings.BaseURL+"/v0/management/api-call", payload)
	if err != nil {
		record.ProbeErrorKind = "management_api"
		record.ProbeErrorText = err.Error()
		return classifyAccountState(record)
	}

	statusCode, ok := intValueFromAny(body["status_code"])
	if !ok {
		record.ProbeErrorKind = "missing_status_code"
		record.ProbeErrorText = msg(settings.Locale, "error.missing_status_code")
		return classifyAccountState(record)
	}
	record.APIStatusCode = intPtr(statusCode)

	if httpStatus, ok := intValueFromAny(body["http_status"]); ok {
		record.APIHTTPStatus = intPtr(httpStatus)
	}

	rawBody := body["body"]
	parsedBody, err := toJSONObject(settings.Locale, rawBody)
	if err != nil && statusCode != http.StatusUnauthorized {
		record.ProbeErrorKind = "body_invalid_json"
		record.ProbeErrorText = err.Error()
		return classifyAccountState(record)
	}

	if statusCode == http.StatusUnauthorized {
		record.ProbeErrorKind = ""
		record.ProbeErrorText = ""
		return classifyAccountState(record)
	}

	rateLimit, _ := parsedBody["rate_limit"].(map[string]any)
	if allowed, ok := boolFromMap(rateLimit, "allowed"); ok {
		record.Allowed = boolPtr(allowed)
	}
	if limitReached, ok := boolFromMap(rateLimit, "limit_reached"); ok {
		record.LimitReached = boolPtr(limitReached)
	}
	if email := strings.TrimSpace(stringValue(parsedBody["email"])); email != "" {
		record.Email = email
	}
	if planType := strings.TrimSpace(stringValue(parsedBody["plan_type"])); planType != "" {
		record.PlanType = planType
	}

	if statusCode != http.StatusOK {
		record.ProbeErrorKind = "unexpected_status"
		record.ProbeErrorText = msg(settings.Locale, "error.unexpected_upstream_status", statusCode)
	}

	return classifyAccountState(record)
}

func resetProbeState(record AccountRecord) AccountRecord {
	record.LastProbedAt = nowISO()
	record.APIHTTPStatus = nil
	record.APIStatusCode = nil
	record.ProbeErrorKind = ""
	record.ProbeErrorText = ""
	record.Allowed = nil
	record.LimitReached = nil
	record.Error = false
	record.Invalid401 = false
	record.QuotaLimited = false
	record.Recovered = false
	return record
}

func shouldRetryProbeResult(record AccountRecord) bool {
	if record.Invalid401 || record.QuotaLimited || record.Recovered || normalizeStateKey(record.StateKey) == stateNormal {
		return false
	}

	switch record.ProbeErrorKind {
	case "management_api", "missing_status_code", "body_invalid_json":
		return true
	case "unexpected_status":
		return retryableProbeStatus(intValue(record.APIStatusCode))
	default:
		return false
	}
}

func (c *Client) DeleteAccount(ctx context.Context, settings AppSettings, name string) ActionResult {
	_, err := c.doManagedAccountRequest(ctx, settings, http.MethodDelete, settings.BaseURL+"/v0/management/auth-files", name, true, false, func(candidate string) any {
		return nil
	})
	if err != nil {
		return ActionResult{
			Name:   name,
			OK:     false,
			Action: "delete",
			Error:  err.Error(),
		}
	}
	return ActionResult{Name: name, OK: true, Action: "delete"}
}

func (c *Client) SetAccountDisabled(ctx context.Context, settings AppSettings, name string, disabled bool) ActionResult {
	body, err := c.doManagedAccountRequest(ctx, settings, http.MethodPatch, settings.BaseURL+"/v0/management/auth-files/status", name, false, true, func(candidate string) any {
		return map[string]any{
			"name":     candidate,
			"disabled": disabled,
		}
	})
	if err != nil {
		result := ActionResult{
			Name:     name,
			OK:       false,
			Action:   "toggle",
			Disabled: boolPtr(disabled),
			Error:    err.Error(),
		}
		return result
	}
	result := ActionResult{
		Name:     name,
		OK:       strings.EqualFold(stringValue(body["status"]), "ok"),
		Action:   "toggle",
		Disabled: boolPtr(disabled),
	}
	if !result.OK {
		result.Error = normalizeText(stringValue(body["error"]), 200)
	}
	return result
}

func (c *Client) doManagedAccountRequest(
	ctx context.Context,
	settings AppSettings,
	method string,
	endpoint string,
	name string,
	preferNormalized bool,
	retryAlternateName bool,
	payloadForName func(string) any,
) (map[string]any, error) {
	candidates := managedAccountNameCandidates(name, preferNormalized, retryAlternateName)
	var lastErr error
	for index, candidate := range candidates {
		requestEndpoint := endpoint
		if method == http.MethodDelete {
			requestEndpoint = endpoint + "?name=" + url.QueryEscape(candidate)
		}
		response, err := c.doRequest(ctx, settings, method, requestEndpoint, payloadForName(candidate))
		if err == nil {
			return response, nil
		}
		lastErr = err
		if index == len(candidates)-1 || !shouldRetryManagedAccountName(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func normalizeManagedAccountName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	normalizedPath := strings.ReplaceAll(trimmed, "\\", "/")
	if base := path.Base(normalizedPath); base != "." && base != "/" && base != "" {
		return base
	}
	return trimmed
}

func managedAccountNameCandidates(name string, preferNormalized bool, retryAlternateName bool) []string {
	original := strings.TrimSpace(name)
	normalized := normalizeManagedAccountName(name)
	if original == "" {
		return []string{normalized}
	}
	if normalized == "" || normalized == original {
		return []string{original}
	}
	if !retryAlternateName {
		if preferNormalized {
			return []string{normalized}
		}
		return []string{original}
	}
	if preferNormalized {
		return []string{normalized, original}
	}
	return []string{original, normalized}
}

func shouldRetryManagedAccountName(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid name") || strings.Contains(message, "auth file not found")
}

func isManagementHTTPStatus(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), fmt.Sprintf("http %d", statusCode))
}

func (c *Client) doRequest(ctx context.Context, settings AppSettings, method string, endpoint string, payload any) (map[string]any, error) {
	if strings.TrimSpace(settings.BaseURL) == "" {
		return nil, errors.New(msg(settings.Locale, "error.base_url_required"))
	}
	if strings.TrimSpace(settings.ManagementToken) == "" {
		return nil, errors.New(msg(settings.Locale, "error.management_token_required"))
	}

	attempts := settings.Retries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		response, retryable, err := c.doRequestOnce(ctx, settings, method, endpoint, payload)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if !retryable || attempt == attempts || ctx.Err() != nil {
			return nil, lastErr
		}
		if err := waitForRetry(ctx, c.retryDelay*time.Duration(attempt)); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *Client) doRequestOnce(ctx context.Context, settings AppSettings, method string, endpoint string, payload any) (map[string]any, bool, error) {
	timeout := time.Duration(settings.TimeoutSeconds)
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, false, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(requestCtx, method, endpoint, bodyReader)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+settings.ManagementToken)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, retryableHTTPStatus(resp.StatusCode), errors.New(msg(settings.Locale, "error.management_api_http", resp.StatusCode, normalizeText(string(responseBody), 300)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, false, errors.New(msg(settings.Locale, "error.response_invalid_json"))
	}
	parsed["http_status"] = resp.StatusCode
	return parsed, false, nil
}

func retryableHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func retryableProbeStatus(statusCode int) bool {
	return statusCode == http.StatusForbidden || retryableHTTPStatus(statusCode)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func classifyAccountState(record AccountRecord) AccountRecord {
	record.Invalid401 = record.Unavailable || intValue(record.APIStatusCode) == http.StatusUnauthorized
	record.QuotaLimited = !record.Invalid401 && intValue(record.APIStatusCode) == http.StatusOK && boolValue(record.LimitReached)
	record.Recovered = !record.Invalid401 &&
		!record.QuotaLimited &&
		record.Disabled &&
		record.ManagedReason == "quota_disabled" &&
		intValue(record.APIStatusCode) == http.StatusOK &&
		boolValue(record.Allowed) &&
		record.LimitReached != nil &&
		!*record.LimitReached
	record.Error = !record.Invalid401 && !record.QuotaLimited && !record.Recovered && record.ProbeErrorKind != ""

	switch {
	case record.Invalid401:
		record.StateKey = stateInvalid401
	case record.QuotaLimited:
		record.StateKey = stateQuotaLimited
	case record.Recovered:
		record.StateKey = stateRecovered
	case record.Error:
		record.StateKey = stateError
	case intValue(record.APIStatusCode) == http.StatusOK:
		record.StateKey = stateNormal
	default:
		record.StateKey = stateUntracked
	}
	record.State = record.StateKey

	record.UpdatedAt = nowISO()
	return record
}

func extractChatGPTAccountID(item map[string]any) string {
	idToken := idTokenObject(item)
	for _, source := range []map[string]any{idToken, item} {
		for _, key := range []string{"chatgpt_account_id", "chatgptAccountId", "account_id", "accountId"} {
			if value := strings.TrimSpace(stringValue(source[key])); value != "" {
				return value
			}
		}
	}
	return ""
}

func extractIDTokenPlanType(item map[string]any) string {
	idToken := idTokenObject(item)
	return strings.TrimSpace(stringValue(idToken["plan_type"]))
}

func idTokenObject(item map[string]any) map[string]any {
	return objectFromAny(item["id_token"])
}

func objectFromAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		return parseJSONString(typed)
	default:
		return map[string]any{}
	}
}

func parseJSONString(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]any{}
	}
	return parsed
}

func toJSONObject(locale string, value any) (map[string]any, error) {
	switch typed := value.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return typed, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return map[string]any{}, nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(typed), &parsed); err != nil {
			return nil, errors.New(msg(locale, "error.body_invalid_json"))
		}
		return parsed, nil
	default:
		return nil, errors.New(msg(locale, "error.body_not_object"))
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func boolValueFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func intValueFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func boolFromMap(values map[string]any, key string) (bool, bool) {
	if values == nil {
		return false, false
	}
	value, ok := values[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	default:
		return false, false
	}
}
