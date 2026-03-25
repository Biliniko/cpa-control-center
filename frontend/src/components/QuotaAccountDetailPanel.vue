<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CodexQuotaAccountDetail, QuotaBucketDetail } from '@/types'
import { formatDateTime } from '@/utils/format'
import { bucketDisplayValue, bucketHeatColor, bucketHeatPercent, normalizeQuotaPlanType, quotaTotalRemaining } from '@/utils/quotas'

const props = defineProps<{
  account: CodexQuotaAccountDetail
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()

const bucketEntries = computed(() => [
  { key: 'fiveHour', label: t('quotas.buckets.fiveHour'), bucket: props.account.fiveHour },
  { key: 'weekly', label: t('quotas.buckets.weekly'), bucket: props.account.weekly },
  { key: 'codeReviewWeekly', label: t('quotas.buckets.codeReviewWeekly'), bucket: props.account.codeReviewWeekly },
])

const tokenEntries = computed(() => {
  const tokenUsage = props.account.tokenUsage ?? {}
  return [
    { key: 'total', label: t('quotas.tokens.total'), value: tokenUsage.totalTokens ?? null },
    { key: 'input', label: t('quotas.tokens.input'), value: tokenUsage.inputTokens ?? null },
    { key: 'output', label: t('quotas.tokens.output'), value: tokenUsage.outputTokens ?? null },
  ]
})

function planLabel(value: string) {
  const normalized = normalizeQuotaPlanType(value)
  if (normalized === 'unknown') {
    return t('common.unknown')
  }
  return normalized.slice(0, 1).toUpperCase() + normalized.slice(1)
}

function planToneClass(value: string) {
  return `quota-detail-modal__tag--plan-${normalizeQuotaPlanType(value)}`
}

function bucketValue(bucket: QuotaBucketDetail) {
  return bucketDisplayValue(bucket, t('quotas.unavailable'), t('quotas.matrix.unsupported'))
}

function bucketRingStyle(bucket: QuotaBucketDetail) {
  const percent = bucketHeatPercent(bucket)
  const color = bucketHeatColor(bucket.remainingPercent)
  if (percent === null || color === null) {
    return {
      '--quota-bucket-ring-percent': '0',
      '--quota-bucket-ring-color': 'rgba(255, 255, 255, 0.14)',
      '--quota-bucket-ring-glow': 'rgba(255, 255, 255, 0.05)',
    } as Record<string, string>
  }
  return {
    '--quota-bucket-ring-percent': `${percent}`,
    '--quota-bucket-ring-color': `rgb(${color})`,
    '--quota-bucket-ring-glow': `rgba(${color}, 0.18)`,
  } as Record<string, string>
}

function bucketRingClass(bucket: QuotaBucketDetail) {
  return {
    'quota-detail-modal__bucket--unsupported': !bucket.supported,
    'quota-detail-modal__bucket--unknown': bucket.supported && bucket.remainingPercent == null,
  }
}

function resetText(value: string) {
  return value ? formatDateTime(value) : t('common.notAvailable')
}

function statusLabel(account: CodexQuotaAccountDetail) {
  return account.success ? t('quotas.matrix.success') : t('quotas.matrix.failed')
}

function tokenValue(value?: number | null) {
  return typeof value === 'number' && !Number.isNaN(value) ? value.toLocaleString() : t('common.notAvailable')
}

function fetchedAtLabel() {
  return t('quotas.matrix.detailFetchedAt', { value: '' }).replace(/[:\uFF1A]\s*$/, '').trim()
}
</script>

<template>
  <Teleport to="body">
    <Transition appear name="quota-detail">
      <div class="quota-detail-overlay" @click.self="emit('close')">
        <article
          class="quota-detail-modal"
          :class="{
            'quota-detail-modal--success': account.success,
            'quota-detail-modal--failed': !account.success,
          }"
          @click.stop
        >
          <div class="quota-detail-modal__head">
            <div>
              <p class="panel-kicker">{{ t('quotas.matrix.detailTitle') }}</p>
              <h3 :title="account.name">{{ account.name }}</h3>
            </div>
            <button type="button" class="quota-detail-modal__close" @click="emit('close')">&times;</button>
          </div>

          <div class="quota-detail-modal__body">
            <div class="quota-detail-modal__field quota-detail-modal__field--email">
              <span class="quota-detail-modal__field-label">{{ t('accounts.columns.email') }}:</span>
              <span class="quota-detail-modal__field-value" :title="account.email || t('common.notAvailable')">{{ account.email || t('common.notAvailable') }}</span>
            </div>

            <div class="quota-detail-modal__meta">
              <span class="quota-detail-modal__tag quota-detail-modal__tag--plan" :class="planToneClass(account.planType)">
                <span class="quota-detail-modal__tag-key">{{ t('accounts.columns.plan') }}</span>
                <span class="quota-detail-modal__tag-value">{{ planLabel(account.planType) }}</span>
              </span>
              <span class="quota-detail-modal__tag quota-detail-modal__tag--status">
                <span class="quota-detail-modal__tag-key">{{ t('accounts.columns.state') }}</span>
                <span class="quota-detail-modal__tag-value">{{ statusLabel(account) }}</span>
              </span>
            </div>

            <div class="quota-detail-modal__headline">
              <strong>{{ t('quotas.matrix.totalRemaining', { value: Math.round(quotaTotalRemaining(account)) }) }}</strong>
              <span class="quota-detail-modal__headline-meta">
                <span class="quota-detail-modal__field-label">{{ t('quotas.resetAtShort') }}:</span>
                <span class="quota-detail-modal__field-value">{{ resetText(account.earliestResetAt) }}</span>
              </span>
            </div>

            <div v-if="!account.success" class="quota-detail-modal__error">
              <strong>{{ t('quotas.matrix.failureReason') }}</strong>
              <p>{{ account.error || t('common.notAvailable') }}</p>
            </div>

            <section class="quota-detail-modal__tokens" :aria-label="t('quotas.tokens.title')">
              <p class="panel-kicker quota-detail-modal__section-kicker">{{ t('quotas.tokens.title') }}</p>
              <div class="quota-detail-modal__tokens-grid">
                <article
                  v-for="entry in tokenEntries"
                  :key="entry.key"
                  class="quota-detail-modal__token-card"
                  :class="{ 'quota-detail-modal__token-card--unknown': entry.value == null }"
                >
                  <span class="quota-detail-modal__token-label">{{ entry.label }}</span>
                  <strong class="quota-detail-modal__token-value">{{ tokenValue(entry.value) }}</strong>
                </article>
              </div>
            </section>

            <div class="quota-detail-modal__buckets">
              <article
                v-for="entry in bucketEntries"
                :key="entry.key"
                class="quota-detail-modal__bucket"
                :class="bucketRingClass(entry.bucket)"
                :style="bucketRingStyle(entry.bucket)"
              >
                <strong class="quota-detail-modal__bucket-label">{{ entry.label }}</strong>
                <span class="quota-detail-modal__bucket-value">{{ bucketValue(entry.bucket) }}</span>
                <small class="quota-detail-modal__bucket-meta">
                  <span class="quota-detail-modal__field-label">{{ t('quotas.resetAtShort') }}:</span>
                  <span class="quota-detail-modal__field-value">{{ resetText(entry.bucket.resetAt) }}</span>
                </small>
              </article>
            </div>

            <p class="muted quota-detail-modal__fetched">
              <span class="quota-detail-modal__field-label">{{ fetchedAtLabel() }}:</span>
              <span class="quota-detail-modal__field-value">{{ resetText(account.fetchedAt) }}</span>
            </p>
          </div>
        </article>
      </div>
    </Transition>
  </Teleport>
</template>
