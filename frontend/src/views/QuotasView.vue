<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElButton, ElMessage, ElOption, ElSelect } from 'element-plus'
import { useI18n } from 'vue-i18n'
import QuotaAccountDetailPanel from '@/components/QuotaAccountDetailPanel.vue'
import QuotaAccountsMatrix from '@/components/QuotaAccountsMatrix.vue'
import QuotaPlanOverview from '@/components/QuotaPlanOverview.vue'
import QuotaRecoveryQueue from '@/components/QuotaRecoveryQueue.vue'
import QuotaViewTabs from '@/components/QuotaViewTabs.vue'
import { useQuotasStore } from '@/stores/quotas'
import { useSettingsStore } from '@/stores/settings'
import { useTasksStore } from '@/stores/tasks'
import type { QuotaRecoveryMode, QuotaViewMode } from '@/types'
import { formatDateTime } from '@/utils/format'
import { toErrorMessage } from '@/utils/errors'
import { compareQuotaAccounts, normalizeQuotaPlanType, quotaPlanRank, quotaRecoveryResetAt } from '@/utils/quotas'

const { t } = useI18n()
const quotasStore = useQuotasStore()
const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const matrixRowOptions = [2, 3, 4, 5, 6]
const usageAutoRefreshIntervalMs = 15000
const matrixColumns = ref(6)
const recoveryColumns = ref(3)
let usageAutoRefreshTimer: number | null = null
const recoveryModeOptions = computed<Array<{ value: QuotaRecoveryMode; label: string }>>(() => [
  { value: 'earliest', label: t('quotas.recovery.filters.earliest') },
  { value: 'fiveHour', label: t('quotas.recovery.filters.fiveHour') },
  { value: 'weekly', label: t('quotas.recovery.filters.weekly') },
])

const tabItems = computed<Array<{ key: QuotaViewMode; label: string; caption: string }>>(() => [
  { key: 'overview', label: t('quotas.views.overview'), caption: t('quotas.views.overviewCaption') },
  { key: 'matrix', label: t('quotas.views.matrix'), caption: t('quotas.views.matrixCaption') },
  { key: 'recovery', label: t('quotas.views.recovery'), caption: t('quotas.views.recoveryCaption') },
])

