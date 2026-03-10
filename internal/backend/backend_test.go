package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeCPAServer struct {
	mu        sync.Mutex
	files     []map[string]any
	deleted   []string
	disabled  []string
	reenabled []string
	fetches   int
	configHits int
	apiCalls  int
	apiAuths  []string
}

type capturedEvent struct {
	event   string
	payload any
}

type captureEmitter struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (e *captureEmitter) Emit(event string, payload any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, capturedEvent{event: event, payload: payload})
}

func (e *captureEmitter) logEntries() []capturedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	entries := make([]capturedEvent, 0, len(e.events))
	for _, item := range e.events {
		if strings.HasSuffix(item.event, ":log") {
			entries = append(entries, item)
		}
	}
	return entries
}

func (e *captureEmitter) eventsByName(name string) []capturedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	entries := make([]capturedEvent, 0, len(e.events))
	for _, item := range e.events {
		if item.event == name {
			entries = append(entries, item)
		}
	}
	return entries
}

func (f *fakeCPAServer) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v0/management/config":
		f.configHits++
		_ = json.NewEncoder(w).Encode(map[string]any{"authDir": "/tmp/auth"})
	case r.Method == http.MethodGet && r.URL.Path == "/v0/management/auth-files":
		f.fetches++
		_ = json.NewEncoder(w).Encode(map[string]any{"files": f.files})
	case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
		f.apiCalls++
		var body struct {
			AuthIndex string `json:"authIndex"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.apiAuths = append(f.apiAuths, body.AuthIndex)
		switch body.AuthIndex {
		case "invalid":
			_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 401, "body": ""})
		case "quota":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":true}}`,
			})
		case "recovered":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		}
	case r.Method == http.MethodDelete && r.URL.Path == "/v0/management/auth-files":
		name := r.URL.Query().Get("name")
		f.deleted = append(f.deleted, name)
		next := make([]map[string]any, 0, len(f.files))
		for _, item := range f.files {
			if item["name"] != name {
				next = append(next, item)
			}
		}
		f.files = next
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	case r.Method == http.MethodPatch && r.URL.Path == "/v0/management/auth-files/status":
		var body struct {
			Name     string `json:"name"`
			Disabled bool   `json:"disabled"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Disabled {
			f.disabled = append(f.disabled, body.Name)
		} else {
			f.reenabled = append(f.reenabled, body.Name)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func TestBackendTestAndSaveSettingsDoesNotSyncInventory(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "inventory-only.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-inventory","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	result, err := service.TestAndSaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("TestAndSaveSettings: %v", err)
	}
	if !result.OK || result.AccountCount != 0 {
		t.Fatalf("unexpected connection result: %+v", result)
	}

	serverState.mu.Lock()
	fetches := serverState.fetches
	configHits := serverState.configHits
	serverState.mu.Unlock()
	if configHits != 1 {
		t.Fatalf("expected exactly one config fetch, got %d", configHits)
	}
	if fetches != 0 {
		t.Fatalf("expected no auth-files fetch during test-and-save, got %d", fetches)
	}

	savedSettings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if savedSettings.BaseURL != server.URL || savedSettings.ManagementToken != "token" {
		t.Fatalf("settings were not persisted: %+v", savedSettings)
	}

	snapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot: %v", err)
	}
	if snapshot.Summary.FilteredAccounts != 0 || snapshot.Summary.PendingCount != 0 {
		t.Fatalf("unexpected dashboard snapshot after test-and-save: %+v", snapshot.Summary)
	}
}

func TestClientTestConnectionFallsBackToAuthFilesOnLegacyCPA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/management/config":
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/v0/management/auth-files":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{"name": "legacy.json"},
					{"name": "legacy-two.json"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient()
	result, err := client.TestConnection(context.Background(), AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
	})
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if !result.OK || result.AccountCount != 2 {
		t.Fatalf("unexpected connection result: %+v", result)
	}
}

func TestSyncInventoryEmitsInventoryTaskEvents(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "inventory-task.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-task","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	emitter := &captureEmitter{}
	service, err := New(dataDir, emitter)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	result, err := service.SyncInventory()
	if err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}
	if result.TotalAccounts != 1 || result.FilteredAccounts != 1 {
		t.Fatalf("unexpected sync result: %+v", result)
	}

	if len(emitter.eventsByName("inventory:progress")) == 0 {
		t.Fatal("expected inventory progress events")
	}
	finished := emitter.eventsByName("task:finished")
	if len(finished) == 0 {
		t.Fatal("expected task finished event")
	}
	lastFinished, ok := finished[len(finished)-1].payload.(TaskFinished)
	if !ok {
		t.Fatalf("unexpected finished payload type: %T", finished[len(finished)-1].payload)
	}
	if lastFinished.Kind != "inventory" || lastFinished.Status != "success" {
		t.Fatalf("unexpected finished payload: %+v", lastFinished)
	}
}

func TestBackendRunScanMaintainAndExport(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "invalid-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "invalid",
				"id_token":   `{"chatgpt_account_id":"acct-invalid","plan_type":"pro"}`,
			},
			{
				"name":       "quota-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "quota",
				"id_token":   `{"chatgpt_account_id":"acct-quota","plan_type":"pro"}`,
			},
			{
				"name":       "healthy-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-healthy","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	syncResult, err := service.SyncInventory()
	if err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}
	if syncResult.TotalAccounts != 3 || syncResult.FilteredAccounts != 3 {
		t.Fatalf("unexpected sync result: %+v", syncResult)
	}

	initialSnapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot before scan: %v", err)
	}
	if initialSnapshot.Summary.FilteredAccounts != 3 || initialSnapshot.Summary.PendingCount != 3 || initialSnapshot.Summary.LastScanAt != "" || len(initialSnapshot.History) != 0 {
		t.Fatalf("unexpected initial dashboard snapshot: %+v", initialSnapshot)
	}

	summary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if summary.Invalid401Count != 1 || summary.QuotaLimitedCount != 1 || summary.NormalCount != 1 {
		t.Fatalf("unexpected scan summary: %+v", summary)
	}

	snapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot: %v", err)
	}
	if snapshot.Summary.FilteredAccounts != 3 || snapshot.Summary.PendingCount != 0 || len(snapshot.History) != 1 {
		t.Fatalf("unexpected dashboard snapshot: %+v", snapshot)
	}

	exported, err := service.ExportAccounts("invalid401", "json", "")
	if err != nil {
		t.Fatalf("ExportAccounts: %v", err)
	}
	if exported.Exported != 1 {
		t.Fatalf("expected one exported invalid record, got %+v", exported)
	}
	if _, err := os.Stat(exported.Path); err != nil {
		t.Fatalf("expected export file: %v", err)
	}

	serverState.mu.Lock()
	serverState.files = append(serverState.files, map[string]any{
		"name":       "recovered-codex.json",
		"type":       "codex",
		"provider":   "codex",
		"auth_index": "recovered",
		"disabled":   true,
		"id_token":   `{"chatgpt_account_id":"acct-recovered","plan_type":"pro"}`,
	})
	serverState.mu.Unlock()

	storeRecord := AccountRecord{
		Name:             "recovered-codex.json",
		Type:             "codex",
		Provider:         "codex",
		State:            stateQuotaLimited,
		StateKey:         stateQuotaLimited,
		Disabled:         true,
		ManagedReason:    "quota_disabled",
		AuthIndex:        "recovered",
		ChatGPTAccountID: "acct-recovered",
		UpdatedAt:        nowISO(),
		LastSeenAt:       nowISO(),
	}
	if err := service.store.UpsertCurrentAccount(storeRecord); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	result, err := service.RunMaintain(MaintainOptions{
		Delete401:    true,
		QuotaAction:  "disable",
		AutoReenable: true,
	})
	if err != nil {
		t.Fatalf("RunMaintain: %v", err)
	}
	if len(result.Delete401Results) != 1 || len(result.QuotaActionResults) != 1 || len(result.ReenableResults) != 1 {
		t.Fatalf("unexpected maintain result: %+v", result)
	}

	records, err := service.ListAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected three remaining records, got %d", len(records))
	}

	detailPage, err := service.GetScanDetailsPage(result.Scan.RunID, 1, 2)
	if err != nil {
		t.Fatalf("GetScanDetailsPage: %v", err)
	}
	if detailPage.TotalRecords != 4 || len(detailPage.Records) != 2 {
		t.Fatalf("unexpected scan detail page: %+v", detailPage)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if len(serverState.deleted) != 1 || serverState.deleted[0] != "invalid-codex.json" {
		t.Fatalf("unexpected deleted names: %+v", serverState.deleted)
	}
	if len(serverState.disabled) != 1 || serverState.disabled[0] != "quota-codex.json" {
		t.Fatalf("unexpected disabled names: %+v", serverState.disabled)
	}
	if len(serverState.reenabled) != 1 || serverState.reenabled[0] != "recovered-codex.json" {
		t.Fatalf("unexpected reenabled names: %+v", serverState.reenabled)
	}
}

func TestBackendBatchAccountActions(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "healthy-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-healthy","plan_type":"pro"}`,
			},
			{
				"name":       "disabled-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "recovered",
				"disabled":   true,
				"id_token":   `{"chatgpt_account_id":"acct-disabled","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := service.SyncInventory(); err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}

	probeResult, err := service.ProbeAccounts([]string{"healthy-codex.json", "disabled-codex.json"})
	if err != nil {
		t.Fatalf("ProbeAccounts: %v", err)
	}
	if probeResult.Requested != 2 || probeResult.Processed != 2 || probeResult.Succeeded != 2 || probeResult.Failed != 0 || probeResult.Skipped != 0 {
		t.Fatalf("unexpected probe result: %+v", probeResult)
	}

	enableResult, err := service.SetAccountsDisabled([]string{"healthy-codex.json", "disabled-codex.json"}, false)
	if err != nil {
		t.Fatalf("SetAccountsDisabled(enable): %v", err)
	}
	if enableResult.Requested != 2 || enableResult.Processed != 1 || enableResult.Succeeded != 1 || enableResult.Failed != 0 || enableResult.Skipped != 1 {
		t.Fatalf("unexpected enable result: %+v", enableResult)
	}

	deleteResult, err := service.DeleteAccounts([]string{"healthy-codex.json", "disabled-codex.json"})
	if err != nil {
		t.Fatalf("DeleteAccounts: %v", err)
	}
	if deleteResult.Requested != 2 || deleteResult.Processed != 2 || deleteResult.Succeeded != 2 || deleteResult.Failed != 0 || deleteResult.Skipped != 0 {
		t.Fatalf("unexpected delete result: %+v", deleteResult)
	}

	records, err := service.ListAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected all records deleted, got %d", len(records))
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if len(serverState.apiAuths) != 2 {
		t.Fatalf("expected 2 probe calls, got %+v", serverState.apiAuths)
	}
	if len(serverState.reenabled) != 1 || serverState.reenabled[0] != "disabled-codex.json" {
		t.Fatalf("unexpected reenabled names: %+v", serverState.reenabled)
	}
	if len(serverState.deleted) != 2 {
		t.Fatalf("unexpected deleted names: %+v", serverState.deleted)
	}
}

func TestBackendBatchAccountActionsEmitLogs(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "healthy-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-healthy","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	emitter := &captureEmitter{}
	service, err := New(dataDir, emitter)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := service.SyncInventory(); err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}

	if _, err := service.ProbeAccounts([]string{"healthy-codex.json"}); err != nil {
		t.Fatalf("ProbeAccounts: %v", err)
	}
	if _, err := service.SetAccountsDisabled([]string{"healthy-codex.json"}, true); err != nil {
		t.Fatalf("SetAccountsDisabled: %v", err)
	}
	if _, err := service.DeleteAccounts([]string{"healthy-codex.json"}); err != nil {
		t.Fatalf("DeleteAccounts: %v", err)
	}

	entries := emitter.logEntries()
	if len(entries) == 0 {
		t.Fatal("expected emitted log entries")
	}

	var messages []string
	for _, entry := range entries {
		logEntry, ok := entry.payload.(LogEntry)
		if !ok {
			t.Fatalf("unexpected payload type for %s: %T", entry.event, entry.payload)
		}
		messages = append(messages, logEntry.Message)
	}

	expectedSnippets := []string{
		"Probed account healthy-codex.json -> Normal",
		"Probe accounts summary: requested=1, processed=1, succeeded=1, failed=0, skipped=0",
		"Set account healthy-codex.json disabled=yes",
		"Disable accounts summary: requested=1, processed=1, succeeded=1, failed=0, skipped=0",
		"Deleted account healthy-codex.json",
		"Delete accounts summary: requested=1, processed=1, succeeded=1, failed=0, skipped=0",
	}
	for _, snippet := range expectedSnippets {
		found := false
		for _, message := range messages {
			if strings.Contains(message, snippet) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected log message containing %q, got %v", snippet, messages)
		}
	}
}

func TestBackendManualActionsNormalizeManagedPathNames(t *testing.T) {
	var patchedNames []string
	var deletedNames []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/management/auth-files":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{
					{
						"name":       "codex/token_oc11f94baa80_dollicons.com_1773142291.json",
						"type":       "codex",
						"provider":   "codex",
						"auth_index": "healthy",
						"id_token":   `{"chatgpt_account_id":"acct-healthy","plan_type":"pro"}`,
					},
				},
			})
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
		case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := service.SyncInventory(); err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}

	if _, err := service.SetAccountsDisabled([]string{"codex/token_oc11f94baa80_dollicons.com_1773142291.json"}, true); err != nil {
		t.Fatalf("SetAccountsDisabled: %v", err)
	}
	if len(patchedNames) != 1 || patchedNames[0] != "codex/token_oc11f94baa80_dollicons.com_1773142291.json" {
		t.Fatalf("expected original patch name, got %v", patchedNames)
	}

	if _, err := service.DeleteAccounts([]string{"codex/token_oc11f94baa80_dollicons.com_1773142291.json"}); err != nil {
		t.Fatalf("DeleteAccounts: %v", err)
	}
	if len(deletedNames) != 1 || deletedNames[0] != "token_oc11f94baa80_dollicons.com_1773142291.json" {
		t.Fatalf("expected delete to use normalized name directly, got %v", deletedNames)
	}
}

func TestInventorySyncAndScanPreservePendingOutsideCurrentFilter(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "codex-one.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-codex","plan_type":"pro"}`,
			},
			{
				"name":       "codex-two.json",
				"type":       "codex",
				"provider":   "openai",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-openai","plan_type":"free"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		Provider:        "codex",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := service.SyncInventory(); err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "",
		Provider:        "",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	}); err != nil {
		t.Fatalf("SaveSettings widen filter after sync: %v", err)
	}

	snapshotAfterSync, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot after sync: %v", err)
	}
	if snapshotAfterSync.Summary.FilteredAccounts != 2 || snapshotAfterSync.Summary.PendingCount != 2 {
		t.Fatalf("unexpected snapshot after sync widen: %+v", snapshotAfterSync.Summary)
	}

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		Provider:        "codex",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	}); err != nil {
		t.Fatalf("SaveSettings narrow filter before scan: %v", err)
	}

	if _, err := service.RunScan(); err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "",
		Provider:        "",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	}); err != nil {
		t.Fatalf("SaveSettings widen filter after scan: %v", err)
	}

	snapshotAfterScan, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot after scan: %v", err)
	}
	if snapshotAfterScan.Summary.FilteredAccounts != 2 || snapshotAfterScan.Summary.NormalCount != 1 || snapshotAfterScan.Summary.PendingCount != 1 {
		t.Fatalf("unexpected snapshot after scan widen: %+v", snapshotAfterScan.Summary)
	}
}

