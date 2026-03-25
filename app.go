package main

import (
	"context"
	"errors"

	"cpa-control-center/internal/backend"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	backend *backend.Backend
	initErr error
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dataDir, err := backend.DefaultDataDir()
	if err != nil {
		a.initErr = err
		return
	}
	service, err := backend.New(dataDir, a)
	if err != nil {
		a.initErr = err
		return
	}
	a.backend = service
}

func (a *App) shutdown(ctx context.Context) {
	if a.backend != nil {
		_ = a.backend.Close()
	}
}

func (a *App) ensureBackend() (*backend.Backend, error) {
	if a.initErr != nil {
		return nil, a.initErr
	}
	if a.backend == nil {
		return nil, errors.New("backend not initialized")
	}
	return a.backend, nil
}

func (a *App) Emit(event string, payload any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, event, payload)
	}
}

func (a *App) GetSettings() (backend.AppSettings, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AppSettings{}, err
	}
	return service.GetSettings()
}

func (a *App) SaveSettings(input backend.AppSettings) (backend.AppSettings, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AppSettings{}, err
	}
	return service.SaveSettings(input)
}

func (a *App) TestConnection(input backend.AppSettings) (backend.ConnectionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ConnectionResult{}, err
	}
	return service.TestConnection(input)
}

func (a *App) TestAndSaveSettings(input backend.AppSettings) (backend.ConnectionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ConnectionResult{}, err
	}
	return service.TestAndSaveSettings(input)
}

func (a *App) SyncInventory() (backend.InventorySyncResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.InventorySyncResult{}, err
	}
	return service.SyncInventory()
}

func (a *App) ImportAuthFiles() (backend.AuthImportResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AuthImportResult{}, err
	}
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select auth JSON files",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "JSON Files (*.json)",
				Pattern:     "*.json",
			},
		},
	})
	if err != nil {
		return backend.AuthImportResult{}, err
	}
	if len(paths) == 0 {
		return backend.AuthImportResult{}, nil
	}
	return service.ImportAuthFiles(paths)
}

func (a *App) ImportAuthDirectory() (backend.AuthImportResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AuthImportResult{}, err
	}
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select auth directory",
	})
	if err != nil {
		return backend.AuthImportResult{}, err
	}
	if directory == "" {
		return backend.AuthImportResult{}, nil
	}
	return service.ImportAuthDirectory(directory)
}

func (a *App) GetSchedulerStatus() (backend.SchedulerStatus, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.SchedulerStatus{}, err
	}
	return service.GetSchedulerStatus(), nil
}

func (a *App) GetAuthImportSchedulerStatus() (backend.SchedulerStatus, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.SchedulerStatus{}, err
	}
	return service.GetAuthImportSchedulerStatus(), nil
}

func (a *App) GetDashboardSummary() (backend.DashboardSummary, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.DashboardSummary{}, err
	}
	return service.GetDashboardSummary()
}

func (a *App) GetDashboardSnapshot() (backend.DashboardSnapshot, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.DashboardSnapshot{}, err
	}
	return service.GetDashboardSnapshot()
}

func (a *App) GetCodexQuotaSnapshot() (backend.CodexQuotaSnapshot, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.CodexQuotaSnapshot{}, err
	}
	return service.GetCodexQuotaSnapshot()
}

func (a *App) GetManagementUsageSummary() (backend.ManagementUsageSummary, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ManagementUsageSummary{}, err
	}
	return service.GetManagementUsageSummary()
}

func (a *App) ListAccounts(filter backend.AccountFilter) ([]backend.AccountRecord, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return nil, err
	}
	return service.ListAccounts(filter)
}

func (a *App) ListAccountsPage(filter backend.AccountFilter, page int, pageSize int) (backend.AccountPage, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AccountPage{}, err
	}
	return service.ListAccountsPage(filter, page, pageSize)
}

func (a *App) RunScan() (backend.ScanSummary, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ScanSummary{}, err
	}
	return service.RunScan()
}

func (a *App) CancelScan() (bool, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return false, err
	}
	return service.CancelScan()
}

func (a *App) RunMaintain(options backend.MaintainOptions) (backend.MaintainResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.MaintainResult{}, err
	}
	return service.RunMaintain(options)
}

func (a *App) ProbeAccount(name string) (backend.AccountRecord, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.AccountRecord{}, err
	}
	return service.ProbeAccount(name)
}

func (a *App) ProbeAccounts(names []string) (backend.BulkAccountActionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.BulkAccountActionResult{}, err
	}
	return service.ProbeAccounts(names)
}

func (a *App) SetAccountDisabled(name string, disabled bool) (backend.ActionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ActionResult{}, err
	}
	return service.SetAccountDisabled(name, disabled)
}

func (a *App) SetAccountsDisabled(names []string, disabled bool) (backend.BulkAccountActionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.BulkAccountActionResult{}, err
	}
	return service.SetAccountsDisabled(names, disabled)
}

func (a *App) DeleteAccount(name string) (backend.ActionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ActionResult{}, err
	}
	return service.DeleteAccount(name)
}

func (a *App) DeleteAccounts(names []string) (backend.BulkAccountActionResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.BulkAccountActionResult{}, err
	}
	return service.DeleteAccounts(names)
}

func (a *App) ExportAccounts(kind string, format string, path string) (backend.ExportResult, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ExportResult{}, err
	}
	return service.ExportAccounts(kind, format, path)
}

func (a *App) ListScanHistory(limit int) ([]backend.ScanSummary, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return nil, err
	}
	return service.ListScanHistory(limit)
}

func (a *App) GetScanDetails(runID int64) (backend.ScanDetail, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ScanDetail{}, err
	}
	return service.GetScanDetails(runID)
}

func (a *App) GetScanDetailsPage(runID int64, page int, pageSize int) (backend.ScanDetailPage, error) {
	service, err := a.ensureBackend()
	if err != nil {
		return backend.ScanDetailPage{}, err
	}
	return service.GetScanDetailsPage(runID, page, pageSize)
}
