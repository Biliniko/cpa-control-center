import type { AppSettings, AuthImportSettings, ScheduleSettings } from '@/types'
import { detectPreferredLocale } from '@/utils/locale'

type Translate = (key: string, params?: Record<string, unknown>) => string

const fallbackTranslate: Translate = (key) => key
const defaultScheduleCron = '0 * * * *'
const defaultAuthImportAutoCron = '10 * * * *'
const defaultQuotaAutoRefreshCron = '20 */2 * * *'

export type ScheduleConflictJob = 'schedule' | 'authImport' | 'quotaAutoRefresh'

export interface ScheduleConflict {
  jobs: [ScheduleConflictJob, ScheduleConflictJob]
  at: string
}

export function createDefaultSettings(): AppSettings {
  return {
    baseUrl: '',
    managementToken: '',
    locale: detectPreferredLocale(),
    detailedLogs: false,
    targetType: 'codex',
    provider: '',
    scanStrategy: 'full',
    scanBatchSize: 1000,
    skipKnown401: true,
    probeWorkers: 40,
    actionWorkers: 20,
    quotaWorkers: 10,
    timeoutSeconds: 15,
    retries: 3,
    userAgent: 'codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal',
    quotaAction: 'disable',
    quotaCheckFree: false,
    quotaCheckPlus: true,
    quotaCheckPro: true,
    quotaCheckTeam: true,
    quotaCheckBusiness: true,
    quotaCheckEnterprise: true,
    quotaFreeMaxAccounts: 100,
    quotaAutoRefreshEnabled: false,
    quotaAutoRefreshCron: defaultQuotaAutoRefreshCron,
    delete401: true,
    autoReenable: true,
    exportDirectory: '',
    authImport: createDefaultAuthImportSettings(),
    schedule: createDefaultScheduleSettings(),
  }
}

export function validateSettings(settings: AppSettings, t: Translate = fallbackTranslate): Record<string, string> {
  const errors: Record<string, string> = {}

  if (!settings.baseUrl.trim()) {
    errors.baseUrl = t('validation.baseUrlRequired')
  } else if (!/^https?:\/\//i.test(settings.baseUrl.trim())) {
    errors.baseUrl = t('validation.baseUrlProtocol')
  }

  if (!settings.managementToken.trim()) {
    errors.managementToken = t('validation.managementTokenRequired')
  }

  if (settings.probeWorkers < 1) {
    errors.probeWorkers = t('validation.probeWorkersMin')
  }
  if (settings.actionWorkers < 1) {
    errors.actionWorkers = t('validation.actionWorkersMin')
  }
  if (settings.quotaWorkers < 1) {
    errors.quotaWorkers = t('validation.quotaWorkersMin')
  }
  if (!['full', 'incremental'].includes(settings.scanStrategy)) {
    errors.scanStrategy = t('validation.scanStrategyInvalid')
  }
  if (settings.scanStrategy === 'incremental' && settings.scanBatchSize < 1) {
    errors.scanBatchSize = t('validation.scanBatchSizeMin')
  }
  if (settings.timeoutSeconds < 1) {
    errors.timeoutSeconds = t('validation.timeoutMin')
  }
  if (settings.retries < 0) {
    errors.retries = t('validation.retriesMin')
  }
  if (!['disable', 'delete'].includes(settings.quotaAction)) {
    errors.quotaAction = t('validation.quotaActionInvalid')
  }
  if (settings.quotaFreeMaxAccounts < -1) {
    errors.quotaFreeMaxAccounts = t('validation.quotaFreeMaxAccountsMin')
  }
  if (settings.quotaAutoRefreshEnabled) {
    if (!settings.quotaAutoRefreshCron.trim()) {
      errors.quotaAutoRefreshCron = t('validation.quotaAutoRefreshCronRequired')
    } else if (!isValidCronExpression(settings.quotaAutoRefreshCron)) {
      errors.quotaAutoRefreshCron = t('validation.quotaAutoRefreshCronInvalid')
    }
  }
  if (settings.authImport.autoEnabled) {
    if (!settings.authImport.sourceDirectory.trim()) {
      errors.authImportSourceDirectory = t('validation.authImportSourceDirectoryRequired')
    }
    if (!settings.authImport.autoCron.trim()) {
      errors.authImportAutoCron = t('validation.authImportAutoCronRequired')
    } else if (!isValidCronExpression(settings.authImport.autoCron)) {
      errors.authImportAutoCron = t('validation.authImportAutoCronInvalid')
    }
  }
  if (settings.schedule.enabled) {
    if (!['scan', 'maintain'].includes(settings.schedule.mode)) {
      errors.scheduleMode = t('validation.scheduleModeInvalid')
    }
    if (!settings.schedule.cron.trim()) {
      errors.scheduleCron = t('validation.scheduleCronRequired')
    } else if (!isValidCronExpression(settings.schedule.cron)) {
      errors.scheduleCron = t('validation.scheduleCronInvalid')
    }
  }

  return errors
}

