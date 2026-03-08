import { i18n } from '@/i18n'
import type { AccountRecord, AccountStateKey } from '@/types'

export const stateOrder: AccountStateKey[] = ['pending', 'normal', 'invalid_401', 'quota_limited', 'recovered', 'error']

export function normalizeStateKey(state: string | null | undefined): AccountStateKey {
  const value = String(state ?? '').trim().toLowerCase()

  switch (value) {
    case 'pending':
    case '待探测':
      return 'pending'
    case 'normal':
    case '正常':
      return 'normal'
    case 'invalid_401':
    case '401 invalid':
    case '401 失效':
    case '401失效':
      return 'invalid_401'
    case 'quota_limited':
    case 'quota limited':
    case '额度用尽':
      return 'quota_limited'
    case 'recovered':
    case '可恢复':
      return 'recovered'
    case 'error':
    case '错误':
      return 'error'
    default:
      return 'untracked'
  }
}

export function statusTagType(state: string): 'success' | 'danger' | 'warning' | 'info' {
  switch (normalizeStateKey(state)) {
    case 'normal':
    case 'recovered':
      return 'success'
    case 'invalid_401':
      return 'danger'
    case 'quota_limited':
      return 'warning'
    default:
      return 'info'
  }
}

export function stateLabel(state: string): string {
  return i18n.global.t(`states.${normalizeStateKey(state)}`)
}

export function stateDescription(record: AccountRecord): string {
  if (record.probeErrorText) {
    return record.probeErrorText
  }
  if (record.statusMessage) {
    return record.statusMessage
  }
  if (record.limitReached) {
    return i18n.global.t('descriptions.quotaReached')
  }
  if (record.unavailable) {
    return i18n.global.t('descriptions.markedUnavailable')
  }
  return i18n.global.t('descriptions.noExtraDetails')
}

export function taskPhaseLabel(phase: string): string {
  const key = `tasks.phases.${phase || 'idle'}`
  return i18n.global.te(key) ? i18n.global.t(key) : phase || i18n.global.t('common.idle')
}

export function taskStatusLabel(status: string): string {
  const key = `tasks.statuses.${status || 'running'}`
  return i18n.global.te(key) ? i18n.global.t(key) : status || i18n.global.t('tasks.statuses.running')
}

export function logKindLabel(kind: string): string {
  const key = `logs.kind.${kind}`
  return i18n.global.te(key) ? i18n.global.t(key) : kind
}

export function logLevelLabel(level: string): string {
  const key = `logs.level.${level}`
  return i18n.global.te(key) ? i18n.global.t(key) : level
}

export function quotaActionLabel(action: string): string {
  const key = `quotaActions.${action}`
  return i18n.global.te(key) ? i18n.global.t(key) : action
}
