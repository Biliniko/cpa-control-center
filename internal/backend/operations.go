package backend

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"sync/atomic"
)

func (b *Backend) RunScan() (ScanSummary, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return ScanSummary{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return ScanSummary{}, err
	}

	ctx, err := b.beginTask("scan", settings.Locale)
	if err != nil {
		return ScanSummary{}, err
	}
	defer b.endTask()

	summary, _, _, err := b.runScan(ctx, "scan", settings)
	status := summary.Status
	if status == "" {
		status = taskStatus(err)
	}
	b.emitTaskFinished("scan", status, summary.Message)
	return summary, err
}

func (b *Backend) RunMaintain(options MaintainOptions) (MaintainResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return MaintainResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return MaintainResult{}, err
	}

	settings.Delete401 = options.Delete401
	if options.QuotaAction == "disable" || options.QuotaAction == "delete" {
		settings.QuotaAction = options.QuotaAction
	}
	settings.AutoReenable = options.AutoReenable

	ctx, err := b.beginTask("maintain", settings.Locale)
	if err != nil {
		return MaintainResult{}, err
	}
	defer b.endTask()

	result, err := b.runMaintain(ctx, settings)
	status := taskStatus(err)
	if err == nil {
		status = "success"
	}
	b.emitTaskFinished("maintain", status, result.Scan.Message)
	return result, err
}

func accountActionLabel(locale string, action string) string {
	switch action {
	case "probe":
		return msg(locale, "task.action.probe")
	case "delete":
		return msg(locale, "task.action.delete")
	case "disable":
		return msg(locale, "task.action.disable")
	case "enable":
		return msg(locale, "task.action.enable")
	default:
		return action
	}
}

func accountActionLogKind(action string) string {
	if action == "probe" {
		return "scan"
	}
	return "maintain"
}

func (b *Backend) emitAccountActionFailure(locale string, action string, name string, err error) {
	if err == nil {
		return
	}
	b.emitLog(accountActionLogKind(action), "error", msg(locale, "task.action.failed", accountActionLabel(locale, action), name, err.Error()))
}

func (b *Backend) emitAccountBatchSummary(locale string, action string, result BulkAccountActionResult) {
	level := "info"
	if result.Failed > 0 {
		level = "warning"
	}
	b.emitLog(accountActionLogKind(action), level, msg(
		locale,
		"task.account.batch_summary",
		accountActionLabel(locale, action),
		result.Requested,
		result.Processed,
		result.Succeeded,
		result.Failed,
		result.Skipped,
	))
}

func (b *Backend) ProbeAccount(name string) (AccountRecord, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return AccountRecord{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return AccountRecord{}, err
	}

	record, ok, err := b.store.GetCurrentAccount(name)
	if err != nil {
		b.emitAccountActionFailure(settings.Locale, "probe", name, err)
		return AccountRecord{}, err
	}
	if !ok {
		err = errors.New(msg(settings.Locale, "error.account_not_found", name))
		b.emitAccountActionFailure(settings.Locale, "probe", name, err)
		return AccountRecord{}, err
	}

	probed := b.client.ProbeUsage(context.Background(), settings, record, b.probeRetryLogger(settings.DetailedLogs, "scan", settings.Locale))
	if err := b.store.UpsertCurrentAccount(probed); err != nil {
		b.emitAccountActionFailure(settings.Locale, "probe", name, err)
		return AccountRecord{}, err
	}
	b.emitAccountUpdate("probe", false, probed)
	b.emitLog("scan", "info", msg(settings.Locale, "task.scan.single_probe", probed.Name, stateLabel(settings.Locale, probed.StateKey)))
	return probed, nil
}

func (b *Backend) ProbeAccounts(names []string) (BulkAccountActionResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return BulkAccountActionResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return BulkAccountActionResult{}, err
	}

	result := BulkAccountActionResult{
		Action:    "probe",
		Requested: len(names),
		Results:   make([]ActionResult, 0, len(names)),
	}

	for _, name := range names {
		result.Processed++
		record, err := b.ProbeAccount(name)
		if err != nil {
			result.Failed++
			result.Results = append(result.Results, ActionResult{
				Name:   name,
				OK:     false,
				Action: "probe",
				Error:  err.Error(),
			})
			continue
		}

		result.Succeeded++
		result.Results = append(result.Results, ActionResult{
			Name:       name,
			OK:         true,
			Action:     "probe",
			StatusCode: record.APIStatusCode,
		})
	}

	b.emitAccountBatchSummary(settings.Locale, "probe", result)
	return result, nil
}

