<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  ElButton,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElMessage,
  ElOption,
  ElPopover,
  ElSelect,
  ElSwitch,
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useTasksStore } from '@/stores/tasks'
import type { AuthImportResult } from '@/types'
import { formatDateTime } from '@/utils/format'
import { toErrorMessage } from '@/utils/errors'
import { listScheduleConflicts, type ScheduleConflictJob } from '@/utils/settings'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const tasksStore = useTasksStore()
const timezoneLabel = Intl.DateTimeFormat().resolvedOptions().timeZone || t('settings.scheduleTimezoneLocal')
const isChinese = computed(() => settingsStore.currentLocale === 'zh-CN')
const skipKnown401Label = computed(() => (isChinese.value ? '扫描时跳过已知 401' : 'Skip known 401 during scan'))

const infoAriaLabel = computed(() => (isChinese.value ? '更多信息' : 'More information'))

const importingFiles = ref(false)
const importingDirectory = ref(false)

const infoCopy = computed<Record<string, string>>(() => (
  isChinese.value
    ? {
        targetType: '这里填写的是账号类型过滤条件，不是模型名。常见值如 codex、chatgpt、gemini。若填成 gpt-5.2 这类模型名，账号列表和仪表盘可能会被全部过滤为空。',
        scanStrategy: '全量扫描会在一次任务中探测所有过滤后的账号。增量扫描每次只探测一个批次，更适合大号池分摊压力。',
        scanBatchSize: '仅在增量扫描模式下生效，决定每次扫描实际探测多少个过滤后的账号。',
        probeWorkers: '扫描时允许同时进行的探测请求数。数值越大完成越快，但对 CPA 和上游的压力也越大。',
        actionWorkers: '维护时允许同时进行的禁用、删除或恢复请求数。',
        quotaWorkers: '额度页允许同时进行的 Codex 额度查询数。数值越大刷新越快，但会增加上游压力。',
        timeoutSeconds: '单次请求的超时时间。数值越大越能容忍慢响应，但上游异常时 worker 会被占用更久。',
        retries: '可重试探测错误的额外尝试次数。数值越大越稳，但也会放大总请求量。',
        quotaAction: '扫描后遇到限额账号时，维护流程要执行的动作。禁用更稳，删除更激进。',
        quotaFreeMaxAccounts: '限制 free 套餐最多查询多少个账号的额度。填 -1 表示不限数量。',
        quotaPlanToggles: '只有开启的套餐会在 Codex 额度页中被查询和展示。默认关闭 free，避免浪费请求。',
        quotaAutoRefreshCron: '启用后，应用会按这个 5 段 Cron 表达式在后台自动刷新 Codex 额度，并把进度写入任务日志。',
        scheduleMode: '扫描只执行定时健康检查。维护会先扫描，再按当前设置执行删除、禁用或恢复动作。',
        scheduleCron: '使用本地时区的标准 5 段 cron：分钟 小时 日 月 星期。示例：0 */6 * * *。',
      }
    : {
        targetType: 'This field filters account type, not model name. Typical values are codex, chatgpt, or gemini. Entering a model like gpt-5.2 can filter the dashboard and accounts list down to zero.',
        scanStrategy: 'Full scan probes every filtered account in one run. Incremental scan probes only one batch each run to spread load across large pools.',
        scanBatchSize: 'Only used in incremental mode. This sets how many filtered accounts are probed in each scan run.',
        probeWorkers: 'Maximum concurrent probe requests during scanning. Higher values finish faster, but they increase CPA and upstream pressure.',
        actionWorkers: 'Maximum concurrent disable, delete, or re-enable requests during maintenance.',
        quotaWorkers: 'Maximum concurrent Codex quota requests on the quota page. Higher values refresh faster, but they also increase upstream pressure.',
        timeoutSeconds: 'Per-request timeout in seconds. Higher values tolerate slower responses, but workers stay occupied longer when upstream is unhealthy.',
        retries: 'Extra attempts for retryable probe failures. Higher values improve resilience, but they multiply total request volume.',
        quotaAction: 'What maintenance should do with quota-limited accounts after scanning. Disable is safer; delete is more aggressive.',
        quotaFreeMaxAccounts: 'Limits how many free accounts can be queried on the quota page. Use -1 for unlimited.',
        quotaPlanToggles: 'Only enabled plans are queried and shown on the Codex quota page. Free is off by default to avoid wasting calls.',
        quotaAutoRefreshCron: 'When enabled, the app refreshes Codex quotas in the background using this 5-field cron expression and writes progress to task logs.',
        scheduleMode: 'Scan runs a scheduled health check only. Maintain first scans, then executes maintenance actions using the current settings.',
        scheduleCron: 'Uses standard 5-field cron in your local timezone: minute hour day month weekday. Example: 0 */6 * * *.',
      }
))

