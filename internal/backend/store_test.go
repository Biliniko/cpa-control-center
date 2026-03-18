package backend

import (
	"os"
	"testing"
)

func TestStoreSettingsAndHistory(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	settings, err := store.SaveSettings(AppSettings{
		BaseURL:         "https://example.com",
		ManagementToken: "token",
		Locale:          localeEnglish,
		DetailedLogs:    true,
		TargetType:      "codex",
		ProbeWorkers:    12,
		ActionWorkers:   6,
		TimeoutSeconds:  10,
		Retries:         2,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: store.exportsDir,
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if settings.BaseURL != "https://example.com" {
		t.Fatalf("unexpected BaseURL: %s", settings.BaseURL)
	}

	loaded, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if loaded.ManagementToken != "token" {
		t.Fatalf("unexpected token: %s", loaded.ManagementToken)
	}
	if !loaded.DetailedLogs {
		t.Fatalf("expected detailed logs to persist")
	}

	records := []AccountRecord{
		{
			Name:      "codex-1.json",
			Type:      "codex",
			Provider:  "codex",
			Email:     "one@example.com",
			PlanType:  "pro",
			State:     stateNormal,
			StateKey:  stateNormal,
			UpdatedAt: nowISO(),
		},
		{
			Name:           "codex-2.json",
			Type:           "codex",
			Provider:       "codex",
			Email:          "two@example.com",
			PlanType:       "free",
			Disabled:       true,
			ProbeErrorText: "timeout",
			State:          stateError,
			StateKey:       stateError,
			UpdatedAt:      nowISO(),
		},
		{
			Name:      "other-1.json",
			Type:      "chatgpt",
			Provider:  "other",
			Email:     "other@example.com",
			State:     statePending,
			StateKey:  statePending,
			UpdatedAt: nowISO(),
		},
	}
	if err := store.ReplaceCurrentAccounts(records); err != nil {
		t.Fatalf("ReplaceCurrentAccounts: %v", err)
	}

	items, err := store.ListAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(items) != 2 || items[0].Name != "codex-2.json" || items[1].Name != "codex-1.json" {
		t.Fatalf("unexpected accounts: %+v", items)
	}

	page, err := store.ListAccountsPage(AccountFilter{Type: "codex", Query: "example"}, 1, 1)
	if err != nil {
		t.Fatalf("ListAccountsPage: %v", err)
	}
	if page.TotalRecords != 2 || len(page.Records) != 1 || len(page.ProviderOptions) != 1 || page.ProviderOptions[0] != "codex" {
		t.Fatalf("unexpected account page: %+v", page)
	}
	if len(page.PlanOptions) != 2 || page.PlanOptions[0] != "free" || page.PlanOptions[1] != "pro" {
		t.Fatalf("unexpected plan options: %+v", page.PlanOptions)
	}

	disabled := true
	disabledItems, err := store.ListAccounts(AccountFilter{Type: "codex", Provider: "codex", Disabled: &disabled})
	if err != nil {
		t.Fatalf("ListAccounts disabled filter: %v", err)
	}
	if len(disabledItems) != 1 || disabledItems[0].Name != "codex-2.json" {
		t.Fatalf("unexpected disabled filter result: %+v", disabledItems)
	}

	proPage, err := store.ListAccountsPage(AccountFilter{Type: "codex", Provider: "codex", PlanType: "pro"}, 1, 10)
	if err != nil {
		t.Fatalf("ListAccountsPage plan filter: %v", err)
	}
	if proPage.TotalRecords != 1 || len(proPage.Records) != 1 || proPage.Records[0].Name != "codex-1.json" {
		t.Fatalf("unexpected plan filter page: %+v", proPage)
	}
	if len(proPage.PlanOptions) != 2 || proPage.PlanOptions[0] != "free" || proPage.PlanOptions[1] != "pro" {
		t.Fatalf("unexpected plan options for filtered page: %+v", proPage.PlanOptions)
	}

	summarySnapshot, err := store.SummarizeAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("SummarizeAccounts: %v", err)
	}
	if summarySnapshot.FilteredAccounts != 2 || summarySnapshot.NormalCount != 1 || summarySnapshot.ErrorCount != 1 || summarySnapshot.PendingCount != 0 {
		t.Fatalf("unexpected account summary: %+v", summarySnapshot)
	}

	runID, err := store.StartScanRun(loaded)
	if err != nil {
		t.Fatalf("StartScanRun: %v", err)
	}

	summary := ScanSummary{
		RunID:             runID,
		Status:            "success",
		StartedAt:         nowISO(),
		FinishedAt:        nowISO(),
		TotalAccounts:     1,
		FilteredAccounts:  1,
		ProbedAccounts:    1,
		NormalCount:       1,
		Invalid401Count:   0,
		QuotaLimitedCount: 0,
		RecoveredCount:    0,
		ErrorCount:        0,
		Delete401:         true,
		QuotaAction:       "disable",
		AutoReenable:      true,
		ProbeWorkers:      12,
		ActionWorkers:     6,
		TimeoutSeconds:    10,
		Retries:           2,
		Message:           "ok",
	}
	if err := store.FinishScanRun(summary); err != nil {
		t.Fatalf("FinishScanRun: %v", err)
	}
	if err := store.SaveScanRecords(runID, []AccountRecord{records[0]}); err != nil {
		t.Fatalf("SaveScanRecords: %v", err)
	}

	history, err := store.ListScanHistory(5)
	if err != nil {
		t.Fatalf("ListScanHistory: %v", err)
	}
	if len(history) != 1 || history[0].RunID != runID {
		t.Fatalf("unexpected history: %+v", history)
	}

	detail, err := store.GetScanDetails(runID)
	if err != nil {
		t.Fatalf("GetScanDetails: %v", err)
	}
	if len(detail.Records) != 1 || detail.Records[0].Name != records[0].Name {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	paged, err := store.GetScanDetailsPage(runID, 1, 1)
	if err != nil {
		t.Fatalf("GetScanDetailsPage: %v", err)
	}
	if paged.TotalRecords != 1 || len(paged.Records) != 1 || paged.Records[0].Name != records[0].Name {
		t.Fatalf("unexpected paged detail: %+v", paged)
	}
}

func TestSaveScanRecordsHandlesDuplicateNamesWithinRun(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	runID, err := store.StartScanRun(defaultSettings(store.exportsDir))
	if err != nil {
		t.Fatalf("StartScanRun: %v", err)
	}

	first := AccountRecord{
		Name:      "duplicate.json",
		Type:      "codex",
		Provider:  "codex",
		State:     statePending,
		StateKey:  statePending,
		UpdatedAt: nowISO(),
	}
	second := first
	second.State = stateNormal
	second.StateKey = stateNormal
	second.StatusMessage = "latest"
	second.UpdatedAt = nowISO()

	if err := store.SaveScanRecords(runID, []AccountRecord{first, second}); err != nil {
		t.Fatalf("SaveScanRecords with duplicate names: %v", err)
	}

	detail, err := store.GetScanDetails(runID)
	if err != nil {
		t.Fatalf("GetScanDetails: %v", err)
	}
	if len(detail.Records) != 1 {
		t.Fatalf("expected 1 deduplicated record, got %d", len(detail.Records))
	}
	if detail.Records[0].StateKey != stateNormal {
		t.Fatalf("expected latest record state %q, got %q", stateNormal, detail.Records[0].StateKey)
	}
	if detail.Records[0].StatusMessage != "latest" {
		t.Fatalf("expected latest record payload to win, got %+v", detail.Records[0])
	}
}

func TestLoadSettingsDefaultsSkipKnown401WhenMissingFromLegacyFile(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	legacy := `{
  "baseUrl": "https://example.com",
  "managementToken": "token",
  "locale": "en-US",
  "detailedLogs": false,
  "targetType": "codex",
  "provider": "",
  "scanStrategy": "full",
  "scanBatchSize": 1000,
  "probeWorkers": 40,
  "actionWorkers": 20,
  "timeoutSeconds": 15,
  "retries": 3,
  "userAgent": "ua",
  "quotaAction": "disable",
  "delete401": true,
  "autoReenable": true,
  "exportDirectory": ""
}`
	if err := os.WriteFile(store.settingsPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile settings.json: %v", err)
	}

	settings, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if !settings.SkipKnown401 {
		t.Fatalf("expected legacy settings to default skipKnown401 to true")
	}
}

func TestStoreCodexQuotaSnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	snapshot := CodexQuotaSnapshot{
		Source:          "scan",
		Coverage:        "partial",
		CoveredAccounts: 2,
		FetchedAt:       nowISO(),
		TotalAccounts:   3,
		Accounts: []CodexQuotaAccountDetail{
			{
				Name:      "codex-one.json",
				PlanType:  "pro",
				Provider:  "codex",
				Success:   true,
				FetchedAt: nowISO(),
			},
		},
	}
	if err := store.SaveCodexQuotaSnapshot(snapshot); err != nil {
		t.Fatalf("SaveCodexQuotaSnapshot: %v", err)
	}

	loaded, ok, err := store.LoadCodexQuotaSnapshot()
	if err != nil {
		t.Fatalf("LoadCodexQuotaSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected saved codex quota snapshot to exist")
	}
	if loaded.Source != snapshot.Source || loaded.Coverage != snapshot.Coverage || loaded.CoveredAccounts != snapshot.CoveredAccounts {
		t.Fatalf("unexpected loaded snapshot metadata: %+v", loaded)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0].Name != snapshot.Accounts[0].Name {
		t.Fatalf("unexpected loaded snapshot accounts: %+v", loaded.Accounts)
	}
}