func (b *Backend) SetAccountDisabled(name string, disabled bool) (ActionResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return ActionResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return ActionResult{}, err
	}
	action := "disable"
	if !disabled {
		action = "enable"
	}

	result := b.client.SetAccountDisabled(context.Background(), settings, name, disabled)
	if !result.OK {
		err = errors.New(result.Error)
		b.emitAccountActionFailure(settings.Locale, action, name, err)
		return result, err
	}

	record, ok, err := b.store.GetCurrentAccount(name)
	if err != nil {
		b.emitAccountActionFailure(settings.Locale, action, name, err)
		return result, err
	}
	if ok {
		record.Disabled = disabled
		record.LastAction = "manual_toggle"
		record.LastActionStatus = "success"
		record.LastActionError = ""
		if disabled {
			record.ManagedReason = "manual_disabled"
		} else if record.ManagedReason == "manual_disabled" {
			record.ManagedReason = ""
		}
		record.UpdatedAt = nowISO()
		if err := b.store.UpsertCurrentAccount(record); err != nil {
			b.emitAccountActionFailure(settings.Locale, action, name, err)
			return result, err
		}
		b.emitAccountUpdate("manual_toggle", false, record)
	}

	b.emitLog("maintain", "info", msg(settings.Locale, "task.account.set_disabled", name, boolLabel(settings.Locale, disabled)))
	return result, nil
}

func (b *Backend) SetAccountsDisabled(names []string, disabled bool) (BulkAccountActionResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return BulkAccountActionResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return BulkAccountActionResult{}, err
	}

	action := "disable"
	if !disabled {
		action = "enable"
	}
	result := BulkAccountActionResult{
		Action:    action,
		Requested: len(names),
		Results:   make([]ActionResult, 0, len(names)),
	}

	for _, name := range names {
		record, ok, err := b.store.GetCurrentAccount(name)
		if err != nil {
			b.emitAccountActionFailure(settings.Locale, action, name, err)
			result.Processed++
			result.Failed++
			result.Results = append(result.Results, ActionResult{
				Name:   name,
				OK:     false,
				Action: action,
				Error:  err.Error(),
			})
			continue
		}
		if !ok {
			err = errors.New(msg(settings.Locale, "error.account_not_found", name))
			b.emitAccountActionFailure(settings.Locale, action, name, err)
			result.Processed++
			result.Failed++
			result.Results = append(result.Results, ActionResult{
				Name:   name,
				OK:     false,
				Action: action,
				Error:  err.Error(),
			})
			continue
		}
		if record.Disabled == disabled {
			result.Skipped++
			disabledValue := record.Disabled
			result.Results = append(result.Results, ActionResult{
				Name:     name,
				OK:       true,
				Action:   action,
				Disabled: &disabledValue,
			})
			continue
		}

		result.Processed++
		actionResult, err := b.SetAccountDisabled(name, disabled)
		if err != nil {
			result.Failed++
			result.Results = append(result.Results, ActionResult{
				Name:     name,
				OK:       false,
				Action:   action,
				Disabled: actionResult.Disabled,
				Error:    err.Error(),
			})
			continue
		}
		result.Succeeded++
		result.Results = append(result.Results, actionResult)
	}

	b.emitAccountBatchSummary(settings.Locale, action, result)
	return result, nil
}

