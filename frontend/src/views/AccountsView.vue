<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  ElButton,
  ElDialog,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import StatusPill from '@/components/StatusPill.vue'
import { useAccountsStore } from '@/stores/accounts'
import { useTasksStore } from '@/stores/tasks'
import type { AccountRecord, BulkAccountActionResult } from '@/types'
import { formatDateTime } from '@/utils/format'
import { stateDescription, stateOrder } from '@/utils/status'
import { toErrorMessage } from '@/utils/errors'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const tasksStore = useTasksStore()

const pageSizeOptions = [20, 50, 100, 200]
const accountsTable = ref<InstanceType<typeof ElTable> | null>(null)
const selectedRecords = ref<AccountRecord[]>([])
const detailDialogOpen = ref(false)
const detailDialogRecordName = ref('')
const detailDialogContent = ref('')

const providerOptions = computed(() => accountsStore.providerOptions)
const planOptions = computed(() => accountsStore.planOptions)
const stateOptions = computed(() => stateOrder.map((value) => ({ value, label: t(`states.${value}`) })))
const disabledOptions = computed(() => [
  { value: 'false', label: t('common.no') },
  { value: 'true', label: t('common.yes') },
])

function normalizedText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

const showCodexProviderFilters = computed(() => normalizedText(accountsStore.providerFilter).toLowerCase() === 'codex')
const disabledFilterValue = computed({
  get: () => {
    if (accountsStore.disabledFilter === null) {
      return ''
    }
    return accountsStore.disabledFilter ? 'true' : 'false'
  },
  set: (value: string) => {
    if (value === 'true') {
      accountsStore.disabledFilter = true
      return
    }
    if (value === 'false') {
      accountsStore.disabledFilter = false
      return
    }
    accountsStore.disabledFilter = null
  },
})
const selectedNames = computed(() => selectedRecords.value.map((item) => item.name))
const selectedCount = computed(() => selectedRecords.value.length)
const selectedDisabledCount = computed(() => selectedRecords.value.filter((item) => item.disabled).length)
const selectedEnabledCount = computed(() => selectedRecords.value.filter((item) => !item.disabled).length)
const selectionDisabled = computed(() => selectedCount.value === 0 || tasksStore.hasActiveTask)
const bulkProbeDisabled = computed(() => selectionDisabled.value)
const bulkEnableDisabled = computed(() => selectionDisabled.value || selectedDisabledCount.value === 0)
const bulkDisableDisabled = computed(() => selectionDisabled.value || selectedEnabledCount.value === 0)
const bulkDeleteDisabled = computed(() => selectionDisabled.value)

async function clearSelection() {
  selectedRecords.value = []
  await nextTick()
  accountsTable.value?.clearSelection()
}

async function reloadAccounts(options?: { page?: number; pageSize?: number; resetPage?: boolean }) {
  await clearSelection()
  await accountsStore.loadAccountsPage(options)
}

watch(
  () => [
    accountsStore.query,
    accountsStore.stateFilter,
    accountsStore.providerFilter,
    accountsStore.planFilter,
    accountsStore.disabledFilter,
  ],
  () => {
    void reloadAccounts({ resetPage: true })
  },
)

watch(
  () => accountsStore.providerFilter,
  (value) => {
    if (normalizedText(value).toLowerCase() === 'codex') {
      return
    }
    if (accountsStore.planFilter !== '') {
      accountsStore.planFilter = ''
    }
    if (accountsStore.disabledFilter !== null) {
      accountsStore.disabledFilter = null
    }
  },
)

function onSelectionChange(records: AccountRecord[]) {
  selectedRecords.value = records
}

function detailText(row: AccountRecord) {
  return stateDescription(row)
}

function planPillClass(planType: string) {
  switch (normalizedText(planType).toLowerCase()) {
    case 'free':
      return 'account-pill account-pill--plan-free'
    case 'team':
      return 'account-pill account-pill--plan-team'
    case 'pro':
      return 'account-pill account-pill--plan-pro'
    case 'plus':
      return 'account-pill account-pill--plan-plus'
    case 'enterprise':
    case 'business':
      return 'account-pill account-pill--plan-enterprise'
    default:
      return 'account-pill account-pill--plan-generic'
  }
}

function planPillLabel(planType: string) {
  return normalizedText(planType) || t('common.notAvailable')
}

function openDetailDialog(row: AccountRecord) {
  detailDialogRecordName.value = row.name
  detailDialogContent.value = detailText(row)
  detailDialogOpen.value = true
}

