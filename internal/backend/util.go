package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultTargetType           = "codex"
	defaultScanStrategy         = "full"
	defaultScanBatchSize        = 1000
	defaultProbeWorkers         = 40
	defaultActionWorkers        = 20
	defaultQuotaWorkers         = 10
	defaultTimeout              = 15
	defaultRetries              = 3
	defaultQuotaAction          = "disable"
	defaultQuotaFreeMaxAccounts = 100
	defaultScheduleMode         = "scan"
	defaultScheduleCron         = "0 * * * *"
	defaultAuthImportAutoCron   = "10 * * * *"
	defaultQuotaAutoRefreshCron = "20 */2 * * *"
	defaultAuthImportArchiveDir = "CPA Imported Auth"
	defaultUserAgent            = "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal"
	defaultHistoryLimit         = 30
	whamUsageURL                = "https://chatgpt.com/backend-api/wham/usage"
)

func defaultSettings(exportDir string) AppSettings {
	return AppSettings{
		Locale:                  localeOrDefault(""),
		DetailedLogs:            false,
		TargetType:              defaultTargetType,
		ScanStrategy:            defaultScanStrategy,
		ScanBatchSize:           defaultScanBatchSize,
		SkipKnown401:            true,
		ProbeWorkers:            defaultProbeWorkers,
		ActionWorkers:           defaultActionWorkers,
		QuotaWorkers:            defaultQuotaWorkers,
		TimeoutSeconds:          defaultTimeout,
		Retries:                 defaultRetries,
		UserAgent:               defaultUserAgent,
		QuotaAction:             defaultQuotaAction,
		QuotaCheckFree:          false,
		QuotaCheckPlus:          true,
		QuotaCheckPro:           true,
		QuotaCheckTeam:          true,
		QuotaCheckBusiness:      true,
		QuotaCheckEnterprise:    true,
		QuotaFreeMaxAccounts:    defaultQuotaFreeMaxAccounts,
		QuotaAutoRefreshEnabled: false,
		QuotaAutoRefreshCron:    defaultQuotaAutoRefreshCron,
		Delete401:               true,
		AutoReenable:            true,
		ExportDirectory:         exportDir,
		AuthImport: AuthImportSettings{
			SourceDirectory:  "",
			ArchiveDirectory: defaultAuthImportArchiveDirectory(),
			MoveImported:     false,
			AutoEnabled:      false,
			AutoCron:         defaultAuthImportAutoCron,
		},
		Schedule: ScheduleSettings{
			Enabled: false,
			Mode:    defaultScheduleMode,
			Cron:    defaultScheduleCron,
		},
	}
}