func (b *Backend) DeleteAccount(name string) (ActionResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return ActionResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return ActionResult{}, err
	}

	record, ok, err := b.store.GetCurrentAccount(name)
	if err != nil {
		b.emitAccountActionFailure(settings.Locale, "delete", name, err)
		return ActionResult{}, err
	}
	if !ok {
		err = errors.New(msg(settings.Locale, "error.account_not_found", name))
		result := ActionResult{Name: name, OK: false, Action: "delete", Error: err.Error()}
		b.emitAccountActionFailure(settings.Locale, "delete", name, err)
		return result, err
	}

	result := b.client.DeleteAccount(context.Background(), settings, name)
	if !result.OK {
		err = errors.New(result.Error)
		b.emitAccountActionFailure(settings.Locale, "delete", name, err)
		return result, err
	}

	if err := b.store.DeleteCurrentAccount(name); err != nil {
		b.emitAccountActionFailure(settings.Locale, "delete", name, err)
		return result, err
	}
	b.emitAccountUpdate("manual_delete", true, record)
	b.emitLog("maintain", "info", msg(settings.Locale, "task.account.deleted", name))
	return result, nil
}

func (b *Backend) DeleteAccounts(names []string) (BulkAccountActionResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return BulkAccountActionResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return BulkAccountActionResult{}, err
	}

	result := BulkAccountActionResult{
		Action:    "delete",
		Requested: len(names),
		Results:   make([]ActionResult, 0, len(names)),
	}

	for _, name := range names {
		result.Processed++
		actionResult, err := b.DeleteAccount(name)
		if err != nil {
			result.Failed++
			result.Results = append(result.Results, ActionResult{
				Name:   name,
				OK:     false,
				Action: "delete",
				Error:  err.Error(),
			})
			continue
		}
		result.Succeeded++
		result.Results = append(result.Results, actionResult)
	}

	b.emitAccountBatchSummary(settings.Locale, "delete", result)
	return result, nil
}

func (b *Backend) ExportAccounts(kind string, format string, path string) (ExportResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return ExportResult{}, err
	}

	records, err := b.store.ListAccounts(AccountFilter{
		Type:     settings.TargetType,
		Provider: settings.Provider,
	})
	if err != nil {
		return ExportResult{}, err
	}

	selected := filterAccountsForExport(records, kind)
	if path == "" {
		path = defaultExportPath(settings.ExportDirectory, kind, format)
	}
	if err := ensureDir(filepathDir(path)); err != nil {
		return ExportResult{}, err
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(selected, "", "  ")
		if err != nil {
			return ExportResult{}, err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return ExportResult{}, err
		}
	case "csv":
		if err := writeCSV(path, settings.Locale, selected); err != nil {
			return ExportResult{}, err
		}
	default:
		return ExportResult{}, errors.New(msg(settings.Locale, "error.unsupported_export_format", format))
	}

	b.emitLog("maintain", "info", msg(settings.Locale, "task.export.completed", len(selected), exportKindLabel(settings.Locale, kind), path))
	return ExportResult{
		Kind:     kind,
		Format:   format,
		Path:     path,
		Exported: len(selected),
	}, nil
}

func (b *Backend) ListScanHistory(limit int) ([]ScanSummary, error) {
	return b.store.ListScanHistory(limit)
}

func (b *Backend) GetScanDetails(runID int64) (ScanDetail, error) {
	return b.store.GetScanDetails(runID)
}

func (b *Backend) GetScanDetailsPage(runID int64, page int, pageSize int) (ScanDetailPage, error) {
	return b.store.GetScanDetailsPage(runID, page, pageSize)
}