export function createDefaultAuthImportSettings(): AuthImportSettings {
  return {
    sourceDirectory: '',
    archiveDirectory: '',
    moveImported: false,
    autoEnabled: false,
    autoCron: defaultAuthImportAutoCron,
  }
}

export function createDefaultScheduleSettings(): ScheduleSettings {
  return {
    enabled: false,
    mode: 'scan',
    cron: defaultScheduleCron,
  }
}

export function listScheduleConflicts(
  settings: AppSettings,
  now = new Date(),
  horizonDays = 14,
): ScheduleConflict[] {
  const jobs = collectEnabledScheduleJobs(settings)
  if (jobs.length < 2) {
    return []
  }

  const start = new Date(now)
  start.setSeconds(0, 0)
  const totalPairs = (jobs.length * (jobs.length - 1)) / 2
  const deadline = start.getTime() + (horizonDays * 24 * 60 * 60 * 1000)
  const seenPairs = new Set<string>()
  const conflicts: ScheduleConflict[] = []

  for (let minute = start.getTime(); minute <= deadline && seenPairs.size < totalPairs; minute += 60_000) {
    const current = new Date(minute)
    const matching = jobs.filter((job) => cronMatchesDate(job.cron, current))
    if (matching.length < 2) {
      continue
    }

    for (let index = 0; index < matching.length; index += 1) {
      for (let inner = index + 1; inner < matching.length; inner += 1) {
        const pair = [matching[index].job, matching[inner].job].sort() as [ScheduleConflictJob, ScheduleConflictJob]
        const key = pair.join(':')
        if (seenPairs.has(key)) {
          continue
        }
        seenPairs.add(key)
        conflicts.push({
          jobs: [matching[index].job, matching[inner].job],
          at: current.toISOString(),
        })
      }
    }
  }

  return conflicts
}

export function isValidCronExpression(value: string): boolean {
  const parts = value.trim().split(/\s+/)
  if (parts.length !== 5) {
    return false
  }

  const fieldSpecs: Array<{ min: number; max: number }> = [
    { min: 0, max: 59 },
    { min: 0, max: 23 },
    { min: 1, max: 31 },
    { min: 1, max: 12 },
    { min: 0, max: 7 },
  ]

  return parts.every((field, index) => isValidCronField(field, fieldSpecs[index].min, fieldSpecs[index].max))
}

export function cronMatchesDate(expression: string, date: Date): boolean {
  const parts = expression.trim().split(/\s+/)
  if (parts.length !== 5) {
    return false
  }

  const values = [
    date.getMinutes(),
    date.getHours(),
    date.getDate(),
    date.getMonth() + 1,
    date.getDay(),
  ]

  const fieldSpecs: Array<{ min: number; max: number }> = [
    { min: 0, max: 59 },
    { min: 0, max: 23 },
    { min: 1, max: 31 },
    { min: 1, max: 12 },
    { min: 0, max: 7 },
  ]

  return parts.every((field, index) => cronFieldMatches(field, values[index], fieldSpecs[index].min, fieldSpecs[index].max))
}