const snapshot = computed(() => quotasStore.snapshot)
const usageSummary = computed(() => quotasStore.usageSummary)
const plans = computed(() => quotasStore.plans)
const accountDetails = computed(() => quotasStore.accountDetails)
const hasData = computed(() => plans.value.length > 0)
const hasDetailData = computed(() => quotasStore.hasDetailData)
const hasAnyResults = computed(() => hasData.value || hasDetailData.value)
const hasRequested = computed(() => quotasStore.hasRequested)
const lastFetchedLabel = computed(() => (
  quotasStore.lastFetchedAt ? formatDateTime(quotasStore.lastFetchedAt) : t('common.notAvailable')
))
const showPartialWarning = computed(() => (
  Boolean(snapshot.value && snapshot.value.failedAccounts > 0 && snapshot.value.successfulAccounts > 0)
))
const isFailureOnlySnapshot = computed(() => (
  Boolean(snapshot.value && snapshot.value.failedAccounts > 0 && snapshot.value.successfulAccounts === 0)
))
const hasUsageConnection = computed(() => (
  Boolean(settingsStore.settings.baseUrl.trim() && settingsStore.settings.managementToken.trim())
))
const usageLastFetchedLabel = computed(() => (
  usageSummary.value?.fetchedAt ? formatDateTime(usageSummary.value.fetchedAt) : t('common.notAvailable')
))
const usageLastRequestLabel = computed(() => (
  usageSummary.value?.lastRequestAt ? formatDateTime(usageSummary.value.lastRequestAt) : t('common.notAvailable')
))
const usageSuccessRateLabel = computed(() => {
  if (!usageSummary.value || usageSummary.value.totalRequests <= 0) {
    return t('common.notAvailable')
  }
  return `${((usageSummary.value.successCount / usageSummary.value.totalRequests) * 100).toFixed(1)}%`
})
const usageCards = computed(() => {
  const summary = usageSummary.value
  const tokens = summary?.tokens ?? {
    totalTokens: 0,
    inputTokens: 0,
    outputTokens: 0,
    reasoningTokens: 0,
    cachedTokens: 0,
  }
  const hasSummary = Boolean(summary)
  return [
    { key: 'totalTokens', label: t('usage.cards.totalTokens'), value: hasSummary ? formatInteger(tokens.totalTokens) : t('common.notAvailable'), hero: true },
    { key: 'todayTokens', label: t('usage.cards.todayTokens'), value: hasSummary ? formatInteger(summary?.todayTokens) : t('common.notAvailable'), hero: false },
    { key: 'inputTokens', label: t('usage.cards.inputTokens'), value: hasSummary ? formatInteger(tokens.inputTokens) : t('common.notAvailable'), hero: false },
    { key: 'outputTokens', label: t('usage.cards.outputTokens'), value: hasSummary ? formatInteger(tokens.outputTokens) : t('common.notAvailable'), hero: false },
    { key: 'totalRequests', label: t('usage.cards.totalRequests'), value: hasSummary ? formatInteger(summary?.totalRequests) : t('common.notAvailable'), hero: false },
    { key: 'successRate', label: t('usage.cards.successRate'), value: hasSummary ? usageSuccessRateLabel.value : t('common.notAvailable'), hero: false },
  ]
})
const usageMetaItems = computed(() => {
  const summary = usageSummary.value
  const hasSummary = Boolean(summary)
  return [
    { key: 'lastRequest', label: t('usage.meta.lastRequest'), value: hasSummary ? usageLastRequestLabel.value : t('common.notAvailable') },
    { key: 'apiKeys', label: t('usage.meta.apiKeys'), value: hasSummary ? formatInteger(summary?.apiKeyCount) : t('common.notAvailable') },
    { key: 'models', label: t('usage.meta.models'), value: hasSummary ? formatInteger(summary?.modelCount) : t('common.notAvailable') },
    { key: 'failedRequests', label: t('usage.meta.failedRequests'), value: hasSummary ? formatInteger(summary?.failedRequests) : t('common.notAvailable') },
  ]
})

const quotaProgress = computed(() => tasksStore.quota)
const isRefreshing = computed(() => tasksStore.quota.active)
const progressPercent = computed(() => {
  const { current, total } = quotaProgress.value
  if (total <= 0) return 0
  return Math.round((current / total) * 100)
})

const planOptions = computed(() => {
  const plansSet = new Set(accountDetails.value.map((account) => normalizeQuotaPlanType(account.planType)))
  return Array.from(plansSet.values()).sort((left, right) => quotaPlanRank(left) - quotaPlanRank(right) || left.localeCompare(right))
})

const filteredPlans = computed(() => (
  quotasStore.planFilter === 'all'
    ? plans.value
    : plans.value.filter((plan) => normalizeQuotaPlanType(plan.planType) === quotasStore.planFilter)
))

const filteredAccounts = computed(() => accountDetails.value.filter((account) => {
  if (quotasStore.planFilter !== 'all' && normalizeQuotaPlanType(account.planType) !== quotasStore.planFilter) {
    return false
  }
  if (quotasStore.resultFilter === 'success' && !account.success) {
    return false
  }
  if (quotasStore.resultFilter === 'failed' && account.success) {
    return false
  }
  return true
}))

const matrixAccounts = computed(() => filteredAccounts.value.slice().sort(compareQuotaAccounts(quotasStore.sortMode)))
const matrixTotal = computed(() => matrixAccounts.value.length)
const matrixEffectivePageSize = computed(() => Math.max(1, matrixColumns.value * quotasStore.matrixRows))
const matrixPageCount = computed(() => Math.max(1, Math.ceil(matrixTotal.value / matrixEffectivePageSize.value)))
const pagedMatrixAccounts = computed(() => {
  const page = Math.min(quotasStore.matrixPage, matrixPageCount.value)
  const start = (page - 1) * matrixEffectivePageSize.value
  return matrixAccounts.value.slice(start, start + matrixEffectivePageSize.value)
})