func (b *Backend) runScan(ctx context.Context, kind string, settings AppSettings) (ScanSummary, []AccountRecord, []UsageProbeResult, error) {
	runID, err := b.store.StartScanRun(settings)
	if err != nil {
		return ScanSummary{}, nil, nil, err
	}

	summary := ScanSummary{
		RunID:          runID,
		Status:         "running",
		StartedAt:      nowISO(),
		Delete401:      settings.Delete401,
		QuotaAction:    settings.QuotaAction,
		AutoReenable:   settings.AutoReenable,
		ProbeWorkers:   settings.ProbeWorkers,
		ActionWorkers:  settings.ActionWorkers,
		TimeoutSeconds: settings.TimeoutSeconds,
		Retries:        settings.Retries,
	}

	var records []AccountRecord
	defer func() {
		summary.FinishedAt = nowISO()
		_ = b.store.FinishScanRun(summary)
	}()

	b.emitLog(kind, "info", msg(settings.Locale, "task.scan.started", taskName(settings.Locale, kind), settingsSummary(settings.Locale, settings)))
	b.emitProgress(kind, "fetch", 0, 1, msg(settings.Locale, "task.scan.loading_inventory"), false)

	files, err := b.client.FetchAuthFiles(ctx, settings)
	if err != nil {
		summary.Status = taskStatus(err)
		summary.Message = err.Error()
		b.emitLog(kind, "error", msg(settings.Locale, "task.scan.failed_auth_files", err))
		b.emitProgress(kind, "fetch", 0, 1, summary.Message, true)
		return summary, nil, nil, err
	}
	b.emitProgress(kind, "fetch", 1, 1, msg(settings.Locale, "task.scan.loaded_auth_files", len(files)), true)

	existing, err := b.store.LoadCurrentMap()
	if err != nil {
		summary.Status = "failed"
		summary.Message = err.Error()
		return summary, nil, nil, err
	}

	timestamp := nowISO()
	probeCandidates := make([]AccountRecord, 0, len(files))
	probeCandidateIndexes := make([]int, 0, len(files))
	recordIndexes := make(map[string]int, len(files))
	records = make([]AccountRecord, 0, len(files))

	for _, item := range files {
		name := stringValue(item["name"])
		if name == "" {
			continue
		}
		var previous *AccountRecord
		if current, ok := existing[name]; ok {
			currentCopy := current
			previous = &currentCopy
		}
		record := b.client.BuildAccountRecord(item, previous, timestamp)
		record = carryInventorySnapshot(record, previous)
		if index, ok := recordIndexes[name]; ok {
			records[index] = record
			continue
		}
		recordIndexes[name] = len(records)
		records = append(records, record)
	}

	filteredAccounts := 0
	for index, record := range records {
		if !matchesInventoryFilter(record, settings) {
			continue
		}
		filteredAccounts++
		if !shouldProbeCandidate(record, settings) {
			continue
		}
		probeCandidateIndexes = append(probeCandidateIndexes, index)
		probeCandidates = append(probeCandidates, record)
	}

	summary.TotalAccounts = len(records)
	summary.FilteredAccounts = filteredAccounts
	b.emitLog(kind, "info", msg(settings.Locale, "task.scan.prepared_candidates", len(probeCandidates), len(records)))

	selectedCandidates := probeCandidates
	selectedIndexes := probeCandidateIndexes
	if useIncrementalScan(kind, settings) && len(probeCandidates) > settings.ScanBatchSize {
		selected := selectIncrementalCandidateIndexes(probeCandidates, settings.ScanBatchSize)
		selectedCandidates = make([]AccountRecord, 0, len(selected))
		selectedIndexes = make([]int, 0, len(selected))
		for _, idx := range selected {
			selectedCandidates = append(selectedCandidates, probeCandidates[idx])
			selectedIndexes = append(selectedIndexes, probeCandidateIndexes[idx])
		}
		b.emitLog(kind, "info", msg(settings.Locale, "task.scan.incremental_selected", len(selectedCandidates), len(probeCandidates), settings.ScanBatchSize))
	}

	probed, err := b.probeAccounts(ctx, kind, settings, selectedCandidates)
	if err != nil {
		summary.Status = taskStatus(err)
		summary.Message = err.Error()
		b.emitLog(kind, "warning", msg(settings.Locale, "task.scan.stopped", taskName(settings.Locale, kind), err))
		return summary, nil, nil, err
	}

	for i, index := range selectedIndexes {
		records[index] = probed[i].Record
	}

	if err := b.store.ReplaceCurrentAccounts(records); err != nil {
		summary.Status = "failed"
		summary.Message = err.Error()
		return summary, nil, nil, err
	}
	if kind == "scan" {
		quotaSnapshot, ok, err := b.buildCodexQuotaSnapshotFromUsageProbes(settings, records, probed, nowISO(), "scan")
		if err != nil {
			summary.Status = "failed"
			summary.Message = err.Error()
			return summary, nil, nil, err
		}
		if ok {
			if err := b.persistCodexQuotaSnapshot(quotaSnapshot); err != nil {
				summary.Status = "failed"
				summary.Message = err.Error()
				return summary, nil, nil, err
			}
		}
	}
	if err := b.store.SaveScanRecords(runID, records); err != nil {
		summary.Status = "failed"
		summary.Message = err.Error()
		return summary, nil, nil, err
	}
	if err := b.store.TrimScanHistory(defaultHistoryLimit); err != nil {
		summary.Status = "failed"
		summary.Message = err.Error()
		return summary, nil, nil, err
	}

	filtered := filterAccountsBySettings(records, settings)
	dashboard := computeSummary(filtered)
	summary.Status = "success"
	summary.ProbedAccounts = len(selectedCandidates)
	if len(selectedCandidates) < len(probeCandidates) {
		summary.Message = msg(settings.Locale, "task.scan.completed_partial", len(selectedCandidates), len(probeCandidates))
	} else {
		summary.Message = msg(settings.Locale, "task.scan.completed", len(filtered))
	}
	summary.NormalCount = dashboard.NormalCount
	summary.Invalid401Count = dashboard.Invalid401Count
	summary.QuotaLimitedCount = dashboard.QuotaLimitedCount
	summary.RecoveredCount = dashboard.RecoveredCount
	summary.ErrorCount = dashboard.ErrorCount

	b.emitProgress(kind, "persist", 1, 1, msg(settings.Locale, "task.scan.saved_snapshot"), true)
	b.emitLog(kind, "info", summary.Message)
	return summary, records, probed, nil
}

