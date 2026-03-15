import { i18n } from '@/i18n'

export function formatDateTime(value: string | null | undefined): string {
  if (!value) {
    return i18n.global.t('common.notAvailable')
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(parsed)
}

export function formatResetCompact(value: string | null | undefined): string {
  if (!value) {
    return i18n.global.t('common.notAvailable')
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  const now = new Date()
  const sameDay =
    parsed.getFullYear() === now.getFullYear() &&
    parsed.getMonth() === now.getMonth() &&
    parsed.getDate() === now.getDate()

  if (sameDay) {
    return new Intl.DateTimeFormat(i18n.global.locale.value, {
      timeStyle: 'short',
    }).format(parsed)
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    month: 'numeric',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(parsed)
}
