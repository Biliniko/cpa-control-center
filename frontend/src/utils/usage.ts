import type { ManagementUsageSummary, UsageTokenTotals } from '@/types'

const numericPattern = /^-?\d+(?:\.\d+)?$/

export function parseIntegerLike(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? Math.round(value) : null
  }

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed || !numericPattern.test(trimmed)) {
      return null
    }
    const parsed = Number(trimmed)
    return Number.isFinite(parsed) ? Math.round(parsed) : null
  }

  return null
}

function coerceIntegerLike(value: unknown): number {
  return parseIntegerLike(value) ?? 0
}

function coerceString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

type UsageTokenTotalsLike = Partial<Record<keyof UsageTokenTotals, unknown>>
type ManagementUsageSummaryLike = Partial<Record<keyof ManagementUsageSummary, unknown>> & {
  tokens?: UsageTokenTotalsLike | null
}

export function normalizeUsageTokenTotals(source?: UsageTokenTotalsLike | null): UsageTokenTotals {
  return {
    inputTokens: coerceIntegerLike(source?.inputTokens),
    outputTokens: coerceIntegerLike(source?.outputTokens),
    reasoningTokens: coerceIntegerLike(source?.reasoningTokens),
    cachedTokens: coerceIntegerLike(source?.cachedTokens),
    totalTokens: coerceIntegerLike(source?.totalTokens),
  }
}

export function normalizeManagementUsageSummary(
  source?: ManagementUsageSummaryLike | null,
): ManagementUsageSummary | null {
  if (!source || typeof source !== 'object') {
    return null
  }

  const summary: ManagementUsageSummary = {
    fetchedAt: coerceString(source.fetchedAt),
    lastRequestAt: coerceString(source.lastRequestAt),
    totalRequests: coerceIntegerLike(source.totalRequests),
    successCount: coerceIntegerLike(source.successCount),
    failureCount: coerceIntegerLike(source.failureCount),
    failedRequests: coerceIntegerLike(source.failedRequests),
    todayRequests: coerceIntegerLike(source.todayRequests),
    todayTokens: coerceIntegerLike(source.todayTokens),
    apiKeyCount: coerceIntegerLike(source.apiKeyCount),
    modelCount: coerceIntegerLike(source.modelCount),
    tokens: normalizeUsageTokenTotals(source.tokens),
  }

  const hasContent = Boolean(
    summary.fetchedAt ||
    summary.lastRequestAt ||
    summary.totalRequests > 0 ||
    summary.successCount > 0 ||
    summary.failureCount > 0 ||
    summary.failedRequests > 0 ||
    summary.todayRequests > 0 ||
    summary.todayTokens > 0 ||
    summary.apiKeyCount > 0 ||
    summary.modelCount > 0 ||
    summary.tokens.totalTokens > 0 ||
    summary.tokens.inputTokens > 0 ||
    summary.tokens.outputTokens > 0 ||
    summary.tokens.reasoningTokens > 0 ||
    summary.tokens.cachedTokens > 0
  )

  return hasContent ? summary : null
}