func (b *Backend) runMaintain(ctx context.Context, settings AppSettings) (MaintainResult, error) {
	result := MaintainResult{}

	summary, records, probes, err := b.runScan(ctx, "maintain", settings)
	result.Scan = summary
	if err != nil {
		return result, err
	}

	recordMap := make(map[string]AccountRecord, len(records))
	for _, record := range records {
		recordMap[record.Name] = record
	}

	filtered := filterAccountsBySettings(records, settings)
	var invalid []AccountRecord
	var quota []AccountRecord
	var recovered []AccountRecord
	for _, record := range filtered {
		switch normalizeStateKey(record.StateKey) {
		case stateInvalid401:
			invalid = append(invalid, record)
		case stateQuotaLimited:
			quota = append(quota, record)
		case stateRecovered:
			recovered = append(recovered, record)
		}
	}

	if settings.Delete401 && len(invalid) > 0 {
		names := namesFromRecords(invalid)
		b.emitLog("maintain", "info", msg(settings.Locale, "task.maintain.delete_invalid", len(names)))
		result.Delete401Results, err = b.runActionGroup(ctx, "maintain", "delete401", "delete", settings.Locale, settings.DetailedLogs, names, settings.ActionWorkers, func(actionCtx context.Context, name string) ActionResult {
			return b.client.DeleteAccount(actionCtx, settings, name)
		})
		if err != nil {
			return result, err
		}
		applyDeleteResults(recordMap, result.Delete401Results, "delete_401", "deleted_401")
	}

	deletedNames := successfulNames(result.Delete401Results)

	switch settings.QuotaAction {
	case "disable":
		var toDisable []string
		for _, record := range quota {
			if slices.Contains(deletedNames, record.Name) || record.Disabled {
				continue
			}
			toDisable = append(toDisable, record.Name)
		}
		if len(toDisable) > 0 {
			b.emitLog("maintain", "info", msg(settings.Locale, "task.maintain.disable_quota", len(toDisable)))
			result.QuotaActionResults, err = b.runActionGroup(ctx, "maintain", "quota", "disable", settings.Locale, settings.DetailedLogs, toDisable, settings.ActionWorkers, func(actionCtx context.Context, name string) ActionResult {
				return b.client.SetAccountDisabled(actionCtx, settings, name, true)
			})
			if err != nil {
				return result, err
			}
			applyDisableResults(recordMap, result.QuotaActionResults, "disable_quota", "quota_disabled", true)
		}
	case "delete":
		var toDelete []string
		for _, record := range quota {
			if slices.Contains(deletedNames, record.Name) {
				continue
			}
			toDelete = append(toDelete, record.Name)
		}
		if len(toDelete) > 0 {
			b.emitLog("maintain", "info", msg(settings.Locale, "task.maintain.delete_quota", len(toDelete)))
			result.QuotaActionResults, err = b.runActionGroup(ctx, "maintain", "quota", "delete", settings.Locale, settings.DetailedLogs, toDelete, settings.ActionWorkers, func(actionCtx context.Context, name string) ActionResult {
				return b.client.DeleteAccount(actionCtx, settings, name)
			})
			if err != nil {
				return result, err
			}
			applyDeleteResults(recordMap, result.QuotaActionResults, "delete_quota", "quota_deleted")
			deletedNames = append(deletedNames, successfulNames(result.QuotaActionResults)...)
		}
	}

	if settings.AutoReenable && len(recovered) > 0 {
		var toEnable []string
		for _, record := range recovered {
			if slices.Contains(deletedNames, record.Name) {
				continue
			}
			toEnable = append(toEnable, record.Name)
		}
		if len(toEnable) > 0 {
			b.emitLog("maintain", "info", msg(settings.Locale, "task.maintain.reenable", len(toEnable)))
			result.ReenableResults, err = b.runActionGroup(ctx, "maintain", "reenable", "enable", settings.Locale, settings.DetailedLogs, toEnable, settings.ActionWorkers, func(actionCtx context.Context, name string) ActionResult {
				return b.client.SetAccountDisabled(actionCtx, settings, name, false)
			})
			if err != nil {
				return result, err
			}
			applyDisableResults(recordMap, result.ReenableResults, "reenable_quota", "", false)
		}
	}

	finalRecords := recordsFromMap(recordMap)
	if err := b.store.ReplaceCurrentAccounts(finalRecords); err != nil {
		return result, err
	}
	quotaSnapshot, ok, err := b.buildCodexQuotaSnapshotFromUsageProbes(settings, finalRecords, probes, nowISO(), "maintain")
	if err != nil {
		return result, err
	}
	if ok {
		if err := b.persistCodexQuotaSnapshot(quotaSnapshot); err != nil {
			return result, err
		}
	}
	b.emitProgress("maintain", "complete", 1, 1, msg(settings.Locale, "task.maintain.completed"), true)
	b.emitLog("maintain", "info", msg(settings.Locale, "task.maintain.completed"))
	return result, nil
}