function infoText(key: string) {
  if (key in infoCopy.value) {
    return infoCopy.value[key]
  }
  return ''
}

const connectionCopy = computed(() => {
  if (!settingsStore.connection) {
    return t('settings.notTestedYet')
  }
  if (settingsStore.connection.accountCount <= 0) {
    return t('settings.connectionSummaryBasic', {
      message: settingsStore.connection.message,
      checkedAt: formatDateTime(settingsStore.connection.checkedAt),
    })
  }
  return t('settings.connectionSummary', {
    message: settingsStore.connection.message,
    count: settingsStore.connection.accountCount,
    checkedAt: formatDateTime(settingsStore.connection.checkedAt),
  })
})

const schedulerStatusText = computed(() => {
  const status = settingsStore.schedulerStatus.lastStatus
  if (!status) {
    return t('common.notAvailable')
  }
  return t(`settings.scheduleStatus.${status}`)
})

const schedulerMessage = computed(() => (
  settingsStore.schedulerStatus.validationMessage ||
  settingsStore.schedulerStatus.lastMessage ||
  t('common.notAvailable')
))

const authImportSchedulerStatusText = computed(() => {
  const status = settingsStore.authImportSchedulerStatus.lastStatus
  if (!status) {
    return t('common.notAvailable')
  }
  return t(`settings.scheduleStatus.${status}`)
})

const authImportSchedulerMessage = computed(() => (
  settingsStore.authImportSchedulerStatus.validationMessage ||
  settingsStore.authImportSchedulerStatus.lastMessage ||
  t('common.notAvailable')
))

function scheduleJobLabel(job: ScheduleConflictJob) {
  switch (job) {
    case 'authImport':
      return t('settings.scheduleJobImport')
    case 'quotaAutoRefresh':
      return t('settings.scheduleJobQuota')
    default:
      return settingsStore.settings.schedule.mode === 'maintain'
        ? t('settings.scheduleJobMaintain')
        : t('settings.scheduleJobScan')
  }
}

const scheduleConflictMessages = computed(() => (
  listScheduleConflicts(settingsStore.settings).map((conflict) => (
    t('settings.scheduleConflictWarning', {
      first: scheduleJobLabel(conflict.jobs[0]),
      second: scheduleJobLabel(conflict.jobs[1]),
      time: formatDateTime(conflict.at),
    })
  ))
))

function scheduleConflictSummary(messages: string[]) {
  return messages.join(isChinese.value ? '；' : '; ')
}

