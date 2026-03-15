<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CodexPlanQuotaSummary, QuotaBucketSummary } from '@/types'
import { formatDateTime } from '@/utils/format'
import { quotaAverageRemainingPercent, quotaCapacity, quotaMeterColor, quotaNormalizedFill } from '@/utils/quotas'

defineProps<{
  plans: CodexPlanQuotaSummary[]
}>()

const { t } = useI18n()

function formatTotalRemainingPercent(value?: number | null) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return t('quotas.unavailable')
  }
  const rounded = Math.abs(value - Math.round(value)) < 0.05 ? Math.round(value) : value.toFixed(1)
  return t('quotas.totalRemainingPercent', { value: rounded })
}

function coverageLabel(successCount: number, failedCount: number) {
  return t('quotas.coverage', { success: successCount, total: successCount + failedCount })
}

function formatAverageRemaining(bucket: QuotaBucketSummary) {
  const average = quotaAverageRemainingPercent(bucket)
  if (typeof average !== 'number' || Number.isNaN(average)) {
    return t('quotas.unavailable')
  }
  const rounded = Math.abs(average - Math.round(average)) < 0.05 ? Math.round(average) : average.toFixed(1)
  return t('quotas.averageRemainingPercent', { value: rounded })
}

function formatCapacity(bucket: QuotaBucketSummary) {
  return t('quotas.capacityPercent', { value: quotaCapacity(bucket) })
}

function formatResetAt(value: string) {
  return value ? formatDateTime(value) : t('common.notAvailable')
}
</script>

<template>
  <section class="quota-grid">
    <article v-for="plan in plans" :key="plan.planType" class="panel quota-plan-card">
      <div class="panel-head panel-head--tight">
        <div>
          <p class="panel-kicker">{{ t('quotas.planLabel') }}</p>
          <h3>{{ plan.planType }}</h3>
        </div>
        <span class="quota-plan-card__count">
          {{ t('quotas.planAccounts', { count: plan.accountCount }) }}
        </span>
      </div>

      <div class="quota-plan-card__buckets quota-plan-card__buckets--visual">
        <section v-if="plan.fiveHour.supported" class="quota-bucket">
          <div class="quota-bucket__head">
            <strong>{{ t('quotas.buckets.fiveHour') }}</strong>
            <span>{{ coverageLabel(plan.fiveHour.successCount, plan.fiveHour.failedCount) }}</span>
          </div>
          <div class="quota-bucket__value quota-bucket__value--hero">
            {{ formatTotalRemainingPercent(plan.fiveHour.totalRemainingPercent) }}
          </div>
          <div class="quota-bucket__meter" aria-hidden="true">
            <span class="quota-bucket__meter-fill" :style="{ width: `${quotaNormalizedFill(plan.fiveHour)}%`, backgroundColor: quotaMeterColor(plan.fiveHour) }" />
          </div>
          <div class="quota-bucket__stats muted">
            <span>{{ formatAverageRemaining(plan.fiveHour) }}</span>
            <span>{{ formatCapacity(plan.fiveHour) }}</span>
          </div>
          <p class="muted quota-bucket__reset">
            {{ t('quotas.resetAt', { value: formatResetAt(plan.fiveHour.resetAt) }) }}
          </p>
        </section>

        <section class="quota-bucket">
          <div class="quota-bucket__head">
            <strong>{{ t('quotas.buckets.weekly') }}</strong>
            <span>{{ coverageLabel(plan.weekly.successCount, plan.weekly.failedCount) }}</span>
          </div>
          <div class="quota-bucket__value quota-bucket__value--hero">
            {{ formatTotalRemainingPercent(plan.weekly.totalRemainingPercent) }}
          </div>
          <div class="quota-bucket__meter" aria-hidden="true">
            <span class="quota-bucket__meter-fill" :style="{ width: `${quotaNormalizedFill(plan.weekly)}%`, backgroundColor: quotaMeterColor(plan.weekly) }" />
          </div>
          <div class="quota-bucket__stats muted">
            <span>{{ formatAverageRemaining(plan.weekly) }}</span>
            <span>{{ formatCapacity(plan.weekly) }}</span>
          </div>
          <p class="muted quota-bucket__reset">
            {{ t('quotas.resetAt', { value: formatResetAt(plan.weekly.resetAt) }) }}
          </p>
        </section>

        <section class="quota-bucket">
          <div class="quota-bucket__head">
            <strong>{{ t('quotas.buckets.codeReviewWeekly') }}</strong>
            <span>{{ coverageLabel(plan.codeReviewWeekly.successCount, plan.codeReviewWeekly.failedCount) }}</span>
          </div>
          <div class="quota-bucket__value quota-bucket__value--hero">
            {{ formatTotalRemainingPercent(plan.codeReviewWeekly.totalRemainingPercent) }}
          </div>
          <div class="quota-bucket__meter" aria-hidden="true">
            <span class="quota-bucket__meter-fill" :style="{ width: `${quotaNormalizedFill(plan.codeReviewWeekly)}%`, backgroundColor: quotaMeterColor(plan.codeReviewWeekly) }" />
          </div>
          <div class="quota-bucket__stats muted">
            <span>{{ formatAverageRemaining(plan.codeReviewWeekly) }}</span>
            <span>{{ formatCapacity(plan.codeReviewWeekly) }}</span>
          </div>
          <p class="muted quota-bucket__reset">
            {{ t('quotas.resetAt', { value: formatResetAt(plan.codeReviewWeekly.resetAt) }) }}
          </p>
        </section>
      </div>
    </article>
  </section>
</template>