async function exportKind(kind: 'invalid401' | 'quotaLimited', format: 'json' | 'csv') {
  try {
    const result = await accountsStore.exportRecords(kind, format)
    ElMessage.success(t('accounts.messages.exported', { count: result.exported, path: result.path }))
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

function summarizeBulkResult(actionLabel: string, result: BulkAccountActionResult) {
  return t('accounts.messages.bulkSummary', {
    action: actionLabel,
    succeeded: result.succeeded,
    failed: result.failed,
    skipped: result.skipped,
  })
}

async function bulkProbe() {
  if (bulkProbeDisabled.value) {
    return
  }
  try {
    await ElMessageBox.confirm(
      t('accounts.dialogs.bulkProbeMessage', { count: selectedCount.value }),
      t('accounts.dialogs.bulkProbeTitle'),
      {
        confirmButtonText: t('accounts.actions.bulkProbe'),
        cancelButtonText: t('accounts.dialogs.cancel'),
        customClass: 'cpa-message-box',
        type: 'info',
      },
    )
    const result = await accountsStore.probeAccounts(selectedNames.value)
    await accountsStore.refreshAll()
    await clearSelection()
    ElMessage({
      type: result.failed > 0 ? 'warning' : 'success',
      message: summarizeBulkResult(t('accounts.actions.bulkProbe'), result),
    })
  } catch (error) {
    if (String(error) !== 'cancel') {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

async function bulkToggle(disabled: boolean) {
  const eligibleCount = disabled ? selectedEnabledCount.value : selectedDisabledCount.value
  if ((disabled ? bulkDisableDisabled : bulkEnableDisabled).value) {
    return
  }
  try {
    await ElMessageBox.confirm(
      t('accounts.dialogs.bulkToggleMessage', {
        count: selectedCount.value,
        state: disabled ? t('accounts.actions.disable') : t('accounts.actions.enable'),
      }),
      t('accounts.dialogs.bulkToggleTitle'),
      {
        confirmButtonText: disabled ? t('accounts.actions.bulkDisable') : t('accounts.actions.bulkEnable'),
        cancelButtonText: t('accounts.dialogs.cancel'),
        customClass: 'cpa-message-box',
        type: disabled ? 'warning' : 'info',
      },
    )
    const result = await accountsStore.setAccountsDisabled(selectedNames.value, disabled)
    await accountsStore.refreshAll()
    await clearSelection()
    ElMessage({
      type: result.failed > 0 ? 'warning' : 'success',
      message: summarizeBulkResult(
        disabled ? t('accounts.actions.bulkDisable') : t('accounts.actions.bulkEnable'),
        {
          ...result,
          processed: eligibleCount,
        },
      ),
    })
  } catch (error) {
    if (String(error) !== 'cancel') {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

async function bulkDelete() {
  if (bulkDeleteDisabled.value) {
    return
  }
  try {
    await ElMessageBox.confirm(
      t('accounts.dialogs.bulkDeleteMessage', { count: selectedCount.value }),
      t('accounts.dialogs.bulkDeleteTitle'),
      {
        confirmButtonText: t('accounts.actions.bulkDelete'),
        cancelButtonText: t('accounts.dialogs.cancel'),
        customClass: 'cpa-message-box',
        type: 'warning',
      },
    )
    const result = await accountsStore.deleteAccounts(selectedNames.value)
    await accountsStore.refreshAll()
    await clearSelection()
    ElMessage({
      type: result.failed > 0 ? 'warning' : 'success',
      message: summarizeBulkResult(t('accounts.actions.bulkDelete'), result),
    })
  } catch (error) {
    if (String(error) !== 'cancel') {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

function changePage(page: number) {
  void reloadAccounts({ page })
}

function changePageSize(pageSize: number) {
  void reloadAccounts({ pageSize, resetPage: true })
}
</script>

<template>
  <div class="view-shell view-shell--accounts">
    <section class="panel panel--fill">
      <div class="toolbar">
        <div class="toolbar-group">
          <el-input v-model="accountsStore.query" :placeholder="t('accounts.searchPlaceholder')" clearable />
          <el-select v-model="accountsStore.stateFilter" :placeholder="t('accounts.statePlaceholder')" clearable style="width: 180px">
            <el-option v-for="option in stateOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
          <el-select v-model="accountsStore.providerFilter" :placeholder="t('accounts.providerPlaceholder')" clearable style="width: 180px">
            <el-option v-for="provider in providerOptions" :key="provider" :label="provider" :value="provider" />
          </el-select>
          <el-select
            v-if="showCodexProviderFilters"
            v-model="accountsStore.planFilter"
            :placeholder="t('accounts.planPlaceholder')"
            clearable
            style="width: 160px"
          >
            <el-option v-for="plan in planOptions" :key="plan" :label="planPillLabel(plan)" :value="plan" />
          </el-select>
          <el-select
            v-if="showCodexProviderFilters"
            v-model="disabledFilterValue"
            :placeholder="t('accounts.disabledPlaceholder')"
            clearable
            style="width: 160px"
          >
            <el-option v-for="option in disabledOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </div>
        <div class="toolbar-group toolbar-group--compact">
          <el-button plain @click="exportKind('invalid401', 'json')">{{ t('accounts.exportInvalidJson') }}</el-button>
          <el-button plain @click="exportKind('invalid401', 'csv')">{{ t('accounts.exportInvalidCsv') }}</el-button>
          <el-button plain @click="exportKind('quotaLimited', 'json')">{{ t('accounts.exportQuotaJson') }}</el-button>
          <el-button plain @click="exportKind('quotaLimited', 'csv')">{{ t('accounts.exportQuotaCsv') }}</el-button>
        </div>
      </div>

      <div class="accounts-bulkbar">
        <div class="accounts-bulkbar__meta">
          <span class="accounts-bulkbar__label">{{ t('accounts.selectedCount', { count: selectedCount }) }}</span>
        </div>
        <div class="accounts-bulkbar__actions">
          <el-button :disabled="bulkProbeDisabled" @click="bulkProbe">{{ t('accounts.actions.bulkProbe') }}</el-button>
          <el-button :disabled="bulkEnableDisabled" @click="bulkToggle(false)">{{ t('accounts.actions.bulkEnable') }}</el-button>
          <el-button :disabled="bulkDisableDisabled" @click="bulkToggle(true)">{{ t('accounts.actions.bulkDisable') }}</el-button>
          <el-button type="danger" plain :disabled="bulkDeleteDisabled" @click="bulkDelete">{{ t('accounts.actions.bulkDelete') }}</el-button>
        </div>
      </div>

      <div class="panel__body panel__body--table">
        <div class="table-wrap">
          <el-table
            ref="accountsTable"
            class="accounts-table"
            :data="accountsStore.records"
            height="100%"
            row-key="name"
            @selection-change="onSelectionChange"
          >
            <el-table-column type="selection" width="52" />
            <el-table-column prop="name" :label="t('accounts.columns.name')" min-width="220" />
            <el-table-column :label="t('accounts.columns.state')" width="116">
              <template #default="{ row }">
                <StatusPill :state="row.stateKey || row.state" />
              </template>
            </el-table-column>
            <el-table-column prop="email" :label="t('accounts.columns.email')" min-width="198" />
            <el-table-column prop="provider" :label="t('accounts.columns.provider')" width="104" />
            <el-table-column :label="t('accounts.columns.plan')" width="108">
              <template #default="{ row }">
                <span :class="planPillClass(row.planType)">{{ planPillLabel(row.planType) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.disabled')" width="78">
              <template #default="{ row }">
                <span :class="['account-pill', row.disabled ? 'account-pill--danger' : 'account-pill--success']">
                  {{ row.disabled ? t('common.yes') : t('common.no') }}
                </span>
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.lastProbed')" min-width="164">
              <template #default="{ row }">
                {{ formatDateTime(row.lastProbedAt) }}
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.details')" min-width="220">
              <template #default="{ row }">
                <button
                  type="button"
                  class="account-detail-trigger"
                  :aria-label="t('accounts.actions.viewDetails')"
                  @click="openDetailDialog(row)"
                >
                  <span class="account-detail-trigger__text">{{ detailText(row) }}</span>
                </button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="table-footer">
          <span class="muted table-footer__summary">
            {{ t('accounts.paginationSummary', { shown: accountsStore.records.length, total: accountsStore.totalRecords, all: accountsStore.summary.filteredAccounts }) }}
          </span>
          <el-pagination
            :current-page="accountsStore.page"
            :page-size="accountsStore.pageSize"
            background
            :page-sizes="pageSizeOptions"
            :total="accountsStore.totalRecords"
            layout="total, sizes, prev, pager, next, jumper"
            @current-change="changePage"
            @size-change="changePageSize"
          />
        </div>
      </div>
    </section>

    <el-dialog
      v-model="detailDialogOpen"
      class="account-detail-dialog"
      :title="t('accounts.dialogs.detailTitle')"
      width="min(760px, calc(100vw - 32px))"
      append-to-body
      destroy-on-close
    >
      <div class="account-detail-dialog__header">
        <span class="account-detail-dialog__eyebrow">{{ t('accounts.columns.name') }}</span>
        <strong class="account-detail-dialog__name">{{ detailDialogRecordName }}</strong>
      </div>
      <div class="account-detail-dialog__body">
        <pre class="account-detail-dialog__content">{{ detailDialogContent }}</pre>
      </div>
    </el-dialog>
  </div>
</template>
