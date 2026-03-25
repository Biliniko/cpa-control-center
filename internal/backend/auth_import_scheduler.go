package backend

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type authImportSchedulerRuntime struct {
	backend *Backend
	parser  cron.Parser

	mu      sync.Mutex
	engine  *cron.Cron
	entryID cron.EntryID
	version int64
	status  SchedulerStatus
}

func newAuthImportSchedulerRuntime(backend *Backend) *authImportSchedulerRuntime {
	return &authImportSchedulerRuntime{
		backend: backend,
		parser: cron.NewParser(
			cron.Minute |
				cron.Hour |
				cron.Dom |
				cron.Month |
				cron.Dow,
		),
		status: SchedulerStatus{
			Enabled:    false,
			Mode:       "import",
			Valid:      true,
			LastStatus: "disabled",
		},
	}
}

func (s *authImportSchedulerRuntime) ApplySettings(settings AppSettings) {
	schedule := settings.AuthImport

	s.mu.Lock()
	previous := s.status
	oldEngine := s.engine
	s.engine = nil
	s.entryID = 0
	s.version++
	version := s.version

	nextStatus := SchedulerStatus{
		Enabled:        schedule.AutoEnabled,
		Mode:           "import",
		Cron:           strings.TrimSpace(schedule.AutoCron),
		Valid:          true,
		LastStartedAt:  previous.LastStartedAt,
		LastFinishedAt: previous.LastFinishedAt,
		LastStatus:     previous.LastStatus,
		LastMessage:    previous.LastMessage,
	}

	if !schedule.AutoEnabled {
		nextStatus.LastStatus = "disabled"
		nextStatus.LastMessage = ""
		s.status = nextStatus
		s.mu.Unlock()
		if oldEngine != nil {
			oldEngine.Stop()
		}
		s.emitStatus(nextStatus)
		return
	}

	if err := validateAuthImportScheduleSettings(settings.Locale, schedule); err != nil {
		nextStatus.Valid = false
		nextStatus.ValidationMessage = err.Error()
		nextStatus.LastStatus = "invalid"
		nextStatus.LastMessage = err.Error()
		s.status = nextStatus
		s.mu.Unlock()
		if oldEngine != nil {
			oldEngine.Stop()
		}
		s.emitStatus(nextStatus)
		return
	}

	engine := cron.New(
		cron.WithLocation(time.Local),
		cron.WithParser(s.parser),
	)
	entryID, err := engine.AddFunc(schedule.AutoCron, func() {
		s.execute(version, schedule.AutoCron)
	})
	if err != nil {
		nextStatus.Valid = false
		nextStatus.ValidationMessage = err.Error()
		nextStatus.LastStatus = "invalid"
		nextStatus.LastMessage = err.Error()
		s.status = nextStatus
		s.mu.Unlock()
		if oldEngine != nil {
			oldEngine.Stop()
		}
		s.emitStatus(nextStatus)
		return
	}

	engine.Start()
	nextStatus.NextRunAt = formatSchedulerTime(engine.Entry(entryID).Next)
	s.engine = engine
	s.entryID = entryID
	s.status = nextStatus
	s.mu.Unlock()

	if oldEngine != nil {
		oldEngine.Stop()
	}
	s.emitStatus(nextStatus)
}

func (s *authImportSchedulerRuntime) Status() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *authImportSchedulerRuntime) Close() {
	s.mu.Lock()
	engine := s.engine
	s.engine = nil
	s.entryID = 0
	s.version++
	s.mu.Unlock()

	if engine != nil {
		engine.Stop()
	}
}

func (s *authImportSchedulerRuntime) execute(version int64, cronExpr string) {
	locale := localeEnglish
	if settings, err := s.backend.store.LoadSettings(); err == nil {
		locale = settings.Locale
	}

	startMessage := msg(locale, "task.schedule.triggered", taskName(locale, "import"), cronExpr)
	s.markRunning(version, startMessage)

	resultStatus, resultMessage := s.backend.executeScheduledTask("import", cronExpr)
	s.finish(version, resultStatus, resultMessage)
}

func (s *authImportSchedulerRuntime) triggerQueued(cronExpr string) {
	s.mu.Lock()
	version := s.version
	s.mu.Unlock()

	s.execute(version, cronExpr)
}

func (s *authImportSchedulerRuntime) markRunning(version int64, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version != version {
		return
	}

	s.status.Running = true
	s.status.LastStartedAt = nowISO()
	s.status.LastStatus = "running"
	s.status.LastMessage = message
	s.status.NextRunAt = s.currentNextRunLocked()
	s.emitStatusLocked()
}

func (s *authImportSchedulerRuntime) finish(version int64, resultStatus string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.version != version {
		return
	}

	s.status.Running = false
	s.status.LastFinishedAt = nowISO()
	s.status.LastStatus = resultStatus
	s.status.LastMessage = message
	s.status.NextRunAt = s.currentNextRunLocked()
	s.emitStatusLocked()
}

func (s *authImportSchedulerRuntime) currentNextRunLocked() string {
	if s.engine == nil || s.entryID == 0 {
		return ""
	}
	return formatSchedulerTime(s.engine.Entry(s.entryID).Next)
}

func (s *authImportSchedulerRuntime) emitStatus(status SchedulerStatus) {
	if s.backend != nil && s.backend.emitter != nil {
		s.backend.emitter.Emit("auth-import:scheduler:status", status)
	}
}

func (s *authImportSchedulerRuntime) emitStatusLocked() {
	snapshot := s.status
	go s.emitStatus(snapshot)
}

func validateAuthImportScheduleSettings(locale string, settings AuthImportSettings) error {
	if strings.TrimSpace(settings.SourceDirectory) == "" {
		return errors.New(msg(locale, "error.auth_import_source_required"))
	}
	if strings.TrimSpace(settings.AutoCron) == "" {
		return errors.New(msg(locale, "error.auth_import_auto_cron_required"))
	}
	if settings.MoveImported && sameFilePath(settings.SourceDirectory, settings.ArchiveDirectory) {
		return errors.New(msg(locale, "error.auth_import_archive_same_directory"))
	}
	return validateCronExpression(locale, settings.AutoCron)
}