func normalizeSettings(input AppSettings, exportDir string) AppSettings {
	settings := defaultSettings(exportDir)

	if trimmed := strings.TrimSpace(input.BaseURL); trimmed != "" {
		settings.BaseURL = strings.TrimRight(trimmed, "/")
	}
	if trimmed := strings.TrimSpace(input.ManagementToken); trimmed != "" {
		settings.ManagementToken = trimmed
	}
	if normalized := normalizeLocaleCode(input.Locale); normalized != "" {
		settings.Locale = normalized
	}
	settings.DetailedLogs = input.DetailedLogs
	if trimmed := strings.TrimSpace(input.TargetType); trimmed != "" {
		settings.TargetType = strings.ToLower(trimmed)
	}
	if trimmed := strings.TrimSpace(input.Provider); trimmed != "" {
		settings.Provider = strings.ToLower(trimmed)
	}
	if normalized := normalizeScanStrategy(input.ScanStrategy); normalized != "" {
		settings.ScanStrategy = normalized
	}
	if input.ScanBatchSize > 0 {
		settings.ScanBatchSize = input.ScanBatchSize
	}
	settings.SkipKnown401 = input.SkipKnown401
	if input.ProbeWorkers > 0 {
		settings.ProbeWorkers = input.ProbeWorkers
	}
	if input.ActionWorkers > 0 {
		settings.ActionWorkers = input.ActionWorkers
	}
	if input.QuotaWorkers > 0 {
		settings.QuotaWorkers = input.QuotaWorkers
	}
	if input.TimeoutSeconds > 0 {
		settings.TimeoutSeconds = input.TimeoutSeconds
	}
	if input.Retries >= 0 {
		settings.Retries = input.Retries
	}
	if trimmed := strings.TrimSpace(input.UserAgent); trimmed != "" {
		settings.UserAgent = trimmed
	}
	if trimmed := strings.ToLower(strings.TrimSpace(input.QuotaAction)); trimmed == "delete" || trimmed == "disable" {
		settings.QuotaAction = trimmed
	}
	settings.QuotaCheckFree = input.QuotaCheckFree
	settings.QuotaCheckPlus = input.QuotaCheckPlus
	settings.QuotaCheckPro = input.QuotaCheckPro
	settings.QuotaCheckTeam = input.QuotaCheckTeam
	settings.QuotaCheckBusiness = input.QuotaCheckBusiness
	settings.QuotaCheckEnterprise = input.QuotaCheckEnterprise
	if input.QuotaFreeMaxAccounts >= -1 {
		settings.QuotaFreeMaxAccounts = input.QuotaFreeMaxAccounts
	}
	settings.QuotaAutoRefreshEnabled = input.QuotaAutoRefreshEnabled
	settings.QuotaAutoRefreshCron = strings.TrimSpace(input.QuotaAutoRefreshCron)
	settings.Delete401 = input.Delete401
	settings.AutoReenable = input.AutoReenable
	if trimmed := strings.TrimSpace(input.ExportDirectory); trimmed != "" {
		settings.ExportDirectory = trimmed
	}
	if trimmed := strings.TrimSpace(input.AuthImport.SourceDirectory); trimmed != "" {
		settings.AuthImport.SourceDirectory = filepath.Clean(trimmed)
	}
	if trimmed := strings.TrimSpace(input.AuthImport.ArchiveDirectory); trimmed != "" {
		settings.AuthImport.ArchiveDirectory = filepath.Clean(trimmed)
	}
	settings.AuthImport.MoveImported = input.AuthImport.MoveImported
	settings.AuthImport.AutoEnabled = input.AuthImport.AutoEnabled
	if trimmed := strings.TrimSpace(input.AuthImport.AutoCron); trimmed != "" {
		settings.AuthImport.AutoCron = trimmed
	}
	settings.Schedule.Enabled = input.Schedule.Enabled
	if normalized := normalizeScheduleMode(input.Schedule.Mode); normalized != "" {
		settings.Schedule.Mode = normalized
	}
	settings.Schedule.Cron = strings.TrimSpace(input.Schedule.Cron)

	if settings.ExportDirectory == "" {
		settings.ExportDirectory = exportDir
	}

	return settings
}

func defaultAuthImportArchiveDirectory() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Join(os.TempDir(), defaultAuthImportArchiveDir)
	}
	return filepath.Join(homeDir, "Desktop", defaultAuthImportArchiveDir)
}

func normalizeScanStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "incremental":
		return "incremental"
	default:
		return defaultScanStrategy
	}
}

func normalizeScheduleMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "maintain":
		return "maintain"
	default:
		return defaultScheduleMode
	}
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func boolPtr(v bool) *bool {
	value := v
	return &value
}

func intPtr(v int) *int {
	value := v
	return &value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func stringOr(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeText(input string, limit int) string {
	normalized := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(input, "\r", " "), "\n", " ")), " ")
	if limit > 0 && len(normalized) > limit {
		return normalized[:limit-3] + "..."
	}
	return normalized
}