func (b *Backend) probeAccounts(ctx context.Context, kind string, settings AppSettings, records []AccountRecord) ([]UsageProbeResult, error) {
	if len(records) == 0 {
		b.emitProgress(kind, "probe", 0, 0, msg(settings.Locale, "task.scan.no_candidates"), true)
		return nil, nil
	}

	workers := settings.ProbeWorkers
	if workers <= 0 {
		workers = defaultProbeWorkers
	}

	results := make([]UsageProbeResult, len(records))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var completed int64

	for index, record := range records {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wg.Add(1)
		go func(index int, record AccountRecord) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			probed := b.client.ProbeUsageResult(ctx, settings, record, b.probeRetryLogger(settings.DetailedLogs, kind, settings.Locale))
			if ctx.Err() != nil {
				return
			}

			results[index] = probed
			current := int(atomic.AddInt64(&completed, 1))
			b.emitDetailedLog(settings.DetailedLogs, kind, probeLogLevel(probed.Record), msg(settings.Locale, "task.scan.single_probe", probed.Record.Name, stateLabel(settings.Locale, probed.Record.StateKey)))
			b.emitProgress(kind, "probe", current, len(records), msg(settings.Locale, "task.scan.probed_account", probed.Record.Name), current == len(records))
		}(index, record)
	}

	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return results, nil
}

func (b *Backend) runActionGroup(ctx context.Context, kind string, phase string, label string, locale string, detailed bool, names []string, workers int, action func(context.Context, string) ActionResult) ([]ActionResult, error) {
	if len(names) == 0 {
		b.emitProgress(kind, phase, 0, 0, msg(locale, "task.action.none_queued"), true)
		return nil, nil
	}
	if workers <= 0 {
		workers = defaultActionWorkers
	}

	results := make([]ActionResult, len(names))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var completed int64

	for index, name := range names {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			result := action(ctx, name)
			if ctx.Err() != nil {
				return
			}

			results[index] = result
			current := int(atomic.AddInt64(&completed, 1))
			state := "info"
			actionLabel := msg(locale, "task.action."+label)
			message := ""
			if result.OK {
				message = msg(locale, "task.action.success", actionLabel, name)
				b.emitDetailedLog(detailed, kind, state, message)
			} else {
				state = "error"
				message = msg(locale, "task.action.failed", actionLabel, name, result.Error)
				b.emitLog(kind, state, message)
			}
			b.emitProgress(kind, phase, current, len(names), message, current == len(names))
		}(index, name)
	}

	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return results, nil
}

