package backend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientFetchProbeAndActions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/management/auth-files":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{
						"name":       "codex-quota.json",
						"type":       "codex",
						"provider":   "codex",
						"auth_index": "quota",
						"id_token":   `{"chatgpt_account_id":"acct-1","plan_type":"pro"}`,
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body": `{
					"plan_type":"pro",
					"rate_limit":{"allowed":true,"limit_reached":true}
				}`,
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/v0/management/auth-files/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodDelete && r.URL.Path == "/v0/management/auth-files":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
	}

	files, err := client.FetchAuthFiles(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchAuthFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one file, got %d", len(files))
	}

	record := client.BuildAccountRecord(files[0], nil, nowISO())
	record = client.ProbeUsage(context.Background(), settings, record)
	if record.StateKey != stateQuotaLimited {
		t.Fatalf("expected quota-limited state key, got %s", record.StateKey)
	}

	action := client.SetAccountDisabled(context.Background(), settings, record.Name, true)
	if !action.OK {
		t.Fatalf("SetAccountDisabled failed: %+v", action)
	}
	deleted := client.DeleteAccount(context.Background(), settings, record.Name)
	if !deleted.OK {
		t.Fatalf("DeleteAccount failed: %+v", deleted)
	}
}

func TestClientUploadAuthFile(t *testing.T) {
	t.Parallel()

	var uploadedName string
	var uploadedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v0/management/auth-files" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		uploadedName = r.URL.Query().Get("name")
		uploadedBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient()
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
	}

	if err := client.UploadAuthFile(context.Background(), settings, "codex/import.json", []byte(`{"type":"codex"}`)); err != nil {
		t.Fatalf("UploadAuthFile: %v", err)
	}
	if uploadedName != "codex/import.json" {
		t.Fatalf("unexpected uploaded name: %s", uploadedName)
	}
	if uploadedBody != `{"type":"codex"}` {
		t.Fatalf("unexpected uploaded body: %s", uploadedBody)
	}
}

func TestClientNormalizesManagedAccountNameForActions(t *testing.T) {
	t.Parallel()

	var patchedNames []string
	var deletedNames []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/v0/management/auth-files/status":
			var body struct {
				Name     string `json:"name"`
				Disabled bool   `json:"disabled"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			patchedNames = append(patchedNames, body.Name)
			if !strings.Contains(body.Name, "/") {
				http.Error(w, `{"error":"auth file not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodDelete && r.URL.Path == "/v0/management/auth-files":
			deletedName := r.URL.Query().Get("name")
			deletedNames = append(deletedNames, deletedName)
			if strings.Contains(deletedName, "/") {
				http.Error(w, `{"error":"invalid name"}`, http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
	}

	name := "codex/token_oc11f94baa80_dollicons.com_1773142291.json"
	action := client.SetAccountDisabled(context.Background(), settings, name, true)
	if !action.OK {
		t.Fatalf("SetAccountDisabled failed: %+v", action)
	}
	if len(patchedNames) != 1 || patchedNames[0] != "codex/token_oc11f94baa80_dollicons.com_1773142291.json" {
		t.Fatalf("expected original patch name, got %v", patchedNames)
	}

	deleted := client.DeleteAccount(context.Background(), settings, name)
	if !deleted.OK {
		t.Fatalf("DeleteAccount failed: %+v", deleted)
	}
	if len(deletedNames) != 1 || deletedNames[0] != "token_oc11f94baa80_dollicons.com_1773142291.json" {
		t.Fatalf("expected delete to use normalized name directly, got %v", deletedNames)
	}
}

func TestClientDeleteDoesNotFallbackToOriginalPathOnNotFound(t *testing.T) {
	t.Parallel()

	var deletedNames []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v0/management/auth-files" {
			http.NotFound(w, r)
			return
		}
		deletedName := r.URL.Query().Get("name")
		deletedNames = append(deletedNames, deletedName)
		http.Error(w, `{"error":"auth file not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient()
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
	}

	name := "codex/token_oc11f94baa80_dollicons.com_1773142291.json"
	deleted := client.DeleteAccount(context.Background(), settings, name)
	if deleted.OK {
		t.Fatalf("DeleteAccount unexpectedly succeeded: %+v", deleted)
	}
	if len(deletedNames) != 1 || deletedNames[0] != "token_oc11f94baa80_dollicons.com_1773142291.json" {
		t.Fatalf("expected delete to try only normalized name, got %v", deletedNames)
	}
	if !strings.Contains(strings.ToLower(deleted.Error), "auth file not found") {
		t.Fatalf("expected original not-found error, got %q", deleted.Error)
	}
}

func TestClientRetriesTransientHTTPFailure(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v0/management/auth-files" {
			http.NotFound(w, r)
			return
		}
		if atomic.AddInt32(&hits, 1) == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{}})
	}))
	defer server.Close()

	client := NewClient()
	client.retryDelay = 0
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
		Retries:         1,
	}

	files, err := client.FetchAuthFiles(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchAuthFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected zero files, got %d", len(files))
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("expected 2 attempts, got %d", hits)
	}
}

