import { describe, expect, it } from 'vitest'
import { normalizeManagementUsageSummary, parseIntegerLike } from '@/utils/usage'

describe('parseIntegerLike', () => {
  it('parses numeric strings from the Wails bridge', () => {
    expect(parseIntegerLike('67859370')).toBe(67859370)
    expect(parseIntegerLike(' 520 ')).toBe(520)
    expect(parseIntegerLike(13)).toBe(13)
  })

  it('rejects non-numeric values', () => {
    expect(parseIntegerLike('')).toBeNull()
    expect(parseIntegerLike('abc')).toBeNull()
    expect(parseIntegerLike(undefined)).toBeNull()
  })
})

describe('normalizeManagementUsageSummary', () => {
  it('coerces token totals when the bridge returns int64 values as strings', () => {
    const summary = normalizeManagementUsageSummary({
      fetchedAt: '2026-03-19T12:30:00+08:00',
      lastRequestAt: '2026-03-19T12:23:44.2730691+08:00',
      totalRequests: 582,
      successCount: 569,
      failureCount: 13,
      failedRequests: 13,
      todayRequests: '520',
      todayTokens: '61693834',
      apiKeyCount: 1,
      modelCount: 2,
      tokens: {
        inputTokens: '123456',
        outputTokens: '789',
        reasoningTokens: '456',
        cachedTokens: '111',
        totalTokens: '67859370',
      },
    })

    expect(summary).toMatchObject({
      totalRequests: 582,
      todayRequests: 520,
      todayTokens: 61693834,
      tokens: {
        inputTokens: 123456,
        outputTokens: 789,
        reasoningTokens: 456,
        cachedTokens: 111,
        totalTokens: 67859370,
      },
    })
  })

  it('returns null for an empty usage payload', () => {
    expect(normalizeManagementUsageSummary({})).toBeNull()
  })
})