const recoveryAccounts = computed(() => filteredAccounts.value.slice().sort((left, right) => {
  const leftResetAt = quotaRecoveryResetAt(left, quotasStore.recoveryMode)
  const rightResetAt = quotaRecoveryResetAt(right, quotasStore.recoveryMode)
  if (leftResetAt && rightResetAt && leftResetAt !== rightResetAt) {
    return leftResetAt.localeCompare(rightResetAt)
  }
  if (leftResetAt !== rightResetAt) {
    return leftResetAt ? -1 : 1
  }
  return compareQuotaAccounts('plan')(left, right)
}))
const recoveryTotal = computed(() => recoveryAccounts.value.length)
const recoveryEffectivePageSize = computed(() => Math.max(1, recoveryColumns.value * quotasStore.recoveryRows))
const recoveryPageCount = computed(() => Math.max(1, Math.ceil(recoveryTotal.value / recoveryEffectivePageSize.value)))
const pagedRecoveryAccounts = computed(() => {
  const page = Math.min(quotasStore.recoveryPage, recoveryPageCount.value)
  const start = (page - 1) * recoveryEffectivePageSize.value
  return recoveryAccounts.value.slice(start, start + recoveryEffectivePageSize.value)
})

const selectedAccount = computed(() => (
  filteredAccounts.value.find((account) => account.name === quotasStore.selectedAccountName) ?? null
))

function closeDetail() {
  quotasStore.setSelectedAccount('')
}

function formatInteger(value?: number | null) {
  return typeof value === 'number' && Number.isFinite(value) ? Math.round(value).toLocaleString() : t('common.notAvailable')
}

watch(matrixPageCount, (count) => {
  if (quotasStore.matrixPage > count) {
    quotasStore.setMatrixPage(count)
  }
})

watch(recoveryPageCount, (count) => {
  if (quotasStore.recoveryPage > count) {
    quotasStore.setRecoveryPage(count)
  }
})

watch(filteredAccounts, (accounts) => {
  if (!accounts.some((account) => account.name === quotasStore.selectedAccountName)) {
    quotasStore.setSelectedAccount('')
  }
})

