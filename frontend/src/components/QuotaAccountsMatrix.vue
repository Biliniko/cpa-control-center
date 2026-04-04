<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElPagination } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { CodexQuotaAccountDetail, QuotaBucketDetail } from '@/types'
import { formatResetCompact } from '@/utils/format'
import { bucketDisplayValue, bucketHeatColor, bucketHeatPercent, normalizeQuotaPlanType } from '@/utils/quotas'

const props = defineProps<{
  accounts: CodexQuotaAccountDetail[]
  total: number
  page: number
  pageSize: number
  pageSizes: number[]
  selectedAccountName: string
}>()

const emit = defineEmits<{
  (event: 'select-account', name: string): void
  (event: 'page-change', value: number): void
  (event: 'page-size-change', value: number): void
  (event: 'columns-change', value: number): void
}>()

const { t } = useI18n()
const gridRef = ref<HTMLElement | null>(null)
let gridResizeObserver: ResizeObserver | null = null

const successCount = computed(() => props.accounts.filter((a) => a.success).length)
const failedCount = computed(() => props.accounts.filter((a) => !a.success).length)
const rangeStart = computed(() => (props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1))
const rangeEnd = computed(() => Math.min(props.total, props.page * props.pageSize))

const bucketKeys = ['fiveHour', 'weekly', 'codeReviewWeekly'] as const
const bucketLabels = computed(() => ({
  fiveHour: t('quotas.buckets.fiveHourShort'),
  weekly: t('quotas.buckets.weeklyShort'),
  codeReviewWeekly: t('quotas.buckets.codeReviewWeeklyShort'),
}))

function bucketValue(account: CodexQuotaAccountDetail, key: 'fiveHour' | 'weekly' | 'codeReviewWeekly') {
  return bucketDisplayValue(account[key], t('quotas.unavailable'), t('quotas.matrix.unsupported'))
}

function heatStyle(bucket: QuotaBucketDetail) {
  const pct = bucketHeatPercent(bucket)
  const rgb = bucketHeatColor(bucket.remainingPercent)
  if (pct === null || rgb === null) {
    return {} as Record<string, string>
  }
  return {
    '--heat-color': `rgb(${rgb})`,
    '--heat-fill': `${pct}%`,
  } as Record<string, string>
}

function planLabel(value: string) {
  const normalized = normalizeQuotaPlanType(value)
  if (normalized === 'unknown') {
    return t('common.unknown')
  }
  return normalized.slice(0, 1).toUpperCase() + normalized.slice(1)
}

function emitColumns() {
  if (!gridRef.value) {
    return
  }
  const templateColumns = window.getComputedStyle(gridRef.value).gridTemplateColumns
  if (!templateColumns || templateColumns === 'none') {
    emit('columns-change', 1)
    return
  }
  const columns = templateColumns.split(' ').filter(Boolean).length
  emit('columns-change', Math.max(1, columns))
}

onMounted(() => {
  if (typeof ResizeObserver === 'undefined') {
    return
  }
  gridResizeObserver = new ResizeObserver(() => {
    emitColumns()
  })
  if (gridRef.value) {
    gridResizeObserver.observe(gridRef.value)
  }
})

onBeforeUnmount(() => {
  gridResizeObserver?.disconnect()
  gridResizeObserver = null
})

watch(gridRef, (value, previous) => {
  if (previous && gridResizeObserver) {
    gridResizeObserver.unobserve(previous)
  }
  if (value && gridResizeObserver) {
    gridResizeObserver.observe(value)
  }
  if (value) {
    void nextTick(() => {
      emitColumns()
    })
  }
})

watch(() => props.accounts.length, () => {
  void nextTick(() => {
    emitColumns()
  })
}, { immediate: true })
</script>

<template>
  <section class="quota-workspace-surface quota-workspace-surface--matrix">
    <article class="quota-workspace-panel quota-workspace-panel--dark">
      <div class="quota-workspace-panel__head">
        <div>
          <p class="panel-kicker">{{ t('quotas.matrix.matrixTitle') }}</p>
          <h3>{{ t('quotas.matrix.matrixHeadline') }}</h3>
        </div>
        <span class="muted quota-workspace-panel__summary">
          {{ t('quotas.paginationSummary', { from: rangeStart, to: rangeEnd, total }) }}
        </span>
      </div>

      <div class="quota-matrix-summary">
        <span>{{ t('quotas.matrix.success') }} {{ successCount }}</span>
        <span>{{ t('quotas.matrix.failed') }} {{ failedCount }}</span>
      </div>

      <div v-if="accounts.length > 0" ref="gridRef" class="quota-card-grid">
        <button
          v-for="account in accounts"
          :key="account.name"
          type="button"
          class="quota-card"
          :class="{
            'quota-card--active': account.name === selectedAccountName,
            'quota-card--failed': !account.success,
          }"
          @click="emit('select-account', account.name)"
        >
          <div class="quota-card__header">
            <strong class="quota-card__name">{{ account.name }}</strong>
            <small class="quota-card__plan">{{ account.email || planLabel(account.planType) }}</small>
          </div>
          <template v-if="account.success">
            <div class="quota-card__buckets">
              <div v-for="key in bucketKeys" :key="key" class="quota-card__bucket">
                <span class="quota-card__bucket-label">{{ bucketLabels[key] }}</span>
                <span class="quota-card__bucket-track" :style="heatStyle(account[key])">
                  <span class="quota-card__bucket-fill" />
                </span>
                <span class="quota-card__bucket-value">{{ bucketValue(account, key) }}</span>
              </div>
            </div>
            <div class="quota-card__footer">
              <small class="quota-card__reset">{{ account.earliestResetAt ? formatResetCompact(account.earliestResetAt) : t('common.notAvailable') }}</small>
            </div>
            <div v-if="account.error" class="quota-card__note">
              <small>{{ account.error }}</small>
            </div>
          </template>
          <template v-else>
            <div class="quota-card__error">
              <small>{{ account.error || t('quotas.matrix.failureReason') }}</small>
            </div>
          </template>
        </button>
      </div>
      <div v-else class="quota-workspace-panel__empty muted">
        {{ t('quotas.matrix.matrixEmptyBody') }}
      </div>

      <div class="quota-workspace-pagination">
        <el-pagination
          :current-page="page"
          :page-size="pageSize"
          background
          :page-sizes="pageSizes"
          :total="total"
          layout="total, prev, pager, next, jumper"
          @current-change="emit('page-change', $event)"
          @size-change="emit('page-size-change', $event)"
        />
      </div>
    </article>
  </section>
</template>