func TestIncrementalScanOnlyProbesSelectedBatch(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "account-a.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy-a",
				"id_token":   `{"chatgpt_account_id":"acct-a","plan_type":"pro"}`,
			},
			{
				"name":       "account-b.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy-b",
				"id_token":   `{"chatgpt_account_id":"acct-b","plan_type":"pro"}`,
			},
			{
				"name":       "account-c.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy-c",
				"id_token":   `{"chatgpt_account_id":"acct-c","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ScanStrategy:    "incremental",
		ScanBatchSize:   2,
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if _, err := service.SyncInventory(); err != nil {
		t.Fatalf("SyncInventory: %v", err)
	}

	firstSummary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan first: %v", err)
	}
	if firstSummary.FilteredAccounts != 3 || firstSummary.ProbedAccounts != 2 {
		t.Fatalf("unexpected first incremental summary: %+v", firstSummary)
	}

	firstSnapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot first: %v", err)
	}
	if firstSnapshot.Summary.PendingCount != 1 || firstSnapshot.Summary.NormalCount != 2 {
		t.Fatalf("unexpected first incremental snapshot: %+v", firstSnapshot.Summary)
	}

	serverState.mu.Lock()
	firstAPICalls := serverState.apiCalls
	serverState.mu.Unlock()
	if firstAPICalls != 2 {
		t.Fatalf("expected 2 probe calls on first incremental scan, got %d", firstAPICalls)
	}

	secondSummary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan second: %v", err)
	}
	if secondSummary.FilteredAccounts != 3 || secondSummary.ProbedAccounts != 2 {
		t.Fatalf("unexpected second incremental summary: %+v", secondSummary)
	}

	secondSnapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot second: %v", err)
	}
	if secondSnapshot.Summary.PendingCount != 0 || secondSnapshot.Summary.NormalCount != 3 {
		t.Fatalf("unexpected second incremental snapshot: %+v", secondSnapshot.Summary)
	}

	serverState.mu.Lock()
	secondAPICalls := serverState.apiCalls
	serverState.mu.Unlock()
	if secondAPICalls != 4 {
		t.Fatalf("expected total 4 probe calls after two incremental scans, got %d", secondAPICalls)
	}
}

func TestRunScanDeduplicatesNamesAndSkipsKnownInvalid401(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "known-invalid.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "invalid",
				"id_token":   `{"chatgpt_account_id":"acct-invalid","plan_type":"pro"}`,
			},
			{
				"name":       "duplicate.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "dup",
				"id_token":   `{"chatgpt_account_id":"acct-dup","plan_type":"pro"}`,
			},
			{
				"name":       "duplicate.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "dup",
				"id_token":   `{"chatgpt_account_id":"acct-dup","plan_type":"pro"}`,
			},
			{
				"name":       "fresh.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-fresh","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		SkipKnown401:    true,
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if err := service.store.UpsertCurrentAccount(AccountRecord{
		Name:             "known-invalid.json",
		AuthIndex:        "invalid",
		Type:             "codex",
		Provider:         "codex",
		State:            stateInvalid401,
		StateKey:         stateInvalid401,
		Status:           stateInvalid401,
		StatusMessage:    "known invalid",
		ChatGPTAccountID: "acct-invalid",
		LastSeenAt:       nowISO(),
		LastProbedAt:     nowISO(),
		UpdatedAt:        nowISO(),
	}); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	summary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if summary.FilteredAccounts != 3 || summary.ProbedAccounts != 2 {
		t.Fatalf("unexpected full scan summary: %+v", summary)
	}

	snapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot: %v", err)
	}
	if snapshot.Summary.FilteredAccounts != 3 || snapshot.Summary.Invalid401Count != 1 || snapshot.Summary.NormalCount != 2 {
		t.Fatalf("unexpected snapshot after deduplicated scan: %+v", snapshot.Summary)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if serverState.apiCalls != 2 {
		t.Fatalf("expected 2 probe calls after dedupe and 401 skip, got %d", serverState.apiCalls)
	}
	dupCalls := 0
	invalidCalls := 0
	for _, auth := range serverState.apiAuths {
		switch auth {
		case "dup":
			dupCalls++
		case "invalid":
			invalidCalls++
		}
	}
	if dupCalls != 1 {
		t.Fatalf("expected duplicate auth to be probed once, got %d calls (%v)", dupCalls, serverState.apiAuths)
	}
	if invalidCalls != 0 {
		t.Fatalf("expected known invalid auth to be skipped, got %d calls (%v)", invalidCalls, serverState.apiAuths)
	}
}

func TestRunScanReprobesKnownInvalid401WhenInventoryChanges(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "known-invalid.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"updated_at": "2026-03-09T10:30:00Z",
				"id_token":   `{"chatgpt_account_id":"acct-replaced","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		SkipKnown401:    true,
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if err := service.store.UpsertCurrentAccount(AccountRecord{
		Name:             "known-invalid.json",
		AuthIndex:        "invalid",
		Type:             "codex",
		Provider:         "codex",
		State:            stateInvalid401,
		StateKey:         stateInvalid401,
		Status:           stateInvalid401,
		StatusMessage:    "known invalid",
		ChatGPTAccountID: "acct-invalid",
		AuthUpdatedAt:    "2026-03-09T09:00:00Z",
		LastSeenAt:       nowISO(),
		LastProbedAt:     nowISO(),
		UpdatedAt:        nowISO(),
	}); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	summary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if summary.FilteredAccounts != 1 || summary.ProbedAccounts != 1 || summary.NormalCount != 1 {
		t.Fatalf("unexpected scan summary after inventory change: %+v", summary)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if serverState.apiCalls != 1 {
		t.Fatalf("expected inventory change to force a reprobe, got %d calls", serverState.apiCalls)
	}
	if len(serverState.apiAuths) != 1 || serverState.apiAuths[0] != "healthy" {
		t.Fatalf("expected reprobe against the refreshed auth index, got %v", serverState.apiAuths)
	}
}

func TestIncrementalScanSkipsKnownInvalid401(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "known-invalid.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "invalid",
				"id_token":   `{"chatgpt_account_id":"acct-invalid","plan_type":"pro"}`,
			},
			{
				"name":       "pending.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-pending","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ScanStrategy:    "incremental",
		ScanBatchSize:   10,
		SkipKnown401:    true,
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	if err := service.store.UpsertCurrentAccount(AccountRecord{
		Name:             "known-invalid.json",
		AuthIndex:        "invalid",
		Type:             "codex",
		Provider:         "codex",
		State:            stateInvalid401,
		StateKey:         stateInvalid401,
		Status:           stateInvalid401,
		StatusMessage:    "known invalid",
		ChatGPTAccountID: "acct-invalid",
		LastSeenAt:       nowISO(),
		LastProbedAt:     nowISO(),
		UpdatedAt:        nowISO(),
	}); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	summary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if summary.FilteredAccounts != 2 || summary.ProbedAccounts != 1 {
		t.Fatalf("unexpected incremental scan summary: %+v", summary)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if serverState.apiCalls != 1 {
		t.Fatalf("expected 1 probe call during incremental scan, got %d", serverState.apiCalls)
	}
	for _, auth := range serverState.apiAuths {
		if auth == "invalid" {
			t.Fatalf("expected known invalid auth to be skipped in incremental scan, got calls %v", serverState.apiAuths)
		}
	}
}

func TestRunScanCanProbeKnownInvalid401WhenSkipDisabled(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "known-invalid.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "invalid",
				"id_token":   `{"chatgpt_account_id":"acct-invalid","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ScanStrategy:    "full",
		SkipKnown401:    false,
		ProbeWorkers:    4,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	existing := AccountRecord{
		Name:             "known-invalid.json",
		Type:             "codex",
		Provider:         "codex",
		AuthIndex:        "invalid",
		ChatGPTAccountID: "acct-invalid",
		State:            stateInvalid401,
		StateKey:         stateInvalid401,
		LastProbedAt:     nowISO(),
		UpdatedAt:        nowISO(),
	}
	if err := service.store.UpsertCurrentAccount(existing); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	if _, err := service.RunScan(); err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	invalidCalls := 0
	for _, auth := range serverState.apiAuths {
		if auth == "invalid" {
			invalidCalls++
		}
	}
	if invalidCalls != 1 {
		t.Fatalf("expected known invalid auth to be probed when skipKnown401 is disabled, got %d calls (%v)", invalidCalls, serverState.apiAuths)
	}
}

func TestSchedulerStatusValidationAndScheduledScan(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "scheduled-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-scheduled","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	settings := AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
		Schedule: ScheduleSettings{
			Enabled: true,
			Mode:    "scan",
			Cron:    "*/15 * * * *",
		},
	}

	if _, err := service.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	status := service.GetSchedulerStatus()
	if !status.Enabled || !status.Valid || status.Mode != "scan" || status.NextRunAt == "" {
		t.Fatalf("unexpected scheduler status after save: %+v", status)
	}

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
		Schedule: ScheduleSettings{
			Enabled: true,
			Mode:    "scan",
			Cron:    "not-a-cron",
		},
	}); err == nil {
		t.Fatal("expected invalid cron save to fail")
	}

	service.scheduler.execute(service.scheduler.version, "scan", "*/15 * * * *")

	updatedStatus := service.GetSchedulerStatus()
	if updatedStatus.LastStatus != "success" || updatedStatus.LastStartedAt == "" || updatedStatus.LastFinishedAt == "" {
		t.Fatalf("unexpected scheduler runtime status: %+v", updatedStatus)
	}

	snapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot: %v", err)
	}
	if snapshot.Summary.FilteredAccounts != 1 || snapshot.Summary.NormalCount != 1 || len(snapshot.History) != 1 {
		t.Fatalf("unexpected snapshot after scheduled scan: %+v", snapshot)
	}
}

func TestScheduledTaskSkipsWhenAnotherTaskIsRunning(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "scheduled-maintain.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-maintain","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	if _, err := service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    2,
		ActionWorkers:   1,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		ExportDirectory: filepath.Join(dataDir, "exports"),
		Schedule: ScheduleSettings{
			Enabled: true,
			Mode:    "maintain",
			Cron:    "0 * * * *",
		},
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	ctx, err := service.beginTask("scan", localeEnglish)
	if err != nil {
		t.Fatalf("beginTask: %v", err)
	}
	defer func() {
		if ctx != nil {
			service.endTask()
		}
	}()

	service.scheduler.execute(service.scheduler.version, "maintain", "0 * * * *")

	status := service.GetSchedulerStatus()
	if status.LastStatus != "skipped" {
		t.Fatalf("expected skipped scheduler status, got %+v", status)
	}
	if len(serverState.deleted) != 0 || len(serverState.disabled) != 0 || len(serverState.reenabled) != 0 {
		t.Fatalf("scheduled task should not have executed actions: deleted=%v disabled=%v reenabled=%v", serverState.deleted, serverState.disabled, serverState.reenabled)
	}

	ctx = nil
	service.endTask()
}
