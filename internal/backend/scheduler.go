package backend

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type schedulerRuntime struct {
	backend *Backend
	parser  cron.Parser

	mu      sync.Mutex
	engine  *cron.Cron
	entryID cron.EntryID
	version int64
	status  SchedulerStatus
}

func newSchedulerRuntime(backend *Backend) *schedulerRuntime {
	return &schedulerRuntime{
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
			Mode:       defaultScheduleMode,
			Valid:      true,
			LastStatus: "disabled",
		},
	}
}

func (s *schedulerRuntime) ApplySettings(settings AppSettings) {
	schedule := ScheduleSettings{
		Enabled: settings.Schedule.Enabled,
		Mode:    normalizeScheduleMode(settings.Schedule.Mode),
		Cron:    strings.TrimSpace(settings.Schedule.Cron),
	}

	s.mu.Lock()
	previous := s.status
	oldEngine := s.engine
	s.engine = nil
	s.entryID = 0
	s.version++
	version := s.version

	nextStatus := SchedulerStatus{
		Enabled:        schedule.Enabled,
		Mode:           schedule.Mode,
		Cron:           schedule.Cron,
		Valid:          true,
		LastStartedAt:  previous.LastStartedAt,
		LastFinishedAt: previous.LastFinishedAt,
		LastStatus:     previous.LastStatus,
		LastMessage:    previous.LastMessage,
	}

	if !schedule.Enabled {
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

	if err := validateScheduleSettings(settings.Locale, schedule); err != nil {
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
	entryID, err := engine.AddFunc(schedule.Cron, func() {
		s.execute(version, schedule.Mode, schedule.Cron)
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

func (s *schedulerRuntime) Status() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *schedulerRuntime) Close() {
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

func (s *schedulerRuntime) execute(version int64, mode string, cronExpr string) {
	locale := localeEnglish
	if settings, err := s.backend.store.LoadSettings(); err == nil {
		locale = settings.Locale
	}

	startMessage := msg(locale, "task.schedule.triggered", taskName(locale, mode), cronExpr)
	s.markRunning(version, startMessage)

	resultStatus, resultMessage := s.backend.executeScheduledTask(mode, cronExpr)
	s.finish(version, resultStatus, resultMessage)
}

func (s *schedulerRuntime) markRunning(version int64, message string) {
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

func (s *schedulerRuntime) finish(version int64, resultStatus string, message string) {
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

func (s *schedulerRuntime) currentNextRunLocked() string {
	if s.engine == nil || s.entryID == 0 {
		return ""
	}
	return formatSchedulerTime(s.engine.Entry(s.entryID).Next)
}

func (s *schedulerRuntime) emitStatus(status SchedulerStatus) {
	if s.backend != nil && s.backend.emitter != nil {
		s.backend.emitter.Emit("scheduler:status", status)
	}
}

func (s *schedulerRuntime) emitStatusLocked() {
	snapshot := s.status
	go s.emitStatus(snapshot)
}

func formatSchedulerTime(next time.Time) string {
	if next.IsZero() {
		return ""
	}
	return next.Format(time.RFC3339)
}

func validateScheduleSettings(locale string, schedule ScheduleSettings) error {
	mode := strings.ToLower(strings.TrimSpace(schedule.Mode))
	if mode != "scan" && mode != "maintain" {
		return errors.New(msg(locale, "error.schedule_invalid_mode"))
	}
	if strings.TrimSpace(schedule.Cron) == "" {
		return errors.New(msg(locale, "error.schedule_cron_required"))
	}
	parser := cron.NewParser(
		cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow,
	)
	if _, err := parser.Parse(strings.TrimSpace(schedule.Cron)); err != nil {
		return fmt.Errorf("%s: %w", msg(locale, "error.schedule_invalid_cron"), err)
	}
	return nil
}

func (b *Backend) executeScheduledTask(mode string, cronExpr string) (string, string) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return "failed", err.Error()
	}
	if err := ensureConfigured(settings); err != nil {
		message := msg(settings.Locale, "task.schedule.failed", taskName(settings.Locale, mode), err)
		b.emitLog(mode, "error", message)
		return "failed", message
	}

	ctx, err := b.beginTask(mode, settings.Locale)
	if err != nil {
		var runningErr taskRunningError
		if errors.As(err, &runningErr) {
			message := msg(settings.Locale, "task.schedule.skipped_active", taskName(settings.Locale, mode), taskName(settings.Locale, runningErr.activeKind))
			b.emitLog(mode, "warning", message)
			return "skipped", message
		}
		message := msg(settings.Locale, "task.schedule.failed", taskName(settings.Locale, mode), err)
		b.emitLog(mode, "error", message)
		return "failed", message
	}
	defer b.endTask()

	b.emitLog(mode, "info", msg(settings.Locale, "task.schedule.triggered", taskName(settings.Locale, mode), cronExpr))

	var runErr error
	var resultMessage string
	switch mode {
	case "maintain":
		result, err := b.runMaintain(ctx, settings)
		runErr = err
		resultMessage = result.Scan.Message
	default:
		summary, _, _, err := b.runScan(ctx, "scan", settings)
		runErr = err
		resultMessage = summary.Message
	}

	if runErr != nil {
		status := taskStatus(runErr)
		message := msg(settings.Locale, "task.schedule.failed", taskName(settings.Locale, mode), runErr)
		level := "error"
		if status == "cancelled" {
			level = "warning"
		}
		b.emitLog(mode, level, message)
		b.emitTaskFinished(mode, status, message)
		return status, message
	}

	message := msg(settings.Locale, "task.schedule.completed", taskName(settings.Locale, mode))
	b.emitLog(mode, "info", message)
	b.emitTaskFinished(mode, "success", resultMessage)
	return "success", message
}
