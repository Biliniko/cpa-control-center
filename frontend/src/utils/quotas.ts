import type { CodexQuotaAccountDetail, QuotaBucketDetail, QuotaBucketSummary, QuotaRecoveryMode, QuotaSortMode } from '@/types'

export function bucketNumber(value?: number | null) {
  return typeof value === 'number' && !Number.isNaN(value) ? value : null
}

export function normalizeQuotaPlanType(value: string) {
  const normalized = value.trim().toLowerCase()
  return normalized || 'unknown'
}

export function quotaPlanRank(value: string) {
  switch (normalizeQuotaPlanType(value)) {
    case 'all':
      return -1
    case 'free':
      return 0
    case 'plus':
      return 1
    case 'pro':
      return 2
    case 'team':
      return 3
    case 'business':
      return 4
    case 'enterprise':
      return 5
    default:
      return 10
  }
}

export function quotaTotalRemaining(account: CodexQuotaAccountDetail) {
  return [account.fiveHour, account.weekly, account.codeReviewWeekly].reduce((sum, bucket) => {
    const value = bucketNumber(bucket.remainingPercent)
    return value === null ? sum : sum + value
  }, 0)
}

export function quotaRecoveryResetAt(account: CodexQuotaAccountDetail, mode: QuotaRecoveryMode) {
  switch (mode) {
    case 'fiveHour':
      return account.fiveHour.resetAt
    case 'weekly':
      return account.weekly.resetAt
    case 'earliest':
    default:
      return account.earliestResetAt
  }
}

function compareQuotaBucketRemaining(left: QuotaBucketDetail, right: QuotaBucketDetail) {
  if (left.supported !== right.supported) {
    return left.supported ? -1 : 1
  }

  const leftValue = bucketNumber(left.remainingPercent)
  const rightValue = bucketNumber(right.remainingPercent)
  if (leftValue !== null && rightValue !== null) {
    const diff = rightValue - leftValue
    if (Math.abs(diff) > 0.01) {
      return diff
    }
  } else if (leftValue !== rightValue) {
    return leftValue === null ? 1 : -1
  }

  if (left.resetAt && right.resetAt && left.resetAt !== right.resetAt) {
    return left.resetAt.localeCompare(right.resetAt)
  }
  if (left.resetAt !== right.resetAt) {
    return left.resetAt ? -1 : 1
  }

  return 0
}

export function quotaAverageRemainingPercent(bucket: QuotaBucketSummary) {
  const total = bucket.totalRemainingPercent
  if (typeof total !== 'number' || Number.isNaN(total) || bucket.successCount <= 0) {
    return null
  }
  return total / bucket.successCount
}

export function quotaCapacity(bucket: QuotaBucketSummary) {
  return bucket.successCount * 100
}

export function quotaNormalizedFill(bucket: QuotaBucketSummary) {
  const average = quotaAverageRemainingPercent(bucket)
  if (typeof average !== 'number' || Number.isNaN(average)) {
    return 0
  }
  return Math.max(0, Math.min(100, average))
}

function interpolateChannel(start: number, end: number, ratio: number) {
  return Math.round(start + (end - start) * ratio)
}

export function quotaMeterColor(bucket: QuotaBucketSummary) {
  const fill = quotaNormalizedFill(bucket)
  const low = { r: 193, g: 74, b: 56 }
  const mid = { r: 201, g: 154, b: 37 }
  const high = { r: 45, g: 139, b: 107 }

  let start = low
  let end = mid
  let ratio = fill / 50
  if (fill >= 50) {
    start = mid
    end = high
    ratio = (fill - 50) / 50
  }

  const r = interpolateChannel(start.r, end.r, ratio)
  const g = interpolateChannel(start.g, end.g, ratio)
  const b = interpolateChannel(start.b, end.b, ratio)
  return `rgb(${r}, ${g}, ${b})`
}

export function compareQuotaAccounts(mode: QuotaSortMode) {
  return (left: CodexQuotaAccountDetail, right: CodexQuotaAccountDetail) => {
    const leftPlan = normalizeQuotaPlanType(left.planType)
    const rightPlan = normalizeQuotaPlanType(right.planType)
    const leftPlanRank = quotaPlanRank(leftPlan)
    const rightPlanRank = quotaPlanRank(rightPlan)
    if (mode === 'plan' && leftPlan !== rightPlan) {
      return leftPlanRank - rightPlanRank || leftPlan.localeCompare(rightPlan)
    }

    switch (mode) {
      case 'total': {
        const diff = quotaTotalRemaining(right) - quotaTotalRemaining(left)
        if (Math.abs(diff) > 0.01) {
          return diff
        }
        break
      }
      case 'fiveHour': {
        const diff = compareQuotaBucketRemaining(left.fiveHour, right.fiveHour)
        if (diff !== 0) {
          return diff
        }
        break
      }
      case 'weekly': {
        const diff = compareQuotaBucketRemaining(left.weekly, right.weekly)
        if (diff !== 0) {
          return diff
        }
        break
      }
      case 'reset': {
        if (left.earliestResetAt && right.earliestResetAt && left.earliestResetAt !== right.earliestResetAt) {
          return left.earliestResetAt.localeCompare(right.earliestResetAt)
        }
        if (left.earliestResetAt !== right.earliestResetAt) {
          return left.earliestResetAt ? -1 : 1
        }
        break
      }
      case 'name':
      case 'plan':
      default:
        break
    }

    if (leftPlan !== rightPlan) {
      return leftPlanRank - rightPlanRank || leftPlan.localeCompare(rightPlan)
    }

    return left.name.localeCompare(right.name)
  }
}

export function bucketDisplayValue(bucket: QuotaBucketDetail, unavailableText: string, unsupportedText: string) {
  if (!bucket.supported) {
    return unsupportedText
  }
  const value = bucketNumber(bucket.remainingPercent)
  if (value === null) {
    return unavailableText
  }
  const rounded = Math.abs(value - Math.round(value)) < 0.05 ? Math.round(value) : value.toFixed(1)
  return `${rounded}%`
}

export function bucketHeatColor(percent: number | null | undefined) {
  if (typeof percent !== 'number' || Number.isNaN(percent)) {
    return null
  }
  const clamped = Math.max(0, Math.min(100, percent))
  const low = { r: 193, g: 74, b: 56 }
  const mid = { r: 201, g: 154, b: 37 }
  const high = { r: 45, g: 139, b: 107 }

  let start = low
  let end = mid
  let ratio = clamped / 50
  if (clamped >= 50) {
    start = mid
    end = high
    ratio = (clamped - 50) / 50
  }

  const r = interpolateChannel(start.r, end.r, ratio)
  const g = interpolateChannel(start.g, end.g, ratio)
  const b = interpolateChannel(start.b, end.b, ratio)
  return `${r}, ${g}, ${b}`
}

export function bucketHeatPercent(bucket: QuotaBucketDetail) {
  if (!bucket.supported) {
    return null
  }
  const value = bucketNumber(bucket.remainingPercent)
  if (value === null) {
    return null
  }
  return Math.max(0, Math.min(100, value))
}