func ensureDir(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func settingsFilePath(dataDir string) string {
	return filepath.Join(dataDir, "settings.json")
}

func stateFilePath(dataDir string) string {
	return filepath.Join(dataDir, "state.db")
}

func logFilePath(dataDir string) string {
	return filepath.Join(dataDir, "app.log")
}

func defaultExportPath(exportDir string, kind string, format string) string {
	fileName := fmt.Sprintf("%s_%s.%s", kind, time.Now().Format("20060102_150405"), format)
	return filepath.Join(exportDir, fileName)
}

func matchesInventoryFilter(record AccountRecord, settings AppSettings) bool {
	if settings.TargetType != "" && !strings.EqualFold(record.Type, settings.TargetType) {
		return false
	}
	if settings.Provider != "" && !strings.EqualFold(record.Provider, settings.Provider) {
		return false
	}
	return true
}

func matchesAccountFilter(record AccountRecord, filter AccountFilter) bool {
	if filter.Type != "" && !strings.EqualFold(record.Type, filter.Type) {
		return false
	}
	if filter.Provider != "" && !strings.EqualFold(record.Provider, filter.Provider) {
		return false
	}
	if filter.State != "" && normalizeStateKey(record.StateKey) != normalizeStateKey(filter.State) {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	if query == "" {
		return true
	}
	candidates := []string{
		record.Name,
		record.Email,
		record.Provider,
		record.Type,
		record.PlanType,
		record.StatusMessage,
		record.ProbeErrorText,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func sortAccounts(records []AccountRecord) {
	sort.Slice(records, func(i, j int) bool {
		leftState := normalizeStateKey(records[i].StateKey)
		rightState := normalizeStateKey(records[j].StateKey)
		if leftState == rightState {
			return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
		}
		return statusSortOrder(leftState) < statusSortOrder(rightState)
	})
}

func useIncrementalScan(kind string, settings AppSettings) bool {
	return kind == "scan" && normalizeScanStrategy(settings.ScanStrategy) == "incremental" && settings.ScanBatchSize > 0
}

func shouldProbeCandidate(record AccountRecord, settings AppSettings) bool {
	if settings.SkipKnown401 && normalizeStateKey(record.StateKey) == stateInvalid401 {
		return false
	}
	return true
}

func inventoryFingerprintChanged(record AccountRecord, previous AccountRecord) bool {
	if record.AuthIndex != previous.AuthIndex {
		return true
	}
	if record.ChatGPTAccountID != previous.ChatGPTAccountID {
		return true
	}
	if record.Email != previous.Email {
		return true
	}
	if record.Provider != previous.Provider {
		return true
	}
	if record.Type != previous.Type {
		return true
	}
	if record.Account != previous.Account {
		return true
	}
	if record.Source != previous.Source {
		return true
	}
	if record.Disabled != previous.Disabled {
		return true
	}
	if record.Unavailable != previous.Unavailable {
		return true
	}
	if record.RuntimeOnly != previous.RuntimeOnly {
		return true
	}
	if record.AuthUpdatedAt != previous.AuthUpdatedAt {
		return true
	}
	if record.AuthModTime != previous.AuthModTime {
		return true
	}
	if record.AuthLastRefresh != previous.AuthLastRefresh {
		return true
	}
	return false
}

func markPending(record AccountRecord) AccountRecord {
	record.State = statePending
	record.StateKey = statePending
	record.LastProbedAt = ""
	record.APIHTTPStatus = nil
	record.APIStatusCode = nil
	record.ProbeErrorKind = ""
	record.ProbeErrorText = ""
	record.Allowed = nil
	record.LimitReached = nil
	record.Invalid401 = false
	record.QuotaLimited = false
	record.Recovered = false
	record.Error = false
	return sanitizeRecord(record)
}

func selectIncrementalCandidateIndexes(candidates []AccountRecord, batchSize int) []int {
	if batchSize <= 0 || len(candidates) <= batchSize {
		indexes := make([]int, len(candidates))
		for i := range candidates {
			indexes[i] = i
		}
		return indexes
	}

	indexes := make([]int, len(candidates))
	for i := range candidates {
		indexes[i] = i
	}
	sort.Slice(indexes, func(i, j int) bool {
		left := candidates[indexes[i]]
		right := candidates[indexes[j]]
		leftPending := strings.TrimSpace(left.LastProbedAt) == ""
		rightPending := strings.TrimSpace(right.LastProbedAt) == ""
		if leftPending != rightPending {
			return leftPending
		}
		if left.LastProbedAt != right.LastProbedAt {
			return left.LastProbedAt < right.LastProbedAt
		}
		leftState := normalizeStateKey(left.StateKey)
		rightState := normalizeStateKey(right.StateKey)
		if leftState != rightState {
			return statusSortOrder(leftState) < statusSortOrder(rightState)
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})

	return indexes[:batchSize]
}

func statusSortOrder(state string) int {
	switch normalizeStateKey(state) {
	case stateInvalid401:
		return 0
	case stateQuotaLimited:
		return 1
	case stateError:
		return 2
	case stateRecovered:
		return 3
	case stateNormal:
		return 4
	case statePending:
		return 5
	case stateUntracked:
		return 6
	default:
		return 7
	}
}

func settingsSummary(locale string, settings AppSettings) string {
	return msg(
		locale,
		"settings.summary",
		settings.TargetType,
		stringOr(settings.Provider, "(any)"),
		settings.ScanStrategy,
		settings.ScanBatchSize,
		settings.ProbeWorkers,
		settings.ActionWorkers,
		settings.TimeoutSeconds,
		settings.Retries,
		settings.QuotaAction,
		boolLabel(locale, settings.Delete401),
		boolLabel(locale, settings.AutoReenable),
	)
}

func validateScanSettings(locale string, strategy string, batchSize int) error {
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	if normalized != "" && normalized != "full" && normalized != "incremental" {
		return fmt.Errorf("%s", msg(locale, "error.scan_invalid_strategy"))
	}
	if normalizeScanStrategy(strategy) == "incremental" && batchSize < 1 {
		return fmt.Errorf("%s", msg(locale, "error.scan_batch_size_invalid"))
	}
	return nil
}

func marshalRecord(record AccountRecord) (string, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseRecord(data string) (AccountRecord, error) {
	var record AccountRecord
	err := json.Unmarshal([]byte(data), &record)
	if err != nil {
		return record, err
	}
	record = sanitizeRecord(record)
	return record, nil
}

func computeSummary(records []AccountRecord) DashboardSummary {
	summary := DashboardSummary{
		FilteredAccounts: len(records),
	}

	for _, record := range records {
		switch normalizeStateKey(record.StateKey) {
		case statePending:
			summary.PendingCount++
		case stateNormal:
			summary.NormalCount++
		case stateInvalid401:
			summary.Invalid401Count++
		case stateQuotaLimited:
			summary.QuotaLimitedCount++
		case stateRecovered:
			summary.RecoveredCount++
		case stateError:
			summary.ErrorCount++
		}
		if summary.LastScanAt == "" || record.LastProbedAt > summary.LastScanAt {
			summary.LastScanAt = record.LastProbedAt
		}
	}

	return summary
}

func sanitizeRecord(record AccountRecord) AccountRecord {
	record.StateKey = normalizeStateKey(stringOr(record.StateKey, record.State))
	record.State = record.StateKey
	if record.Status == "" {
		record.Status = record.StateKey
	}
	return record
}

func carryProbeSnapshot(record AccountRecord, previous AccountRecord) AccountRecord {
	record.State = previous.State
	record.StateKey = previous.StateKey
	record.Status = previous.Status
	record.StatusMessage = stringOr(record.StatusMessage, previous.StatusMessage)
	record.Allowed = previous.Allowed
	record.LimitReached = previous.LimitReached
	record.Invalid401 = previous.Invalid401
	record.QuotaLimited = previous.QuotaLimited
	record.Recovered = previous.Recovered
	record.Error = previous.Error
	record.APIHTTPStatus = previous.APIHTTPStatus
	record.APIStatusCode = previous.APIStatusCode
	record.ProbeErrorKind = previous.ProbeErrorKind
	record.ProbeErrorText = stringOr(previous.ProbeErrorText, record.ProbeErrorText)
	record.LastProbedAt = previous.LastProbedAt
	if record.PlanType == "" {
		record.PlanType = previous.PlanType
	}
	if record.Email == "" {
		record.Email = previous.Email
	}
	return sanitizeRecord(record)
}

func carryInventorySnapshot(record AccountRecord, previous *AccountRecord) AccountRecord {
	if previous == nil {
		return markPending(record)
	}
	if previous.LastProbedAt != "" {
		if inventoryFingerprintChanged(record, *previous) {
			return markPending(record)
		}
		return carryProbeSnapshot(record, *previous)
	}
	return markPending(record)
}