function isValidCronField(field: string, min: number, max: number): boolean {
  return field.split(',').every((segment) => isValidCronSegment(segment.trim(), min, max))
}

function collectEnabledScheduleJobs(settings: AppSettings): Array<{ job: ScheduleConflictJob; cron: string }> {
  const jobs: Array<{ job: ScheduleConflictJob; cron: string }> = []

  if (settings.schedule.enabled && settings.schedule.cron.trim() && isValidCronExpression(settings.schedule.cron)) {
    jobs.push({ job: 'schedule', cron: settings.schedule.cron.trim() })
  }
  if (settings.authImport.autoEnabled && settings.authImport.autoCron.trim() && isValidCronExpression(settings.authImport.autoCron)) {
    jobs.push({ job: 'authImport', cron: settings.authImport.autoCron.trim() })
  }
  if (settings.quotaAutoRefreshEnabled && settings.quotaAutoRefreshCron.trim() && isValidCronExpression(settings.quotaAutoRefreshCron)) {
    jobs.push({ job: 'quotaAutoRefresh', cron: settings.quotaAutoRefreshCron.trim() })
  }

  return jobs
}

function cronFieldMatches(field: string, value: number, min: number, max: number): boolean {
  return field.split(',').some((segment) => cronSegmentMatches(segment.trim(), value, min, max))
}

function isValidCronSegment(segment: string, min: number, max: number): boolean {
  if (!segment) {
    return false
  }
  if (segment === '*') {
    return true
  }

  const [base, stepValue] = segment.split('/')
  if (segment.split('/').length > 2) {
    return false
  }
  if (stepValue !== undefined) {
    if (!/^\d+$/.test(stepValue)) {
      return false
    }
    const step = Number(stepValue)
    if (!Number.isInteger(step) || step <= 0) {
      return false
    }
  }

  if (base === '*') {
    return true
  }

  if (/^\d+$/.test(base)) {
    const value = Number(base)
    return value >= min && value <= max
  }

  const rangeMatch = base.match(/^(\d+)-(\d+)$/)
  if (!rangeMatch) {
    return false
  }

  const start = Number(rangeMatch[1])
  const end = Number(rangeMatch[2])
  return start >= min && end <= max && start <= end
}

function cronSegmentMatches(segment: string, value: number, min: number, max: number): boolean {
  if (!segment) {
    return false
  }
  if (segment === '*') {
    return true
  }

  const [base, stepValue] = segment.split('/')
  if (segment.split('/').length > 2) {
    return false
  }

  let baseMatches = false
  if (base === '*') {
    baseMatches = value >= min && value <= max
  } else if (/^\d+$/.test(base)) {
    baseMatches = Number(base) === value
  } else {
    const rangeMatch = base.match(/^(\d+)-(\d+)$/)
    if (!rangeMatch) {
      return false
    }
    const start = Number(rangeMatch[1])
    const end = Number(rangeMatch[2])
    if (start > end) {
      return false
    }
    baseMatches = value >= start && value <= end
  }

  if (!baseMatches) {
    return false
  }
  if (stepValue === undefined) {
    return true
  }
  if (!/^\d+$/.test(stepValue)) {
    return false
  }
  const step = Number(stepValue)
  if (!Number.isInteger(step) || step <= 0) {
    return false
  }
  if (base === '*') {
    return (value - min) % step === 0
  }
  if (/^\d+$/.test(base)) {
    return value === Number(base)
  }
  const rangeMatch = base.match(/^(\d+)-(\d+)$/)
  if (!rangeMatch) {
    return false
  }
  const start = Number(rangeMatch[1])
  return (value - start) % step === 0
}