async function testOnly() {
  try {
    const result = await settingsStore.testConnection()
    if (!result) {
      ElMessage.success(t('settings.testReachableBasic', { message: t('common.notAvailable') }))
      return
    }
    ElMessage.success(
      result.accountCount > 0
        ? t('settings.testReachable', { message: result.message, count: result.accountCount })
        : t('settings.testReachableBasic', { message: result.message }),
    )
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function testAndSave() {
  try {
    const scheduleWarnings = [...scheduleConflictMessages.value]
    const result = await settingsStore.testAndSave()
    tasksStore.scheduleInventorySync()
    if (!result) {
      ElMessage.success(t('settings.savedReachableSyncingBasic'))
      if (scheduleWarnings.length > 0) {
        ElMessage.warning(scheduleConflictSummary(scheduleWarnings))
      }
      return
    }
    ElMessage.success(
      result.accountCount > 0
        ? t('settings.savedReachableSyncing', { count: result.accountCount })
        : t('settings.savedReachableSyncingBasic'),
    )
    if (scheduleWarnings.length > 0) {
      ElMessage.warning(scheduleConflictSummary(scheduleWarnings))
    }
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function changeLocale(locale: string) {
  try {
    await settingsStore.saveLocalePreference(locale)
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

function firstImportError(result: AuthImportResult) {
  return result.results.find((item) => !item.ok && item.error.trim().length > 0)?.error || ''
}

function notifyImportResult(result: AuthImportResult) {
  if (!result || result.requested <= 0) {
    return
  }

  if (result.uploaded > 0 && result.archiveFailed === 0 && result.archived > 0 && !result.syncError && result.failed === 0 && result.skipped === 0) {
    ElMessage.success(t('settings.importArchiveSuccess', { uploaded: result.uploaded, archived: result.archived }))
    return
  }

  if (result.uploaded > 0 && result.archiveFailed > 0) {
    ElMessage.warning(
      t('settings.importArchiveWarning', {
        uploaded: result.uploaded,
        requested: result.requested,
        archiveFailed: result.archiveFailed,
      }),
    )
    return
  }

  if (result.uploaded > 0 && !result.syncError && result.failed === 0 && result.skipped === 0) {
    ElMessage.success(t('settings.importSuccess', { uploaded: result.uploaded }))
    return
  }

  if (result.uploaded > 0 && result.syncError) {
    ElMessage.warning(
      t('settings.importPartialWithSyncError', {
        uploaded: result.uploaded,
        requested: result.requested,
        failed: result.failed,
        skipped: result.skipped,
        error: result.syncError,
      }),
    )
    return
  }

  if (result.uploaded > 0) {
    ElMessage.warning(
      t('settings.importPartial', {
        uploaded: result.uploaded,
        requested: result.requested,
        failed: result.failed,
        skipped: result.skipped,
      }),
    )
    return
  }

  ElMessage.error(
    firstImportError(result) || t('settings.importFailure', {
      failed: result.failed,
      skipped: result.skipped,
    }),
  )
}

async function importFiles() {
  importingFiles.value = true
  try {
    const result = await settingsStore.importAuthFiles()
    notifyImportResult(result)
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  } finally {
    importingFiles.value = false
  }
}

async function importDirectory() {
  importingDirectory.value = true
  try {
    const result = await settingsStore.importAuthDirectory()
    notifyImportResult(result)
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  } finally {
    importingDirectory.value = false
  }
}
</script>

<template>
  <div class="view-shell view-shell--settings">
    <section class="panel settings-panel panel--scroll">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">{{ t('settings.connectionProfile') }}</p>
          <h3>{{ t('settings.savedTarget') }}</h3>
        </div>
        <span class="muted">{{ connectionCopy }}</span>
      </div>

      <el-form label-position="top" class="settings-form">
        <div class="settings-grid">
          <el-form-item :label="t('settings.language')">
            <el-select :model-value="settingsStore.currentLocale" @change="changeLocale">
              <el-option :label="t('topbar.english')" value="en-US" />
              <el-option :label="t('topbar.chinese')" value="zh-CN" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('settings.baseUrl')" :error="settingsStore.errors.baseUrl">
            <el-input v-model="settingsStore.settings.baseUrl" :placeholder="t('settings.baseUrlPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.managementToken')" :error="settingsStore.errors.managementToken">
            <el-input v-model="settingsStore.settings.managementToken" type="password" show-password :placeholder="t('settings.tokenPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.targetType')">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.targetType') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.targetType') }}</strong>
                    <p>{{ infoText('targetType') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input v-model="settingsStore.settings.targetType" />
          </el-form-item>
          <el-form-item :label="t('settings.provider')">
            <el-input v-model="settingsStore.settings.provider" :placeholder="t('settings.providerPlaceholder')" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.scanStrategy">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.scanStrategy') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.scanStrategy') }}</strong>
                    <p>{{ infoText('scanStrategy') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-select v-model="settingsStore.settings.scanStrategy">
              <el-option :label="t('settings.scanStrategyFull')" value="full" />
              <el-option :label="t('settings.scanStrategyIncremental')" value="incremental" />
            </el-select>
          </el-form-item>
          <el-form-item :error="settingsStore.errors.scanBatchSize">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.scanBatchSize') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.scanBatchSize') }}</strong>
                    <p>{{ infoText('scanBatchSize') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input-number v-model="settingsStore.settings.scanBatchSize" :min="1" :max="50000" :disabled="settingsStore.settings.scanStrategy !== 'incremental'" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.probeWorkers">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.probeWorkers') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.probeWorkers') }}</strong>
                    <p>{{ infoText('probeWorkers') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input-number v-model="settingsStore.settings.probeWorkers" :min="1" :max="200" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.actionWorkers">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.actionWorkers') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.actionWorkers') }}</strong>
                    <p>{{ infoText('actionWorkers') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input-number v-model="settingsStore.settings.actionWorkers" :min="1" :max="100" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.timeoutSeconds">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.timeoutSeconds') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.timeoutSeconds') }}</strong>
                    <p>{{ infoText('timeoutSeconds') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input-number v-model="settingsStore.settings.timeoutSeconds" :min="1" :max="120" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.retries">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.retries') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.retries') }}</strong>
                    <p>{{ infoText('retries') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-input-number v-model="settingsStore.settings.retries" :min="0" :max="10" />
          </el-form-item>
          <el-form-item :error="settingsStore.errors.quotaAction">
            <template #label>
              <span class="form-label-with-info">
                <span>{{ t('settings.quotaAction') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.quotaAction') }}</strong>
                    <p>{{ infoText('quotaAction') }}</p>
                  </div>
                </el-popover>
              </span>
            </template>
            <el-select v-model="settingsStore.settings.quotaAction">
              <el-option :label="t('quotaActions.disable')" value="disable" />
              <el-option :label="t('quotaActions.delete')" value="delete" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('settings.exportDirectory')">
            <el-input v-model="settingsStore.settings.exportDirectory" />
          </el-form-item>
          <el-form-item :label="t('settings.userAgent')" class="span-2">
            <el-input v-model="settingsStore.settings.userAgent" />
          </el-form-item>
        </div>

        <p class="muted">{{ t('settings.languageHint') }}</p>
        <p class="muted">{{ t('settings.scanBatchHint') }}</p>

        <div class="settings-toggles">
          <el-switch v-model="settingsStore.settings.skipKnown401" :active-text="t('settings.skipKnown401')" />
          <el-switch v-model="settingsStore.settings.delete401" :active-text="t('settings.delete401')" />
          <el-switch v-model="settingsStore.settings.autoReenable" :active-text="t('settings.autoReenable')" />
          <el-switch v-model="settingsStore.settings.detailedLogs" :active-text="t('settings.detailedLogs')" />
        </div>

        <section class="settings-schedule settings-quota">
          <div class="panel-head panel-head--tight">
            <div>
              <p class="panel-kicker">{{ t('settings.quotaSection') }}</p>
              <h3>{{ t('settings.quotaTitle') }}</h3>
            </div>
          </div>

          <div class="settings-grid settings-grid--schedule">
            <el-form-item :error="settingsStore.errors.quotaWorkers">
              <template #label>
                <span class="form-label-with-info">
                  <span>{{ t('settings.quotaWorkers') }}</span>
                  <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                    <template #reference>
                      <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                    </template>
                    <div class="settings-info-popover__content">
                      <strong>{{ t('settings.quotaWorkers') }}</strong>
                      <p>{{ infoText('quotaWorkers') }}</p>
                    </div>
                  </el-popover>
                </span>
              </template>
              <el-input-number v-model="settingsStore.settings.quotaWorkers" :min="1" :max="100" />
            </el-form-item>

            <el-form-item :error="settingsStore.errors.quotaFreeMaxAccounts">
              <template #label>
                <span class="form-label-with-info">
                  <span>{{ t('settings.quotaFreeMaxAccounts') }}</span>
                  <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                    <template #reference>
                      <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                    </template>
                    <div class="settings-info-popover__content">
                      <strong>{{ t('settings.quotaFreeMaxAccounts') }}</strong>
                      <p>{{ infoText('quotaFreeMaxAccounts') }}</p>
                    </div>
                  </el-popover>
                </span>
              </template>
              <el-input-number v-model="settingsStore.settings.quotaFreeMaxAccounts" :min="-1" :max="50000" />
            </el-form-item>
          </div>

          <p class="muted">{{ t('settings.quotaUnlimitedHint') }}</p>

          <div class="settings-toggles settings-toggles--quota">
            <div class="settings-toggle-group">
              <span class="form-label-with-info">
                <span>{{ t('settings.quotaPlanToggles') }}</span>
                <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                  <template #reference>
                    <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                  </template>
                  <div class="settings-info-popover__content">
                    <strong>{{ t('settings.quotaPlanToggles') }}</strong>
                    <p>{{ infoText('quotaPlanToggles') }}</p>
                  </div>
                </el-popover>
              </span>
              <div class="settings-plan-switches">
                <el-switch v-model="settingsStore.settings.quotaCheckFree" :active-text="t('settings.quotaPlanFree')" />
                <el-switch v-model="settingsStore.settings.quotaCheckPlus" :active-text="t('settings.quotaPlanPlus')" />
                <el-switch v-model="settingsStore.settings.quotaCheckPro" :active-text="t('settings.quotaPlanPro')" />
                <el-switch v-model="settingsStore.settings.quotaCheckTeam" :active-text="t('settings.quotaPlanTeam')" />
                <el-switch v-model="settingsStore.settings.quotaCheckBusiness" :active-text="t('settings.quotaPlanBusiness')" />
                <el-switch v-model="settingsStore.settings.quotaCheckEnterprise" :active-text="t('settings.quotaPlanEnterprise')" />
              </div>
            </div>
          </div>

          <div class="settings-grid settings-grid--schedule">
            <el-form-item :label="t('settings.quotaAutoRefreshEnabled')">
              <el-switch v-model="settingsStore.settings.quotaAutoRefreshEnabled" />
            </el-form-item>
            <el-form-item :error="settingsStore.errors.quotaAutoRefreshCron">
              <template #label>
                <span class="form-label-with-info">
                  <span>{{ t('settings.quotaAutoRefreshCron') }}</span>
                  <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                    <template #reference>
                      <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                    </template>
                    <div class="settings-info-popover__content">
                      <strong>{{ t('settings.quotaAutoRefreshCron') }}</strong>
                      <p>{{ infoText('quotaAutoRefreshCron') }}</p>
                    </div>
                  </el-popover>
                </span>
              </template>
              <el-input
                v-model="settingsStore.settings.quotaAutoRefreshCron"
                :placeholder="t('settings.scheduleCronPlaceholder')"
                :disabled="!settingsStore.settings.quotaAutoRefreshEnabled"
              />
            </el-form-item>
          </div>
        </section>

        <section class="settings-schedule">
          <div class="panel-head panel-head--tight">
            <div>
              <p class="panel-kicker">{{ t('settings.scheduleSection') }}</p>
              <h3>{{ t('settings.scheduleTitle') }}</h3>
            </div>
            <span class="muted">{{ t('settings.scheduleTimezoneHint', { timezone: timezoneLabel }) }}</span>
          </div>

          <div class="settings-grid settings-grid--schedule">
            <el-form-item :label="t('settings.scheduleEnabled')">
              <el-switch v-model="settingsStore.settings.schedule.enabled" />
            </el-form-item>
            <el-form-item :error="settingsStore.errors.scheduleMode">
              <template #label>
                <span class="form-label-with-info">
                  <span>{{ t('settings.scheduleMode') }}</span>
                  <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                    <template #reference>
                      <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                    </template>
                    <div class="settings-info-popover__content">
                      <strong>{{ t('settings.scheduleMode') }}</strong>
                      <p>{{ infoText('scheduleMode') }}</p>
                    </div>
                  </el-popover>
                </span>
              </template>
              <el-select v-model="settingsStore.settings.schedule.mode" :disabled="!settingsStore.settings.schedule.enabled">
                <el-option :label="t('settings.scheduleModeScan')" value="scan" />
                <el-option :label="t('settings.scheduleModeMaintain')" value="maintain" />
              </el-select>
            </el-form-item>
            <el-form-item class="span-2" :error="settingsStore.errors.scheduleCron">
              <template #label>
                <span class="form-label-with-info">
                  <span>{{ t('settings.scheduleCron') }}</span>
                  <el-popover trigger="click" placement="top-start" :width="320" popper-class="settings-info-popover">
                    <template #reference>
                      <button type="button" class="info-trigger" :aria-label="infoAriaLabel">i</button>
                    </template>
                    <div class="settings-info-popover__content">
                      <strong>{{ t('settings.scheduleCron') }}</strong>
                      <p>{{ infoText('scheduleCron') }}</p>
                    </div>
                  </el-popover>
                </span>
              </template>
              <el-input
                v-model="settingsStore.settings.schedule.cron"
                :disabled="!settingsStore.settings.schedule.enabled"
                :placeholder="t('settings.scheduleCronPlaceholder')"
              />
            </el-form-item>
          </div>

          <p class="muted">
            {{ t('settings.scheduleExamples') }}
          </p>

          <div class="settings-schedule-status">
            <div>
              <strong>{{ t('settings.scheduleNextRun') }}</strong>
              <span>{{ formatDateTime(settingsStore.schedulerStatus.nextRunAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.scheduleLastStarted') }}</strong>
              <span>{{ formatDateTime(settingsStore.schedulerStatus.lastStartedAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.scheduleLastFinished') }}</strong>
              <span>{{ formatDateTime(settingsStore.schedulerStatus.lastFinishedAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.scheduleLastResult') }}</strong>
              <span>{{ schedulerStatusText }}</span>
            </div>
            <div class="span-2">
              <strong>{{ t('settings.scheduleStatusMessage') }}</strong>
              <span>{{ schedulerMessage }}</span>
            </div>
          </div>
        </section>

        <section class="settings-schedule">
          <div class="panel-head panel-head--tight">
            <div>
              <p class="panel-kicker">{{ t('settings.importSection') }}</p>
              <h3>{{ t('settings.importTitle') }}</h3>
            </div>
          </div>

          <p class="muted">{{ t('settings.importHint') }}</p>
          <p class="muted">{{ t('settings.importDirectoryRememberHint') }}</p>
          <p class="muted">{{ t('settings.importArchiveHint') }}</p>

          <div class="settings-grid settings-grid--schedule">
            <el-form-item :label="t('settings.importSourceDirectory')" :error="settingsStore.errors.authImportSourceDirectory">
              <el-input v-model="settingsStore.settings.authImport.sourceDirectory" />
            </el-form-item>
            <el-form-item :label="t('settings.importArchiveDirectory')">
              <el-input v-model="settingsStore.settings.authImport.archiveDirectory" />
            </el-form-item>
            <el-form-item :label="t('settings.importAutoEnabled')">
              <el-switch v-model="settingsStore.settings.authImport.autoEnabled" />
            </el-form-item>
            <el-form-item :label="t('settings.importMoveImported')">
              <el-switch v-model="settingsStore.settings.authImport.moveImported" />
            </el-form-item>
            <el-form-item class="span-2" :label="t('settings.importAutoCron')" :error="settingsStore.errors.authImportAutoCron">
              <el-input
                v-model="settingsStore.settings.authImport.autoCron"
                :disabled="!settingsStore.settings.authImport.autoEnabled"
                :placeholder="t('settings.scheduleCronPlaceholder')"
              />
            </el-form-item>
          </div>

          <div class="hero-actions">
            <el-button :loading="importingFiles" :disabled="settingsStore.saving || importingDirectory" @click="importFiles">
              {{ t('settings.importFiles') }}
            </el-button>
            <el-button plain :loading="importingDirectory" :disabled="settingsStore.saving || importingFiles" @click="importDirectory">
              {{ t('settings.importDirectory') }}
            </el-button>
          </div>

          <div class="settings-schedule-status">
            <div>
              <strong>{{ t('settings.importScheduleNextRun') }}</strong>
              <span>{{ formatDateTime(settingsStore.authImportSchedulerStatus.nextRunAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.importScheduleLastStarted') }}</strong>
              <span>{{ formatDateTime(settingsStore.authImportSchedulerStatus.lastStartedAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.importScheduleLastFinished') }}</strong>
              <span>{{ formatDateTime(settingsStore.authImportSchedulerStatus.lastFinishedAt) }}</span>
            </div>
            <div>
              <strong>{{ t('settings.importScheduleLastResult') }}</strong>
              <span>{{ authImportSchedulerStatusText }}</span>
            </div>
            <div class="span-2">
              <strong>{{ t('settings.importScheduleStatusMessage') }}</strong>
              <span>{{ authImportSchedulerMessage }}</span>
            </div>
          </div>
        </section>

        <div class="settings-schedule-hints">
          <p class="muted">{{ t('settings.scheduleRecommendedOffsets') }}</p>
          <div v-if="scheduleConflictMessages.length > 0" class="settings-schedule-conflicts">
            <strong>{{ t('settings.scheduleConflictTitle') }}</strong>
            <p v-for="message in scheduleConflictMessages" :key="message">
              {{ message }}
            </p>
          </div>
        </div>

        <div class="hero-actions">
          <el-button plain @click="testOnly">{{ t('settings.testConnection') }}</el-button>
          <el-button type="primary" :loading="settingsStore.saving" @click="testAndSave">
            {{ t('settings.testAndSave') }}
          </el-button>
        </div>
      </el-form>
    </section>
  </div>
</template>

<style scoped>
.settings-schedule-hints {
  display: grid;
  gap: 0.75rem;
}

.settings-schedule-conflicts {
  display: grid;
  gap: 0.5rem;
  padding: 0.875rem 1rem;
  border: 1px solid rgba(184, 119, 0, 0.28);
  border-radius: 14px;
  background: rgba(255, 243, 205, 0.72);
  color: #6a4b00;
}

.settings-schedule-conflicts p {
  margin: 0;
}
</style>
