<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch, ref } from 'vue'
import {
  ElButton,
  ElDrawer,
  ElMessage,
  ElMessageBox,
  ElPagination,
  ElTable,
  ElTableColumn,
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import SummaryDonut from '@/components/SummaryDonut.vue'
import StatusPill from '@/components/StatusPill.vue'
import { useAccountsStore } from '@/stores/accounts'
import { useSettingsStore } from '@/stores/settings'
import { useTasksStore } from '@/stores/tasks'
import { formatDateTime } from '@/utils/format'
import { toErrorMessage } from '@/utils/errors'
import { quotaActionLabel, taskStatusLabel } from '@/utils/status'
import { emitDebug, emitDebugError } from '@/utils/debug'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const drawerOpen = ref(false)
const detailLoading = ref(false)
const detailRunId = ref<number | null>(null)
const detailPage = ref(1)
const detailPageSize = ref(20)
const detailPageSizes = [20, 50, 100]

const scanDetailSummary = computed(() => accountsStore.scanDetail?.summary ?? null)
const scanDetailRecords = computed(() => accountsStore.scanDetail?.records ?? [])
const scanDetailTotal = computed(() => accountsStore.scanDetail?.totalRecords ?? 0)

const scanDetailStatusState = computed(() => {
  const status = scanDetailSummary.value?.status?.toLowerCase() || ''

  switch (status) {
    case 'success':
      return 'normal'
    case 'failed':
    case 'cancelled':
      return 'error'
    default:
      return 'pending'
  }
})

const historyRows = computed(() => accountsStore.history.map((item) => ({
  ...item,
  statusLabel: taskStatusLabel(item.status),
  finishedAtLabel: formatDateTime(item.finishedAt),
})))

onMounted(() => {
  emitDebug('dashboard', 'mounted', {
    filtered: accountsStore.summary.filteredAccounts,
    pending: accountsStore.summary.pendingCount,
    history: historyRows.value.length,
  })
})

onUnmounted(() => {
  emitDebug('dashboard', 'unmounted')
})

watch(
  () => [
    accountsStore.summary.filteredAccounts,
    accountsStore.summary.pendingCount,
    accountsStore.summary.lastScanAt,
    historyRows.value.length,
  ],
  ([filtered, pending, lastScanAt, historyCount]) => {
    emitDebug('dashboard', 'summary changed', {
      filtered,
      pending,
      lastScanAt,
      historyCount,
    })
  },
  { immediate: true },
)

async function runScan() {
  try {
    await tasksStore.runScan()
    ElMessage.success(t('dashboard.scanCompleted'))
  } catch (error) {
    emitDebugError('dashboard', 'runScan failed', error)
    ElMessage.error(toErrorMessage(error))
  }
}

async function runMaintain() {
  const settings = settingsStore.settings
  const previewLines = [
    t('dashboard.maintainDialog.delete401', { count: settings.delete401 ? accountsStore.summary.invalid401Count : 0 }),
    t('dashboard.maintainDialog.quotaAction', { action: quotaActionLabel(settings.quotaAction) }),
    t('dashboard.maintainDialog.autoReenable', { count: settings.autoReenable ? accountsStore.summary.recoveredCount : 0 }),
  ]

  try {
    await ElMessageBox.confirm(previewLines.join('\n'), t('dashboard.maintainDialog.title'), {
      confirmButtonText: t('dashboard.maintainDialog.confirm'),
      cancelButtonText: t('dashboard.maintainDialog.cancel'),
      customClass: 'cpa-message-box',
      type: 'warning',
    })
    await tasksStore.runMaintain({
      delete401: settings.delete401,
      quotaAction: settings.quotaAction,
      autoReenable: settings.autoReenable,
    })
    ElMessage.success(t('dashboard.maintainCompleted'))
  } catch (error) {
    if (String(error) !== 'cancel') {
      emitDebugError('dashboard', 'runMaintain failed', error)
      ElMessage.error(toErrorMessage(error))
    }
  }
}

async function loadScanDetailPage(page = detailPage.value, pageSize = detailPageSize.value) {
  if (detailRunId.value === null) {
    return
  }
  detailLoading.value = true
  try {
    const detail = await accountsStore.loadScanDetail(detailRunId.value, page, pageSize)
    emitDebug('dashboard', 'scan detail page loaded', {
      runId: detailRunId.value,
      page: detail.page,
      pageSize: detail.pageSize,
      total: detail.totalRecords,
    })
    detailPage.value = detail.page
    detailPageSize.value = detail.pageSize
  } finally {
    detailLoading.value = false
  }
}

async function openHistory(runId: number) {
  detailRunId.value = runId
  detailPage.value = 1
  try {
    await loadScanDetailPage(1, detailPageSize.value)
    drawerOpen.value = true
  } catch (error) {
    emitDebugError('dashboard', 'openHistory failed', error)
    ElMessage.error(toErrorMessage(error))
  }
}

function changeDetailPage(page: number) {
  detailPage.value = page
  void loadScanDetailPage(page, detailPageSize.value)
}

function changeDetailPageSize(pageSize: number) {
  detailPageSize.value = pageSize
  detailPage.value = 1
  void loadScanDetailPage(1, pageSize)
}
</script>

<template>
  <div class="view-shell view-shell--dashboard">
    <section class="hero-panel">
      <div>
        <p class="eyebrow">{{ t('dashboard.eyebrow') }}</p>
        <h2>{{ t('dashboard.title') }}</h2>
        <p class="lead">
          {{ t('dashboard.lead') }}
        </p>
      </div>
      <div class="hero-actions">
        <el-button type="primary" size="large" :disabled="tasksStore.hasActiveTask" @click="runScan">
          {{ t('dashboard.scanNow') }}
        </el-button>
        <el-button size="large" :disabled="tasksStore.hasActiveTask" @click="runMaintain">
          {{ t('dashboard.runMaintain') }}
        </el-button>
        <el-button v-if="tasksStore.hasActiveTask" type="danger" plain size="large" @click="tasksStore.cancelCurrentTask()">
          {{ t('dashboard.cancelTask') }}
        </el-button>
      </div>
    </section>

    <section class="stats-grid">
      <article class="stat-card stat-card--accent">
        <span class="stat-label">{{ t('dashboard.trackedAccounts') }}</span>
        <strong>{{ accountsStore.summary.filteredAccounts }}</strong>
        <small>{{ t('dashboard.filteredBy', { type: settingsStore.settings.targetType || t('common.anyType'), provider: settingsStore.settings.provider || t('common.allProviders') }) }}</small>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ t('states.invalid_401') }}</span>
        <strong>{{ accountsStore.summary.invalid401Count }}</strong>
        <small>{{ t('dashboard.invalidHint') }}</small>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ t('states.quota_limited') }}</span>
        <strong>{{ accountsStore.summary.quotaLimitedCount }}</strong>
        <small>{{ t('dashboard.quotaHint', { action: quotaActionLabel(settingsStore.settings.quotaAction) }) }}</small>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ t('states.recovered') }}</span>
        <strong>{{ accountsStore.summary.recoveredCount }}</strong>
        <small>{{ t('dashboard.recoveredHint') }}</small>
      </article>
    </section>

    <section class="dashboard-grid">
      <article class="panel panel--fill panel--chart dashboard-panel dashboard-panel--health">
        <div class="panel-head">
          <div>
            <p class="panel-kicker">{{ t('dashboard.stateDistribution') }}</p>
            <h3>{{ t('dashboard.poolHealth') }}</h3>
          </div>
          <StatusPill :state="tasksStore.scan.active ? 'pending' : 'normal'" :label="tasksStore.scan.active ? t('tasks.running') : t('tasks.ready')" />
        </div>
        <div class="panel__body panel__body--chart">
          <SummaryDonut :summary="accountsStore.summary" />
        </div>
      </article>

      <article class="panel panel--fill panel--history dashboard-panel dashboard-panel--history">
        <div class="panel-head">
          <div>
            <p class="panel-kicker">{{ t('dashboard.latestHistory') }}</p>
            <h3>{{ t('dashboard.recentRuns') }}</h3>
          </div>
          <span class="muted">{{ accountsStore.summary.lastScanAt ? formatDateTime(accountsStore.summary.lastScanAt) : t('dashboard.noCompletedScan') }}</span>
        </div>
        <div class="panel__body panel__body--table">
          <div class="table-wrap">
            <el-table class="dashboard-history-table" :data="historyRows" height="100%">
              <el-table-column prop="runId" :label="t('dashboard.historyColumns.run')" width="76" />
              <el-table-column prop="statusLabel" :label="t('dashboard.historyColumns.status')" width="110" />
              <el-table-column prop="filteredAccounts" :label="t('dashboard.historyColumns.filtered')" width="96" />
              <el-table-column prop="invalid401Count" :label="t('dashboard.historyColumns.invalid')" width="74" />
              <el-table-column prop="quotaLimitedCount" :label="t('dashboard.historyColumns.quota')" width="84" />
              <el-table-column prop="recoveredCount" :label="t('dashboard.historyColumns.recovered')" width="104" />
              <el-table-column prop="finishedAtLabel" :label="t('dashboard.historyColumns.finished')" min-width="180" />
              <el-table-column label="" width="120">
                <template #default="{ row }">
                  <el-button text @click="openHistory(row.runId)">
                    {{ t('dashboard.inspect') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
      </article>
    </section>

    <el-drawer
      v-model="drawerOpen"
      class="scan-detail-drawer"
      modal-class="scan-detail-overlay"
      size="min(1120px, 78vw)"
      @closed="detailRunId = null"
    >
      <template #header>
        <div v-if="scanDetailSummary" class="scan-detail-header">
          <div class="scan-detail-header__copy">
            <p class="panel-kicker">{{ t('dashboard.latestHistory') }}</p>
            <h3>{{ t('dashboard.scanDetail') }}</h3>
            <p class="muted">
              {{ t('dashboard.runLabel', { id: scanDetailSummary.runId }) }}
              <span class="scan-detail-header__dot">|</span>
              {{ t('dashboard.historyColumns.finished') }}
              {{ formatDateTime(scanDetailSummary.finishedAt) }}
            </p>
          </div>
          <StatusPill :state="scanDetailStatusState" :label="taskStatusLabel(scanDetailSummary.status)" />
        </div>
      </template>

      <template v-if="scanDetailSummary">
        <div class="scan-detail-shell">
          <div class="scan-detail-metrics">
            <article class="scan-detail-metric">
              <span class="scan-detail-metric__label">{{ t('dashboard.historyColumns.run') }}</span>
              <strong>#{{ scanDetailSummary.runId }}</strong>
            </article>
            <article class="scan-detail-metric scan-detail-metric--status">
              <span class="scan-detail-metric__label">{{ t('dashboard.historyColumns.status') }}</span>
              <StatusPill :state="scanDetailStatusState" :label="taskStatusLabel(scanDetailSummary.status)" />
            </article>
            <article class="scan-detail-metric">
              <span class="scan-detail-metric__label">{{ t('dashboard.historyColumns.filtered') }}</span>
              <strong>{{ scanDetailSummary.filteredAccounts }}</strong>
            </article>
            <article class="scan-detail-metric">
              <span class="scan-detail-metric__label">{{ t('states.error') }}</span>
              <strong>{{ scanDetailSummary.errorCount }}</strong>
            </article>
          </div>

          <div class="scan-detail-table-shell" :class="{ 'scan-detail-table-shell--loading': detailLoading }">
            <div class="scan-detail-table-frame">
              <el-table class="scan-detail-table" :data="scanDetailRecords" height="100%">
                <el-table-column :label="t('dashboard.detailColumns.name')" min-width="320" show-overflow-tooltip>
                  <template #default="{ row }">
                    <div class="scan-detail-name">
                      <strong>{{ row.name }}</strong>
                      <span>{{ row.provider || t('common.unknown') }}</span>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column :label="t('dashboard.detailColumns.state')" width="128">
                  <template #default="{ row }">
                    <StatusPill :state="row.stateKey || row.state" />
                  </template>
                </el-table-column>
                <el-table-column prop="email" :label="t('dashboard.detailColumns.email')" min-width="220" show-overflow-tooltip />
                <el-table-column :label="t('dashboard.detailColumns.plan')" width="120">
                  <template #default="{ row }">
                    <span class="scan-detail-muted-value">{{ row.planType || t('common.notAvailable') }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('dashboard.detailColumns.probeError')" min-width="240" show-overflow-tooltip>
                  <template #default="{ row }">
                    <span class="scan-detail-muted-value">{{ row.probeErrorText || t('common.notAvailable') }}</span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <div class="scan-detail-table-footer">
              <el-pagination
                background
                :page-sizes="detailPageSizes"
                :current-page="detailPage"
                :page-size="detailPageSize"
                :total="scanDetailTotal"
                layout="total, sizes, prev, pager, next"
                @current-change="changeDetailPage"
                @size-change="changeDetailPageSize"
              />
            </div>
          </div>
        </div>
      </template>
    </el-drawer>
  </div>
</template>
