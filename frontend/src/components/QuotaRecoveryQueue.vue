<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElPagination } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { CodexQuotaAccountDetail, QuotaBucketDetail, QuotaRecoveryMode } from '@/types'
import { formatResetCompact } from '@/utils/format'
import { bucketDisplayValue, bucketHeatColor, bucketHeatPercent, normalizeQuotaPlanType, quotaRecoveryResetAt } from '@/utils/quotas'

const props = defineProps<{
  accounts: CodexQuotaAccountDetail[]
  total: number
  page: number
  pageSize: number
  pageSizes: number[]
  recoveryMode: QuotaRecoveryMode
  selectedAccountName: string
}>()

const emit = defineEmits<{
  (event: 'select-account', name: string): void
  (event: 'page-change', value: number): void
  (event: 'columns-change', value: number): void
}>()

const { t } = useI18n()
const surfaceRef = ref<HTMLElement | null>(null)
let gridResizeObserver: ResizeObserver | null = null
const rangeStart = computed(() => (props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1))
const rangeEnd = computed(() => Math.min(props.total, props.page * props.pageSize))

const bucketKeys = ['fiveHour', 'weekly', 'codeReviewWeekly'] as const
const bucketLabels = computed(() => ({
  fiveHour: t('quotas.buckets.fiveHourShort'),
  weekly: t('quotas.buckets.weeklyShort'),
  codeReviewWeekly: t('quotas.buckets.codeReviewWeeklyShort'),
}))

const groupedAccounts = computed(() => {
  const lanes = [
    { key: 'immediate', label: t('quotas.recovery.bands.immediate'), items: [] as CodexQuotaAccountDetail[] },
    { key: 'today', label: t('quotas.recovery.bands.today'), items: [] as CodexQuotaAccountDetail[] },
    { key: 'soon', label: t('quotas.recovery.bands.soon'), items: [] as CodexQuotaAccountDetail[] },
    { key: 'later', label: t('quotas.recovery.bands.later'), items: [] as CodexQuotaAccountDetail[] },
    { key: 'unknown', label: t('quotas.recovery.bands.unknown'), items: [] as CodexQuotaAccountDetail[] },
  ]
  const laneMap = new Map(lanes.map((lane) => [lane.key, lane]))
  const now = Date.now()

  props.accounts.forEach((account) => {
    let laneKey = 'unknown'
    const resetAt = quotaRecoveryResetAt(account, props.recoveryMode)
    if (account.success && resetAt) {
      const resetTime = Date.parse(resetAt)
      if (!Number.isNaN(resetTime)) {
        const diffHours = (resetTime - now) / 3_600_000
        if (diffHours <= 6) {
          laneKey = 'immediate'
        } else if (diffHours <= 24) {
          laneKey = 'today'
        } else if (diffHours <= 72) {
          laneKey = 'soon'
        } else {
          laneKey = 'later'
        }
      }
    }
    laneMap.get(laneKey)?.items.push(account)
  })

  return lanes.filter((lane) => lane.items.length > 0)
})

function planLabel(value: string) {
  const normalized = normalizeQuotaPlanType(value)
  if (normalized === 'unknown') {
    return t('common.unknown')
  }
  return normalized.slice(0, 1).toUpperCase() + normalized.slice(1)
}

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

function emitColumns() {
  const grid = surfaceRef.value?.querySelector<HTMLElement>('.quota-card-grid')
  if (!grid) {
    emit('columns-change', 1)
    return
  }
  const templateColumns = window.getComputedStyle(grid).gridTemplateColumns
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
  if (surfaceRef.value) {
    gridResizeObserver.observe(surfaceRef.value)
  }
})

onBeforeUnmount(() => {
  gridResizeObserver?.disconnect()
  gridResizeObserver = null
})

watch(surfaceRef, (value, previous) => {
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
  <section ref="surfaceRef" class="quota-workspace-surface quota-workspace-surface--recovery">
    <article class="quota-workspace-panel quota-workspace-panel--dark">
      <div class="quota-workspace-panel__head">
        <div>
          <p class="panel-kicker">{{ t('quotas.recovery.title') }}</p>
          <h3>{{ t('quotas.recovery.headline') }}</h3>
        </div>
        <span class="muted quota-workspace-panel__summary">
          {{ t('quotas.paginationSummary', { from: rangeStart, to: rangeEnd, total }) }}
        </span>
      </div>

      <div v-if="groupedAccounts.length > 0" class="quota-recovery-ledger">
        <section v-for="group in groupedAccounts" :key="group.key" class="quota-recovery-ledger__section">
          <header class="quota-recovery-ledger__head">
            <strong>{{ group.label }}</strong>
            <span>{{ group.items.length }}</span>
          </header>

          <div class="quota-card-grid">
            <button
              v-for="account in group.items"
              :key="account.name"
              type="button"
              class="quota-card"
              :class="{ 'quota-card--active': account.name === selectedAccountName }"
              @click="emit('select-account', account.name)"
            >
              <div class="quota-card__header">
                <strong class="quota-card__name">{{ account.name }}</strong>
                <small class="quota-card__plan">{{ account.email || planLabel(account.planType) }}</small>
              </div>
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
                <small class="quota-card__reset">{{ quotaRecoveryResetAt(account, recoveryMode) ? formatResetCompact(quotaRecoveryResetAt(account, recoveryMode)) : t('common.notAvailable') }}</small>
              </div>
            </button>
          </div>
        </section>
      </div>
      <div v-else class="quota-workspace-panel__empty muted">
        {{ t('quotas.recovery.emptyBody') }}
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
        />
      </div>
    </article>
  </section>
</template>
