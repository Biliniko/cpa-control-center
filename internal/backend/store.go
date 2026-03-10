package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db           *sql.DB
	dataDir      string
	settingsPath string
	exportsDir   string
}

func NewStore(dataDir string) (*Store, error) {
	if err := ensureDir(dataDir); err != nil {
		return nil, err
	}

	exportsDir := filepathJoin(dataDir, "exports")
	if err := ensureDir(exportsDir); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", stateFilePath(dataDir))
	if err != nil {
		return nil, err
	}

	store := &Store{
		db:           db,
		dataDir:      dataDir,
		settingsPath: settingsFilePath(dataDir),
		exportsDir:   exportsDir,
	}

	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS accounts_current (
	name TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	account_type TEXT NOT NULL,
	state_key TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	plan_type TEXT NOT NULL DEFAULT '',
	probe_error_text TEXT NOT NULL DEFAULT '',
	disabled INTEGER NOT NULL,
	unavailable INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	last_probed_at TEXT NOT NULL,
	managed_reason TEXT NOT NULL,
	data_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_runs (
	run_id INTEGER PRIMARY KEY AUTOINCREMENT,
	status TEXT NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL,
	total_accounts INTEGER NOT NULL,
	filtered_accounts INTEGER NOT NULL,
	probed_accounts INTEGER NOT NULL,
	normal_count INTEGER NOT NULL,
	invalid_401_count INTEGER NOT NULL,
	quota_limited_count INTEGER NOT NULL,
	recovered_count INTEGER NOT NULL,
	error_count INTEGER NOT NULL,
	delete_401 INTEGER NOT NULL,
	quota_action TEXT NOT NULL,
	auto_reenable INTEGER NOT NULL,
	probe_workers INTEGER NOT NULL,
	action_workers INTEGER NOT NULL,
	timeout_seconds INTEGER NOT NULL,
	retries INTEGER NOT NULL,
	message TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_records (
	run_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	provider TEXT NOT NULL,
	account_type TEXT NOT NULL,
	state_key TEXT NOT NULL,
	data_json TEXT NOT NULL,
	PRIMARY KEY (run_id, name),
	FOREIGN KEY (run_id) REFERENCES scan_runs(run_id) ON DELETE CASCADE
);
`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	for _, migration := range []struct {
		table      string
		column     string
		definition string
	}{
		{table: "accounts_current", column: "email", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "accounts_current", column: "plan_type", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "accounts_current", column: "probe_error_text", definition: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(migration.table, migration.column, migration.definition); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ensureColumn(table string, column string, definition string) error {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition))
	return err
}

func (s *Store) LoadSettings() (AppSettings, error) {
	defaults := defaultSettings(s.exportsDir)
	data, err := os.ReadFile(s.settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return defaults, err
	}

	var raw AppSettings
	if err := json.Unmarshal(data, &raw); err != nil {
		return defaults, err
	}
	if _, ok := keys["skipKnown401"]; !ok {
		raw.SkipKnown401 = defaults.SkipKnown401
	}

	return normalizeSettings(raw, s.exportsDir), nil
}

func (s *Store) SaveSettings(input AppSettings) (AppSettings, error) {
	settings := normalizeSettings(input, s.exportsDir)
	if err := ensureDir(settings.ExportDirectory); err != nil {
		return settings, err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settings, err
	}

	if err := os.WriteFile(s.settingsPath, data, 0o644); err != nil {
		return settings, err
	}

	return settings, nil
}

func (s *Store) LoadCurrentMap() (map[string]AccountRecord, error) {
	rows, err := s.db.Query(`SELECT data_json FROM accounts_current`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make(map[string]AccountRecord)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		record, err := parseRecord(data)
		if err != nil {
			return nil, err
		}
		records[record.Name] = record
	}

	return records, rows.Err()
}

func (s *Store) ListAccounts(filter AccountFilter) ([]AccountRecord, error) {
	query, args := currentAccountsSelectQuery(filter)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]AccountRecord, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		record, err := parseRecord(data)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) ListAccountsPage(filter AccountFilter, page int, pageSize int) (AccountPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	result := AccountPage{
		Page:            page,
		PageSize:        pageSize,
		Records:         make([]AccountRecord, 0),
		ProviderOptions: make([]string, 0),
		PlanOptions:     make([]string, 0),
	}

	whereClause, whereArgs := currentAccountsWhereClause(filter)
	countQuery := `SELECT COUNT(1) FROM accounts_current` + whereClause
	if err := s.db.QueryRow(countQuery, whereArgs...).Scan(&result.TotalRecords); err != nil {
		return result, err
	}
	maxPage := 1
	if result.TotalRecords > 0 {
		maxPage = (result.TotalRecords + pageSize - 1) / pageSize
	}
	if page > maxPage {
		page = maxPage
		result.Page = page
	}

	queryArgs := append([]any{}, whereArgs...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(
		`SELECT data_json
		   FROM accounts_current`+whereClause+`
		  ORDER BY `+currentAccountsOrderByClause()+`
		  LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return result, err
		}
		record, err := parseRecord(data)
		if err != nil {
			return result, err
		}
		result.Records = append(result.Records, record)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	providerOptions, err := s.listProviderOptions(filter)
	if err != nil {
		return result, err
	}
	result.ProviderOptions = providerOptions

	planOptions, err := s.listPlanOptions(filter)
	if err != nil {
		return result, err
	}
	result.PlanOptions = planOptions

	return result, nil
}

func (s *Store) SummarizeAccounts(filter AccountFilter) (DashboardSummary, error) {
	whereClause, whereArgs := currentAccountsWhereClause(filter)
	query := `SELECT
		COUNT(1),
		COALESCE(MAX(last_probed_at), ''),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN state_key = ? THEN 1 ELSE 0 END), 0)
	FROM accounts_current` + whereClause

	args := []any{
		statePending,
		stateNormal,
		stateInvalid401,
		stateQuotaLimited,
		stateRecovered,
		stateError,
	}
	args = append(args, whereArgs...)

	var summary DashboardSummary
	if err := s.db.QueryRow(query, args...).Scan(
		&summary.FilteredAccounts,
		&summary.LastScanAt,
		&summary.PendingCount,
		&summary.NormalCount,
		&summary.Invalid401Count,
		&summary.QuotaLimitedCount,
		&summary.RecoveredCount,
		&summary.ErrorCount,
	); err != nil {
		return summary, err
	}
	return summary, nil
}

func (s *Store) CountAccounts(filter AccountFilter) (int, error) {
	whereClause, args := currentAccountsWhereClause(filter)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM accounts_current`+whereClause, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) GetCurrentAccount(name string) (AccountRecord, bool, error) {
	var data string
	err := s.db.QueryRow(`SELECT data_json FROM accounts_current WHERE name = ?`, name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecord{}, false, nil
	}
	if err != nil {
		return AccountRecord{}, false, err
	}
	record, err := parseRecord(data)
	if err != nil {
		return AccountRecord{}, false, err
	}
	return record, true, nil
}

func (s *Store) ReplaceCurrentAccounts(records []AccountRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM accounts_current`); err != nil {
		return err
	}
	for _, record := range records {
		if err := upsertCurrentAccountTx(tx, record); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) UpsertCurrentAccount(record AccountRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertCurrentAccountTx(tx, record); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteCurrentAccount(name string) error {
	_, err := s.db.Exec(`DELETE FROM accounts_current WHERE name = ?`, name)
	return err
}

func upsertCurrentAccountTx(tx *sql.Tx, record AccountRecord) error {
	record = sanitizeRecord(record)
	data, err := marshalRecord(record)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO accounts_current (
			name, provider, account_type, state_key, email, plan_type, probe_error_text, disabled, unavailable, updated_at, last_probed_at, managed_reason, data_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			provider = excluded.provider,
			account_type = excluded.account_type,
			state_key = excluded.state_key,
			email = excluded.email,
			plan_type = excluded.plan_type,
			probe_error_text = excluded.probe_error_text,
			disabled = excluded.disabled,
			unavailable = excluded.unavailable,
			updated_at = excluded.updated_at,
			last_probed_at = excluded.last_probed_at,
			managed_reason = excluded.managed_reason,
			data_json = excluded.data_json`,
		record.Name,
		record.Provider,
		record.Type,
		record.StateKey,
		record.Email,
		record.PlanType,
		record.ProbeErrorText,
		boolToInt(record.Disabled),
		boolToInt(record.Unavailable),
		record.UpdatedAt,
		record.LastProbedAt,
		record.ManagedReason,
		data,
	)
	return err
}

func (s *Store) StartScanRun(settings AppSettings) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO scan_runs (
			status, started_at, finished_at, total_accounts, filtered_accounts, probed_accounts, normal_count,
			invalid_401_count, quota_limited_count, recovered_count, error_count, delete_401, quota_action,
			auto_reenable, probe_workers, action_workers, timeout_seconds, retries, message
		) VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"running",
		nowISO(),
		"",
		boolToInt(settings.Delete401),
		settings.QuotaAction,
		boolToInt(settings.AutoReenable),
		settings.ProbeWorkers,
		settings.ActionWorkers,
		settings.TimeoutSeconds,
		settings.Retries,
		"",
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) FinishScanRun(summary ScanSummary) error {
	_, err := s.db.Exec(
		`UPDATE scan_runs SET
			status = ?,
			finished_at = ?,
			total_accounts = ?,
			filtered_accounts = ?,
			probed_accounts = ?,
			normal_count = ?,
			invalid_401_count = ?,
			quota_limited_count = ?,
			recovered_count = ?,
			error_count = ?,
			delete_401 = ?,
			quota_action = ?,
			auto_reenable = ?,
			probe_workers = ?,
			action_workers = ?,
			timeout_seconds = ?,
			retries = ?,
			message = ?
		WHERE run_id = ?`,
		summary.Status,
		summary.FinishedAt,
		summary.TotalAccounts,
		summary.FilteredAccounts,
		summary.ProbedAccounts,
		summary.NormalCount,
		summary.Invalid401Count,
		summary.QuotaLimitedCount,
		summary.RecoveredCount,
		summary.ErrorCount,
		boolToInt(summary.Delete401),
		summary.QuotaAction,
		boolToInt(summary.AutoReenable),
		summary.ProbeWorkers,
		summary.ActionWorkers,
		summary.TimeoutSeconds,
		summary.Retries,
		summary.Message,
		summary.RunID,
	)
	return err
}

func (s *Store) SaveScanRecords(runID int64, records []AccountRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM scan_records WHERE run_id = ?`, runID); err != nil {
		return err
	}

	for _, record := range records {
		record = sanitizeRecord(record)
		data, err := marshalRecord(record)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO scan_records (run_id, name, provider, account_type, state_key, data_json)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(run_id, name) DO UPDATE SET
				provider = excluded.provider,
				account_type = excluded.account_type,
				state_key = excluded.state_key,
				data_json = excluded.data_json`,
			runID,
			record.Name,
			record.Provider,
			record.Type,
			record.StateKey,
			data,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) TrimScanHistory(limit int) error {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	rows, err := s.db.Query(`SELECT run_id FROM scan_runs ORDER BY run_id DESC LIMIT -1 OFFSET ?`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var runID int64
		if err := rows.Scan(&runID); err != nil {
			return err
		}
		ids = append(ids, runID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, runID := range ids {
		if _, err := tx.Exec(`DELETE FROM scan_records WHERE run_id = ?`, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM scan_runs WHERE run_id = ?`, runID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListScanHistory(limit int) ([]ScanSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT run_id, status, started_at, finished_at, total_accounts, filtered_accounts, probed_accounts,
		        normal_count, invalid_401_count, quota_limited_count, recovered_count, error_count, delete_401,
		        quota_action, auto_reenable, probe_workers, action_workers, timeout_seconds, retries, message
		   FROM scan_runs
		  ORDER BY run_id DESC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ScanSummary, 0)
	for rows.Next() {
		var item ScanSummary
		var delete401 int
		var autoReenable int
		if err := rows.Scan(
			&item.RunID,
			&item.Status,
			&item.StartedAt,
			&item.FinishedAt,
			&item.TotalAccounts,
			&item.FilteredAccounts,
			&item.ProbedAccounts,
			&item.NormalCount,
			&item.Invalid401Count,
			&item.QuotaLimitedCount,
			&item.RecoveredCount,
			&item.ErrorCount,
			&delete401,
			&item.QuotaAction,
			&autoReenable,
			&item.ProbeWorkers,
			&item.ActionWorkers,
			&item.TimeoutSeconds,
			&item.Retries,
			&item.Message,
		); err != nil {
			return nil, err
		}
		item.Delete401 = delete401 == 1
		item.AutoReenable = autoReenable == 1
		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *Store) GetScanDetails(runID int64) (ScanDetail, error) {
	detail := ScanDetail{
		Records: make([]AccountRecord, 0),
	}
	summary, err := s.scanSummaryByRunID(runID)
	if err != nil {
		return detail, err
	}
	detail.Summary = summary

	rows, err := s.db.Query(`SELECT data_json FROM scan_records WHERE run_id = ? ORDER BY name ASC`, runID)
	if err != nil {
		return detail, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return detail, err
		}
		record, err := parseRecord(data)
		if err != nil {
			return detail, err
		}
		detail.Records = append(detail.Records, record)
	}

	return detail, rows.Err()
}

func (s *Store) GetScanDetailsPage(runID int64, page int, pageSize int) (ScanDetailPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	detail := ScanDetailPage{
		Page:     page,
		PageSize: pageSize,
		Records:  make([]AccountRecord, 0),
	}

	summary, err := s.scanSummaryByRunID(runID)
	if err != nil {
		return detail, err
	}
	detail.Summary = summary

	if err := s.db.QueryRow(`SELECT COUNT(1) FROM scan_records WHERE run_id = ?`, runID).Scan(&detail.TotalRecords); err != nil {
		return detail, err
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		`SELECT data_json
		   FROM scan_records
		  WHERE run_id = ?
		  ORDER BY name ASC
		  LIMIT ? OFFSET ?`,
		runID,
		pageSize,
		offset,
	)
	if err != nil {
		return detail, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return detail, err
		}
		record, err := parseRecord(data)
		if err != nil {
			return detail, err
		}
		detail.Records = append(detail.Records, record)
	}

	return detail, rows.Err()
}

func (s *Store) scanSummaryByRunID(runID int64) (ScanSummary, error) {
	var summary ScanSummary
	var delete401 int
	var autoReenable int

	err := s.db.QueryRow(
		`SELECT run_id, status, started_at, finished_at, total_accounts, filtered_accounts, probed_accounts,
		        normal_count, invalid_401_count, quota_limited_count, recovered_count, error_count, delete_401,
		        quota_action, auto_reenable, probe_workers, action_workers, timeout_seconds, retries, message
		   FROM scan_runs
		  WHERE run_id = ?`,
		runID,
	).Scan(
		&summary.RunID,
		&summary.Status,
		&summary.StartedAt,
		&summary.FinishedAt,
		&summary.TotalAccounts,
		&summary.FilteredAccounts,
		&summary.ProbedAccounts,
		&summary.NormalCount,
		&summary.Invalid401Count,
		&summary.QuotaLimitedCount,
		&summary.RecoveredCount,
		&summary.ErrorCount,
		&delete401,
		&summary.QuotaAction,
		&autoReenable,
		&summary.ProbeWorkers,
		&summary.ActionWorkers,
		&summary.TimeoutSeconds,
		&summary.Retries,
		&summary.Message,
	)
	if err != nil {
		return summary, err
	}
	summary.Delete401 = delete401 == 1
	summary.AutoReenable = autoReenable == 1
	return summary, nil
}

func filepathJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, part := range parts[1:] {
		path = fmt.Sprintf("%s%c%s", path, os.PathSeparator, part)
	}
	return path
}

func currentAccountsSelectQuery(filter AccountFilter) (string, []any) {
	whereClause, args := currentAccountsWhereClause(filter)
	return `SELECT data_json FROM accounts_current` + whereClause + ` ORDER BY ` + currentAccountsOrderByClause(), args
}

func currentAccountsWhereClause(filter AccountFilter) (string, []any) {
	var conditions []string
	var args []any

	if trimmed := strings.TrimSpace(filter.Type); trimmed != "" {
		conditions = append(conditions, `LOWER(account_type) = ?`)
		args = append(args, strings.ToLower(trimmed))
	}
	if trimmed := strings.TrimSpace(filter.Provider); trimmed != "" {
		conditions = append(conditions, `LOWER(provider) = ?`)
		args = append(args, strings.ToLower(trimmed))
	}
	if trimmed := strings.TrimSpace(filter.State); trimmed != "" {
		conditions = append(conditions, `state_key = ?`)
		args = append(args, normalizeStateKey(trimmed))
	}
	if trimmed := strings.ToLower(strings.TrimSpace(filter.PlanType)); trimmed != "" {
		conditions = append(conditions, `LOWER(plan_type) = ?`)
		args = append(args, trimmed)
	}
	if filter.Disabled != nil {
		conditions = append(conditions, `disabled = ?`)
		args = append(args, boolToInt(*filter.Disabled))
	}
	if trimmed := strings.ToLower(strings.TrimSpace(filter.Query)); trimmed != "" {
		pattern := "%" + trimmed + "%"
		conditions = append(conditions, `(LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(provider) LIKE ? OR LOWER(plan_type) LIKE ? OR LOWER(probe_error_text) LIKE ?)`)
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	if len(conditions) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(conditions, ` AND `), args
}

func currentAccountsOrderByClause() string {
	return `CASE state_key
		WHEN 'invalid_401' THEN 0
		WHEN 'quota_limited' THEN 1
		WHEN 'error' THEN 2
		WHEN 'recovered' THEN 3
		WHEN 'normal' THEN 4
		WHEN 'pending' THEN 5
		WHEN 'untracked' THEN 6
		ELSE 7
	END, LOWER(name) ASC`
}

func (s *Store) listProviderOptions(filter AccountFilter) ([]string, error) {
	providerFilter := filter
	providerFilter.Provider = ""

	whereClause, args := currentAccountsWhereClause(providerFilter)
	rows, err := s.db.Query(
		`SELECT DISTINCT provider
		   FROM accounts_current`+whereClause+`
		  ORDER BY LOWER(provider) ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		if strings.TrimSpace(provider) != "" {
			options = append(options, provider)
		}
	}
	return options, rows.Err()
}

func (s *Store) listPlanOptions(filter AccountFilter) ([]string, error) {
	planFilter := filter
	planFilter.PlanType = ""

	whereClause, args := currentAccountsWhereClause(planFilter)
	rows, err := s.db.Query(
		`SELECT DISTINCT plan_type
		   FROM accounts_current`+whereClause+`
		  ORDER BY LOWER(plan_type) ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []string
	for rows.Next() {
		var planType string
		if err := rows.Scan(&planType); err != nil {
			return nil, err
		}
		if strings.TrimSpace(planType) != "" {
			options = append(options, planType)
		}
	}
	return options, rows.Err()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