async function refreshUsageSummary(silent = false) {
  if (!hasUsageConnection.value) {
    return
  }
  try {
    await quotasStore.refreshUsageSummary(silent)
  } catch (error) {
    if (!silent) {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

function stopUsageAutoRefresh() {
  if (usageAutoRefreshTimer !== null) {
    window.clearInterval(usageAutoRefreshTimer)
    usageAutoRefreshTimer = null
  }
}

function syncUsageAutoRefresh() {
  stopUsageAutoRefresh()
  if (!hasUsageConnection.value) {
    return
  }
  usageAutoRefreshTimer = window.setInterval(() => {
    if (document.hidden || quotasStore.usageLoading) {
      return
    }
    void refreshUsageSummary(true)
  }, usageAutoRefreshIntervalMs)
}

async function refreshSnapshot() {
  try {
    const [latestSnapshot] = await Promise.all([
      quotasStore.refreshSnapshot(),
      hasUsageConnection.value ? quotasStore.refreshUsageSummary(true).catch(() => null) : Promise.resolve(null),
    ])
    if (
      latestSnapshot.failedAccounts > 0 &&
      latestSnapshot.successfulAccounts === 0 &&
      quotasStore.activeView === 'overview'
    ) {
      quotasStore.setActiveView('matrix')
    }
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

watch(hasUsageConnection, () => {
  syncUsageAutoRefresh()
})

onMounted(() => {
  syncUsageAutoRefresh()
  if (hasUsageConnection.value && !usageSummary.value && !quotasStore.usageLoading) {
    void refreshUsageSummary(true)
  }
})

onUnmounted(() => {
  stopUsageAutoRefresh()
})
</script>

<template>
  <div class="view-shell view-shell--quotas">
    <section class="hero-panel quota-hero">
      <div>
        <p class="eyebrow">{{ t('quotas.eyebrow') }}</p>
        <h2>{{ t('quotas.title') }}</h2>
        <p class="lead">
          {{ t('quotas.lead') }}
        </p>
      </div>
      <div class="quota-hero__actions">
        <div class="quota-hero__meta muted">
          <span>{{ t('quotas.lastUpdated', { value: lastFetchedLabel }) }}</span>
          <span v-if="snapshot">
            {{ t('quotas.accountsSummary', { total: snapshot.totalAccounts, success: snapshot.successfulAccounts, failed: snapshot.failedAccounts }) }}
          </span>
        </div>
        <div class="quota-hero__buttons">
          <el-button :loading="quotasStore.loading" @click="refreshSnapshot">
            {{ t('quotas.refresh') }}
          </el-button>
          <div v-if="isRefreshing && hasAnyResults" class="quota-progress-inline">
            <div class="quota-progress-inline__text">
              <span>{{ quotaProgress.message }}</span>
              <span v-if="quotaProgress.total > 0" class="quota-progress-inline__counter">{{ quotaProgress.current }}/{{ quotaProgress.total }}</span>
            </div>
            <div class="quota-progress-inline__track">
              <span class="quota-progress-inline__bar" :style="{ width: `${progressPercent}%` }" />
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="panel usage-summary-panel">
      <div class="usage-summary-panel__head">
        <div>
          <p class="panel-kicker">{{ t('usage.kicker') }}</p>
          <h3>{{ t('usage.title') }}</h3>
          <p class="muted usage-summary-panel__lead">{{ t('usage.lead') }}</p>
        </div>
        <div class="usage-summary-panel__actions">
          <span class="muted usage-summary-panel__hint">{{ t('usage.autoRefreshHint') }}</span>
          <div class="usage-summary-panel__controls">
            <span class="muted">{{ t('usage.lastUpdated', { value: usageLastFetchedLabel }) }}</span>
            <el-button :loading="quotasStore.usageLoading" :disabled="!hasUsageConnection" @click="refreshUsageSummary(false)">
              {{ t('usage.refresh') }}
            </el-button>
          </div>
        </div>
      </div>

      <div v-if="!hasUsageConnection" class="usage-summary-panel__empty muted">
        {{ t('usage.unconfigured') }}
      </div>

      <template v-else>
        <div v-if="quotasStore.usageError" class="quota-callout" :class="usageSummary ? 'quota-callout--warning' : 'quota-callout--error'">
          <strong>{{ usageSummary ? t('usage.staleWarningTitle') : t('usage.loadFailedTitle') }}</strong>
          <p v-if="usageSummary" class="muted">{{ t('usage.staleWarningBody') }}</p>
          <p class="muted">{{ quotasStore.usageError }}</p>
        </div>

        <p v-else-if="quotasStore.usageLoading && !usageSummary" class="muted usage-summary-panel__status">
          {{ t('usage.loading') }}
        </p>

        <p v-else-if="!usageSummary" class="muted usage-summary-panel__status">
          {{ t('usage.empty') }}
        </p>

        <div class="usage-summary-panel__grid">
          <article
            v-for="card in usageCards"
            :key="card.key"
            class="usage-summary-panel__card"
            :class="{ 'usage-summary-panel__card--hero': card.hero }"
          >
            <span class="usage-summary-panel__card-label">{{ card.label }}</span>
            <strong class="usage-summary-panel__card-value">{{ card.value }}</strong>
          </article>
        </div>

        <div class="usage-summary-panel__meta-list">
          <span v-for="item in usageMetaItems" :key="item.key" class="usage-summary-panel__meta-pill">
            <strong>{{ item.label }}</strong>
            <span>{{ item.value }}</span>
          </span>
        </div>
      </template>
    </section>

    <QuotaViewTabs v-model="quotasStore.activeView" :items="tabItems" />

    <article v-if="quotasStore.error && !hasData" class="panel quota-callout quota-callout--error">
      <strong>{{ t('quotas.loadFailedTitle') }}</strong>
      <p class="muted">{{ quotasStore.error }}</p>
    </article>

    <article v-if="quotasStore.loading && !hasAnyResults" class="panel panel--fill quota-progress-center">
      <div class="quota-progress-center__body">
        <p class="panel-kicker">{{ t('quotas.eyebrow') }}</p>
        <h3>{{ t('common.loading') }}</h3>
        <p class="muted quota-progress-center__message">{{ quotaProgress.message || t('quotas.loading') }}</p>
        <div v-if="quotaProgress.total > 0" class="quota-progress-center__stats">
          <span>{{ quotaProgress.current }} / {{ quotaProgress.total }}</span>
          <span>{{ progressPercent }}%</span>
        </div>
        <div class="quota-progress-center__track">
          <span class="quota-progress-center__bar" :style="{ width: quotaProgress.total > 0 ? `${progressPercent}%` : undefined }" :class="{ 'quota-progress-center__bar--indeterminate': quotaProgress.total <= 0 }" />
        </div>
      </div>
    </article>

    <article v-else-if="!hasRequested" class="panel panel--fill quota-empty-state">
      <div class="panel__body quota-empty-state__body">
        <strong>{{ t('quotas.clickRefreshTitle') }}</strong>
        <p class="muted">{{ t('quotas.clickRefreshBody') }}</p>
        <el-button type="primary" :loading="quotasStore.loading" @click="refreshSnapshot">
          {{ t('quotas.refresh') }}
        </el-button>
      </div>
    </article>

    <article v-else-if="!hasAnyResults" class="panel panel--fill">
      <div class="panel-head panel-head--tight">
        <div>
          <p class="panel-kicker">{{ t('quotas.eyebrow') }}</p>
          <h3>{{ t('quotas.emptyTitle') }}</h3>
        </div>
      </div>
      <div class="panel__body muted">
        {{ t('quotas.emptyBody') }}
      </div>
    </article>

    <template v-else>
      <section class="panel quota-workspace-toolbar">
        <div class="quota-workspace-toolbar__controls">
          <div class="quota-workspace-toolbar__group">
            <span class="quota-workspace-toolbar__label">{{ t('quotas.filters.plan') }}</span>
            <el-select :model-value="quotasStore.planFilter" @change="quotasStore.setPlanFilter($event)">
              <el-option :label="t('quotas.filters.allPlans')" value="all" />
              <el-option v-for="plan in planOptions" :key="plan" :label="plan" :value="plan" />
            </el-select>
          </div>

          <template v-if="quotasStore.activeView !== 'overview'">
            <div class="quota-workspace-toolbar__group quota-workspace-toolbar__group--chips">
              <span class="quota-workspace-toolbar__label">{{ t('quotas.filters.result') }}</span>
              <div class="quota-workspace-toolbar__chips">
                <button type="button" class="quota-workspace-chip" :class="{ 'quota-workspace-chip--active': quotasStore.resultFilter === 'all' }" @click="quotasStore.setResultFilter('all')">
                  {{ t('quotas.filters.results.all') }}
                </button>
                <button type="button" class="quota-workspace-chip" :class="{ 'quota-workspace-chip--active': quotasStore.resultFilter === 'success' }" @click="quotasStore.setResultFilter('success')">
                  {{ t('quotas.filters.results.success') }}
                </button>
                <button type="button" class="quota-workspace-chip" :class="{ 'quota-workspace-chip--active': quotasStore.resultFilter === 'failed' }" @click="quotasStore.setResultFilter('failed')">
                  {{ t('quotas.filters.results.failed') }}
                </button>
              </div>
            </div>

            <div v-if="quotasStore.activeView === 'matrix'" class="quota-workspace-toolbar__group">
              <span class="quota-workspace-toolbar__label">{{ t('quotas.filters.sort') }}</span>
              <el-select :model-value="quotasStore.sortMode" @change="quotasStore.setSortMode($event)">
                <el-option :label="t('quotas.filters.sorts.plan')" value="plan" />
                <el-option :label="t('quotas.filters.sorts.total')" value="total" />
                <el-option :label="t('quotas.filters.sorts.fiveHour')" value="fiveHour" />
                <el-option :label="t('quotas.filters.sorts.weekly')" value="weekly" />
                <el-option :label="t('quotas.filters.sorts.reset')" value="reset" />
                <el-option :label="t('quotas.filters.sorts.name')" value="name" />
              </el-select>
            </div>

            <div v-if="quotasStore.activeView === 'recovery'" class="quota-workspace-toolbar__group">
              <span class="quota-workspace-toolbar__label">{{ t('quotas.recovery.filters.label') }}</span>
              <el-select :model-value="quotasStore.recoveryMode" @change="quotasStore.setRecoveryMode($event)">
                <el-option
                  v-for="option in recoveryModeOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </div>

            <div class="quota-workspace-toolbar__group">
              <span class="quota-workspace-toolbar__label">{{ t('quotas.filters.rows') }}</span>
              <el-select
                :model-value="quotasStore.activeView === 'matrix' ? quotasStore.matrixRows : quotasStore.recoveryRows"
                @change="quotasStore.activeView === 'matrix' ? quotasStore.setMatrixRows($event) : quotasStore.setRecoveryRows($event)"
              >
                <el-option
                  v-for="size in matrixRowOptions"
                  :key="size"
                  :label="t('quotas.filters.rowsValue', { value: size })"
                  :value="size"
                />
              </el-select>
            </div>
          </template>
        </div>

        <div class="quota-workspace-toolbar__meta">
          <div v-if="quotasStore.activeView === 'overview'" class="quota-workspace-toolbar__summary">
            <span>{{ t('quotas.planCountSummary', { count: filteredPlans.length }) }}</span>
            <span v-if="snapshot">{{ t('quotas.accountsSummary', { total: snapshot.totalAccounts, success: snapshot.successfulAccounts, failed: snapshot.failedAccounts }) }}</span>
          </div>

          <div v-if="showPartialWarning" class="quota-workspace-toolbar__status quota-workspace-toolbar__status--warning">
            <strong>{{ t('quotas.partialWarningTitle') }}</strong>
            <span class="muted">{{ t('quotas.partialWarningBody', { failed: snapshot?.failedAccounts ?? 0 }) }}</span>
          </div>
        </div>
      </section>

      <article v-if="quotasStore.activeView === 'overview' && filteredPlans.length === 0" class="panel panel--fill quota-empty-state">
        <div class="panel__body quota-empty-state__body">
          <strong>{{ isFailureOnlySnapshot ? t('quotas.failureOnlyTitle') : t('quotas.overviewEmpty') }}</strong>
          <p class="muted">
            {{ isFailureOnlySnapshot ? t('quotas.failureOnlyBody', { failed: snapshot?.failedAccounts ?? 0 }) : t('quotas.overviewEmpty') }}
          </p>
          <el-button v-if="isFailureOnlySnapshot" type="primary" @click="quotasStore.setActiveView('matrix')">
            {{ t('quotas.openMatrix') }}
          </el-button>
        </div>
      </article>

      <QuotaPlanOverview v-else-if="quotasStore.activeView === 'overview'" :plans="filteredPlans" />

      <section v-else class="quota-workspace-layout">
        <QuotaAccountsMatrix
          v-if="quotasStore.activeView === 'matrix'"
          :accounts="pagedMatrixAccounts"
          :total="matrixTotal"
          :page="quotasStore.matrixPage"
          :page-size="matrixEffectivePageSize"
          :page-sizes="matrixRowOptions.map((rows) => Math.max(1, rows * matrixColumns))"
          :selected-account-name="quotasStore.selectedAccountName"
          @select-account="quotasStore.setSelectedAccount"
          @columns-change="matrixColumns = $event"
          @page-change="quotasStore.setMatrixPage"
        />

        <QuotaRecoveryQueue
          v-else
          :accounts="pagedRecoveryAccounts"
          :total="recoveryTotal"
          :page="quotasStore.recoveryPage"
          :page-size="recoveryEffectivePageSize"
          :page-sizes="matrixRowOptions.map((rows) => Math.max(1, rows * recoveryColumns))"
          :recovery-mode="quotasStore.recoveryMode"
          :selected-account-name="quotasStore.selectedAccountName"
          @select-account="quotasStore.setSelectedAccount"
          @columns-change="recoveryColumns = $event"
          @page-change="quotasStore.setRecoveryPage"
        />
      </section>

      <QuotaAccountDetailPanel v-if="selectedAccount" :account="selectedAccount" @close="closeDetail" />
    </template>
  </div>
</template>