func probeLogLevel(record AccountRecord) string {
	switch normalizeStateKey(record.StateKey) {
	case stateError:
		return "error"
	case stateInvalid401, stateQuotaLimited:
		return "warning"
	default:
		return "info"
	}
}

func (b *Backend) probeRetryLogger(enabled bool, kind string, locale string) ProbeRetryObserver {
	if !enabled {
		return nil
	}
	return func(event ProbeRetryEvent) {
		b.emitDetailedLog(true, kind, "warning", msg(locale, "task.scan.retry_probe", event.AccountName, event.RetryIndex, event.MaxRetries, event.ProbeErrorText))
	}
}

func taskStatus(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "failed"
	}
}

func applyDeleteResults(records map[string]AccountRecord, results []ActionResult, lastAction string, managedReason string) {
	for _, result := range results {
		record, ok := records[result.Name]
		if !ok {
			continue
		}
		record.LastAction = lastAction
		record.LastActionStatus = statusText(result.OK)
		record.LastActionError = result.Error
		record.UpdatedAt = nowISO()
		if result.OK {
			record.ManagedReason = managedReason
			delete(records, result.Name)
		} else {
			records[result.Name] = record
		}
	}
}

func applyDisableResults(records map[string]AccountRecord, results []ActionResult, lastAction string, managedReason string, disabled bool) {
	for _, result := range results {
		record, ok := records[result.Name]
		if !ok {
			continue
		}
		record.LastAction = lastAction
		record.LastActionStatus = statusText(result.OK)
		record.LastActionError = result.Error
		record.UpdatedAt = nowISO()
		if result.OK {
			record.Disabled = disabled
			record.ManagedReason = managedReason
			if !disabled {
				record.StateKey = stateNormal
				record.State = stateNormal
				record.Recovered = false
			}
		}
		records[result.Name] = record
	}
}

func recordsFromMap(values map[string]AccountRecord) []AccountRecord {
	records := make([]AccountRecord, 0, len(values))
	for _, record := range values {
		records = append(records, record)
	}
	sortAccounts(records)
	return records
}

func successfulNames(results []ActionResult) []string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		if result.OK {
			names = append(names, result.Name)
		}
	}
	return names
}

func namesFromRecords(records []AccountRecord) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}

func statusText(ok bool) string {
	if ok {
		return "success"
	}
	return "failed"
}

func filterAccountsForExport(records []AccountRecord, kind string) []AccountRecord {
	var selected []AccountRecord
	for _, record := range records {
		switch kind {
		case "invalid401":
			if normalizeStateKey(record.StateKey) == stateInvalid401 {
				selected = append(selected, record)
			}
		case "quotaLimited":
			if normalizeStateKey(record.StateKey) == stateQuotaLimited {
				selected = append(selected, record)
			}
		}
	}
	return selected
}

func writeCSV(path string, locale string, records []AccountRecord) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		msg(locale, "csv.header.name"),
		msg(locale, "csv.header.email"),
		msg(locale, "csv.header.provider"),
		msg(locale, "csv.header.type"),
		msg(locale, "csv.header.plan_type"),
		msg(locale, "csv.header.state"),
		msg(locale, "csv.header.disabled"),
		msg(locale, "csv.header.status_message"),
		msg(locale, "csv.header.probe_error_text"),
		msg(locale, "csv.header.last_probed_at"),
		msg(locale, "csv.header.last_action"),
		msg(locale, "csv.header.last_action_status"),
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, record := range records {
		record = sanitizeRecord(record)
		row := []string{
			record.Name,
			record.Email,
			record.Provider,
			record.Type,
			record.PlanType,
			stateLabel(locale, record.StateKey),
			boolLabel(locale, record.Disabled),
			record.StatusMessage,
			record.ProbeErrorText,
			record.LastProbedAt,
			record.LastAction,
			record.LastActionStatus,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}