func TestClientDoesNotRetryPermanentHTTPFailure(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v0/management/auth-files" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&hits, 1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient()
	client.retryDelay = 0
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
		Retries:         3,
	}

	_, err := client.FetchAuthFiles(context.Background(), settings)
	if err == nil {
		t.Fatal("expected FetchAuthFiles to fail")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 attempt, got %d", hits)
	}
}

func TestClientProbeRetriesTransientUpstreamStatus(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
			if atomic.AddInt32(&hits, 1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status_code": 502,
					"body":        `{"error":"temporary"}`,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.retryDelay = 0
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
		Retries:         1,
		UserAgent:       defaultUserAgent,
	}

	record := AccountRecord{
		Name:             "retry-candidate.json",
		AuthIndex:        "retry",
		Type:             "codex",
		Provider:         "codex",
		ChatGPTAccountID: "acct-retry",
	}

	probed := client.ProbeUsage(context.Background(), settings, record)
	if probed.StateKey != stateNormal {
		t.Fatalf("expected normal state after retry, got %+v", probed)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("expected 2 probe attempts, got %d", hits)
	}
}

func TestClientProbeRetriesTransientForbidden(t *testing.T) {
	t.Parallel()

	var hits int32
	var retryEvents []ProbeRetryEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
			if atomic.AddInt32(&hits, 1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status_code": 403,
					"body":        `{"error":"temporary forbidden"}`,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.retryDelay = 0
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
		Retries:         1,
		UserAgent:       defaultUserAgent,
	}

	record := AccountRecord{
		Name:             "retry-403.json",
		AuthIndex:        "retry-403",
		Type:             "codex",
		Provider:         "codex",
		ChatGPTAccountID: "acct-retry-403",
	}

	probed := client.ProbeUsage(context.Background(), settings, record, func(event ProbeRetryEvent) {
		retryEvents = append(retryEvents, event)
	})
	if probed.StateKey != stateNormal {
		t.Fatalf("expected normal state after retry, got %+v", probed)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("expected 2 probe attempts, got %d", hits)
	}
	if len(retryEvents) != 1 {
		t.Fatalf("expected 1 retry event, got %d", len(retryEvents))
	}
	if retryEvents[0].RetryIndex != 1 || retryEvents[0].MaxRetries != 1 {
		t.Fatalf("unexpected retry event: %+v", retryEvents[0])
	}
	if retryEvents[0].StatusCode != http.StatusForbidden || retryEvents[0].ProbeErrorKind != "unexpected_status" {
		t.Fatalf("unexpected retry reason: %+v", retryEvents[0])
	}
}

func TestClientProbeDoesNotRetryInvalid401(t *testing.T) {
	t.Parallel()

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
			atomic.AddInt32(&hits, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 401,
				"body":        "",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.retryDelay = 0
	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TimeoutSeconds:  5,
		Retries:         3,
		UserAgent:       defaultUserAgent,
	}

	record := AccountRecord{
		Name:             "invalid-account.json",
		AuthIndex:        "invalid",
		Type:             "codex",
		Provider:         "codex",
		ChatGPTAccountID: "acct-invalid",
	}

	probed := client.ProbeUsage(context.Background(), settings, record)
	if probed.StateKey != stateInvalid401 {
		t.Fatalf("expected invalid_401 state, got %+v", probed)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 probe attempt, got %d", hits)
	}
}
