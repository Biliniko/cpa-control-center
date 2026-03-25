<script lang="ts" setup>
import { computed, nextTick, onErrorCaptured, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { ElConfigProvider, ElMessage, ElOption, ElSelect } from 'element-plus'
import type { Language } from 'element-plus/es/locale'
import en from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import {
  ClipboardSetText,
  ScreenGetAll,
  WindowSetLightTheme,
  WindowSetMinSize,
} from '../wailsjs/runtime/runtime'
import { useAccountsStore } from '@/stores/accounts'
import { useQuotasStore } from '@/stores/quotas'
import { useSettingsStore } from '@/stores/settings'
import { useTasksStore } from '@/stores/tasks'
import type { ViewKey } from '@/types'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { localeChinese } from '@/utils/locale'
import { toErrorMessage } from '@/utils/errors'
import { cronMatchesDate, isValidCronExpression } from '@/utils/settings'
import { debugEventName, emitDebug, emitDebugError, setDebugEnabled, snapshotDebugEntries, type DebugEntry } from '@/utils/debug'
import DashboardView from '@/views/DashboardView.vue'
import AccountsView from '@/views/AccountsView.vue'
import QuotasView from '@/views/QuotasView.vue'
import LogsView from '@/views/LogsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import { resolveShellMode, shellModeKey, type ShellMode } from '@/layout/shell'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const accountsStore = useAccountsStore()
const quotasStore = useQuotasStore()
const tasksStore = useTasksStore()

const activeView = ref<ViewKey>('dashboard')
const appReady = ref(false)
const shellRevision = ref(0)
const viewRevision = ref(0)
const debugVisible = ref(false)
const debugEntries = ref<DebugEntry[]>([])
const appViewport = ref<HTMLDivElement | null>(null)
let debugListenersBound = false
let viewportObserver: ResizeObserver | null = null
let quotaAutoRefreshTimer: number | null = null
let lastQuotaAutoRefreshKey = ''
let pendingQuotaAutoRefresh = false

const safeMinWidth = 1280
const safeMinHeight = 720
const startupFallbackMinWidth = 720
const startupFallbackMinHeight = 480
const viewportWidth = ref(window.innerWidth)
const viewportHeight = ref(window.innerHeight)
const isMacOS = /Mac|iPhone|iPad|iPod/.test(navigator.platform || navigator.userAgent)

const navItems = computed<Array<{ key: ViewKey; label: string; caption: string }>>(() => [
  { key: 'dashboard', label: t('nav.dashboard'), caption: t('nav.dashboardCaption') },
  { key: 'accounts', label: t('nav.accounts'), caption: t('nav.accountsCaption') },
  { key: 'quotas', label: t('nav.quotas'), caption: t('nav.quotasCaption') },
  { key: 'logs', label: t('nav.logs'), caption: t('nav.logsCaption') },
  { key: 'settings', label: t('nav.settings'), caption: t('nav.settingsCaption') },
])

const activeComponent = computed(() => {
  switch (activeView.value) {
    case 'accounts':
      return AccountsView
    case 'quotas':
      return QuotasView
    case 'logs':
      return LogsView
    case 'settings':
      return SettingsView
    default:
      return DashboardView
  }
})

const connectionLabel = computed(() => {
  if (!settingsStore.connection) {
    return t('topbar.configured')
  }
  return settingsStore.connection.ok ? t('topbar.connected') : t('topbar.attention')
})

const elementLocale = computed<Language>(() => (
  (settingsStore.currentLocale === localeChinese ? zhCn : en) as unknown as Language
))

const lastScanText = computed(() => (
  accountsStore.summary.lastScanAt
    ? formatDateTime(accountsStore.summary.lastScanAt)
    : t('topbar.noRecentScan')
))

const safeHistoryCount = computed(() => (
  Array.isArray(accountsStore.history) ? accountsStore.history.length : 0
))

const safeRecordCount = computed(() => (
  Array.isArray(accountsStore.records) ? accountsStore.records.length : 0
))

const safeDebugEntries = computed(() => (
  Array.isArray(debugEntries.value) ? debugEntries.value : []
))

const shellMode = computed<ShellMode>(() => resolveShellMode(viewportWidth.value, viewportHeight.value))

const shellClasses = computed(() => ({
  'app-shell--wide': shellMode.value === 'wide',
  'app-shell--desktop': shellMode.value === 'desktop',
  'app-shell--compact': shellMode.value === 'compact',
  'app-shell--macos': isMacOS,
}))

provide(shellModeKey, shellMode)

function resetQuotaViewScroll() {
  const quotaView = appViewport.value?.querySelector<HTMLElement>('.view-shell--quotas')
  if (!quotaView) {
    return
  }
  quotaView.scrollTo({
    top: 0,
    left: 0,
    behavior: 'auto',
  })
}

async function refreshShell() {
  await nextTick()
  await new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
  shellRevision.value += 1
  viewRevision.value += 1
}

function appendDebug(entry: DebugEntry) {
  debugEntries.value = [entry, ...debugEntries.value].slice(0, 120)
}

function updateViewportMetrics() {
  if (!appViewport.value) {
    return
  }
  viewportWidth.value = Math.max(appViewport.value.clientWidth, 1)
  viewportHeight.value = Math.max(appViewport.value.clientHeight, 1)
}

function bindViewportObserver() {
  if (!appViewport.value || viewportObserver) {
    return
  }
  viewportObserver = new ResizeObserver(() => {
    updateViewportMetrics()
  })
  viewportObserver.observe(appViewport.value)
}

function unbindViewportObserver() {
  viewportObserver?.disconnect()
  viewportObserver = null
}

function stopQuotaAutoRefresh() {
  if (quotaAutoRefreshTimer !== null) {
    window.clearInterval(quotaAutoRefreshTimer)
    quotaAutoRefreshTimer = null
  }
  pendingQuotaAutoRefresh = false
}

function minuteKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth() + 1}-${date.getDate()}-${date.getHours()}-${date.getMinutes()}`
}

function tryQuotaAutoRefresh(now = new Date()) {
  const settings = settingsStore.settings
  const cron = settings.quotaAutoRefreshCron.trim()
  if (!settings.quotaAutoRefreshEnabled || !cron || !isValidCronExpression(cron)) {
    pendingQuotaAutoRefresh = false
    return
  }
  if (!settings.baseUrl || !settings.managementToken) {
    pendingQuotaAutoRefresh = false
    return
  }
  const currentMinuteKey = minuteKey(now)
  const matchesCurrentMinute = cronMatchesDate(cron, now)
  if (tasksStore.hasActiveTask) {
    if (matchesCurrentMinute && lastQuotaAutoRefreshKey !== currentMinuteKey) {
      lastQuotaAutoRefreshKey = currentMinuteKey
      pendingQuotaAutoRefresh = true
      emitDebug('quota', 'auto refresh queued', { cron, minute: currentMinuteKey })
    }
    return
  }
  if (quotasStore.loading || tasksStore.hasActiveTask) {
    return
  }
  if (pendingQuotaAutoRefresh) {
    pendingQuotaAutoRefresh = false
    emitDebug('quota', 'auto refresh draining queued run', { cron, minute: currentMinuteKey })
    void quotasStore.refreshSnapshot().catch((error) => {
      emitDebugError('quota', 'queued auto refresh failed', error)
    })
    return
  }
  if (lastQuotaAutoRefreshKey === currentMinuteKey) {
    return
  }
  if (!matchesCurrentMinute) {
    return
  }
  lastQuotaAutoRefreshKey = currentMinuteKey
  emitDebug('quota', 'auto refresh triggered', { cron, minute: currentMinuteKey })
  void quotasStore.refreshSnapshot().catch((error) => {
    emitDebugError('quota', 'auto refresh failed', error)
  })
}

function syncQuotaAutoRefresh() {
  stopQuotaAutoRefresh()
  lastQuotaAutoRefreshKey = ''
  const settings = settingsStore.settings
  const cron = settings.quotaAutoRefreshCron.trim()
  if (!settings.quotaAutoRefreshEnabled || !cron || !isValidCronExpression(cron)) {
    return
  }
  quotaAutoRefreshTimer = window.setInterval(() => {
    tryQuotaAutoRefresh()
  }, 15000)
  tryQuotaAutoRefresh()
}

function mergeBufferedDebug(entries: DebugEntry[]) {
  const existingChronological = [...debugEntries.value].reverse()
  const mergedChronological = [...entries, ...existingChronological]
  const seen = new Set<string>()
  const deduped: DebugEntry[] = []

  for (const entry of mergedChronological) {
    const key = `${entry.timestamp}|${entry.source}|${entry.message}|${entry.detail || ''}`
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    deduped.push(entry)
  }

  debugEntries.value = deduped.slice(-120).reverse()
}

function onDebugEvent(event: Event) {
  const customEvent = event as CustomEvent<DebugEntry>
  if (customEvent.detail) {
    appendDebug(customEvent.detail)
  }
}

function onWindowError(event: ErrorEvent) {
  appendDebug({
    timestamp: new Date().toISOString(),
    level: 'error',
    source: 'window.error',
    message: event.message || 'Unhandled window error',
    detail: event.error?.stack || event.filename,
  })
}

function onUnhandledRejection(event: PromiseRejectionEvent) {
  appendDebug({
    timestamp: new Date().toISOString(),
    level: 'error',
    source: 'window.rejection',
    message: 'Unhandled promise rejection',
    detail: String(event.reason),
  })
}

function onDebugHotkey(event: KeyboardEvent) {
  if (event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 'd') {
    event.preventDefault()
    debugVisible.value = !debugVisible.value
  }
}

function bindDebugListeners() {
  if (debugListenersBound) {
    return
  }
  window.addEventListener(debugEventName(), onDebugEvent as EventListener)
  debugListenersBound = true
}

function unbindDebugListeners() {
  if (!debugListenersBound) {
    return
  }
  window.removeEventListener(debugEventName(), onDebugEvent as EventListener)
  debugListenersBound = false
}

async function copyDebugDump() {
  const dump = [
    `appReady=${appReady.value}`,
    `activeView=${activeView.value}`,
    `locale=${settingsStore.currentLocale}`,
    `summary.filtered=${accountsStore.summary.filteredAccounts}`,
    `summary.pending=${accountsStore.summary.pendingCount}`,
    `history.count=${safeHistoryCount.value}`,
    `records.page=${safeRecordCount.value}/${accountsStore.totalRecords}`,
    '',
    ...safeDebugEntries.value.map((entry) => {
      const detail = entry.detail ? `\n${entry.detail}` : ''
      return `[${entry.timestamp}] [${entry.level}] [${entry.source}] ${entry.message}${detail}`
    }),
  ].join('\n')
  await ClipboardSetText(dump)
  ElMessage.success('Debug info copied')
}

async function changeLocale(locale: string) {
  try {
    await settingsStore.saveLocalePreference(locale)
    await refreshShell()
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function calibrateWindowToScreen() {
  try {
    const screens = await ScreenGetAll()
    const screen = screens.find((item) => item.isCurrent) ?? screens.find((item) => item.isPrimary) ?? screens[0]
    if (!screen) {
      return
    }

    const availableWidth = Math.max(
      screen.width - 32,
      Math.min(startupFallbackMinWidth, screen.width),
    )
    const availableHeight = Math.max(
      screen.height - 96,
      Math.min(startupFallbackMinHeight, screen.height),
    )
    const minWidth = Math.min(safeMinWidth, availableWidth)
    const minHeight = Math.min(safeMinHeight, availableHeight)

    WindowSetMinSize(minWidth, minHeight)
  } catch (error) {
    emitDebugError('app', 'window calibration failed', error)
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onDebugHotkey)
  window.addEventListener('error', onWindowError)
  window.addEventListener('unhandledrejection', onUnhandledRejection)
  updateViewportMetrics()
  bindViewportObserver()
  emitDebug('app', 'startup begin')
  try {
    WindowSetLightTheme()
    await calibrateWindowToScreen()
    updateViewportMetrics()
    await settingsStore.loadSettings()
    settingsStore.initSchedulerBridge()
    emitDebug('app', 'settings loaded', {
      locale: settingsStore.currentLocale,
      baseUrl: settingsStore.settings.baseUrl,
    })
    tasksStore.initEventBridge()
    await accountsStore.refreshAll()
    emitDebug('app', 'dashboard snapshot loaded', {
      filtered: accountsStore.summary.filteredAccounts,
      pending: accountsStore.summary.pendingCount,
      history: accountsStore.history.length,
    })
    if (
      !accountsStore.hasInventory &&
      settingsStore.settings.baseUrl &&
      settingsStore.settings.managementToken
    ) {
      emitDebug('app', 'inventory sync queued during startup')
      tasksStore.scheduleInventorySync()
    }
    syncQuotaAutoRefresh()
    await refreshShell()
    appReady.value = true
    emitDebug('app', 'startup complete', {
      activeView: activeView.value,
      shellRevision: shellRevision.value,
      viewRevision: viewRevision.value,
    })
  } catch (error) {
    emitDebugError('app', 'startup failed', error)
    ElMessage.error(toErrorMessage(error))
    appReady.value = true
  }
})

onUnmounted(() => {
  tasksStore.destroyEventBridge()
  settingsStore.destroySchedulerBridge()
  stopQuotaAutoRefresh()
  setDebugEnabled(false)
  unbindViewportObserver()
  unbindDebugListeners()
  window.removeEventListener('keydown', onDebugHotkey)
  window.removeEventListener('error', onWindowError)
  window.removeEventListener('unhandledrejection', onUnhandledRejection)
})

onErrorCaptured((error, instance, info) => {
  const componentName = (instance?.$options?.name as string | undefined) || 'anonymous'
  appendDebug({
    timestamp: new Date().toISOString(),
    level: 'error',
    source: `vue:${componentName}`,
    message: info,
    detail: error instanceof Error ? error.stack || error.message : String(error),
  })
  return false
})

watch(activeView, async (value) => {
  if (value !== 'quotas' && quotasStore.error) {
    quotasStore.error = ''
  }
  if (value === 'quotas') {
    await nextTick()
    window.requestAnimationFrame(() => {
      resetQuotaViewScroll()
    })
  }
  emitDebug('app', 'active view changed', { value })
})

watch(
  () => [
    settingsStore.settings.quotaAutoRefreshEnabled,
    settingsStore.settings.quotaAutoRefreshCron,
    settingsStore.settings.baseUrl,
    settingsStore.settings.managementToken,
  ],
  () => {
    syncQuotaAutoRefresh()
  },
)

watch(debugVisible, (visible) => {
  setDebugEnabled(visible)
  if (visible) {
    bindDebugListeners()
    mergeBufferedDebug(snapshotDebugEntries())
    appendDebug({
      timestamp: new Date().toISOString(),
      level: 'info',
      source: 'app',
      message: 'Debug mode enabled',
      detail: JSON.stringify({
        activeView: activeView.value,
        locale: settingsStore.currentLocale,
        filtered: accountsStore.summary.filteredAccounts,
        pending: accountsStore.summary.pendingCount,
      }, null, 2),
    })
    emitDebug('app', 'debug mode enabled', {
      activeView: activeView.value,
      locale: settingsStore.currentLocale,
      filtered: accountsStore.summary.filteredAccounts,
      pending: accountsStore.summary.pendingCount,
    })
    return
  }
  unbindDebugListeners()
}, { flush: 'post' })
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <div
      ref="appViewport"
      class="app-viewport"
    >
      <div :key="shellRevision" class="app-shell" :class="shellClasses">
        <div v-if="isMacOS" class="window-titlebar" aria-hidden="true" />
        <aside class="app-sidebar">
          <div>
            <p class="sidebar-kicker">{{ t('app.name') }}</p>
            <h1>{{ t('app.headline') }}</h1>
            <p class="sidebar-copy">
              {{ t('app.copy') }}
            </p>
          </div>

          <nav class="nav-list">
            <button
              v-for="item in navItems"
              :key="item.key"
              class="nav-item"
              :class="{ 'nav-item--active': item.key === activeView }"
              @click="activeView = item.key"
            >
              <strong>{{ item.label }}</strong>
              <span>{{ item.caption }}</span>
            </button>
          </nav>
        </aside>

        <main class="app-main">
          <header class="topbar">
            <div class="topbar-status">
              <span class="status-dot" :data-tone="settingsStore.connectionTone" />
              <div>
                <strong>{{ connectionLabel }}</strong>
                <p>{{ settingsStore.settings.baseUrl || t('topbar.endpointHint') }}</p>
              </div>
            </div>
            <div class="topbar-meta">
              <span>{{ t('topbar.tracked', { count: accountsStore.summary.filteredAccounts }) }}</span>
              <span>{{ lastScanText }}</span>
              <el-select
                class="locale-switcher"
                :model-value="settingsStore.currentLocale"
                size="small"
                @change="changeLocale"
              >
                <el-option :label="t('topbar.english')" value="en-US" />
                <el-option :label="t('topbar.chinese')" value="zh-CN" />
              </el-select>
            </div>
          </header>

          <section v-if="!appReady" class="view-shell view-shell--settings">
            <article class="panel panel--fill">
              <div class="panel-head">
                <div>
                  <p class="panel-kicker">{{ t('app.name') }}</p>
                  <h3>{{ t('common.loading') }}</h3>
                </div>
              </div>
              <div class="panel__body muted">
                {{ t('settings.notTestedYet') }}
              </div>
            </article>
          </section>
          <component v-else :is="activeComponent" :key="`${activeView}-${viewRevision}`" />
        </main>
      </div>

      <aside v-if="debugVisible" class="debug-panel">
        <div class="debug-panel__header">
          <div>
            <strong>Debug Panel</strong>
            <p>Ctrl+Shift+D</p>
          </div>
          <div class="debug-panel__actions">
            <el-button text @click="copyDebugDump">Copy</el-button>
            <el-button text @click="debugVisible = false">Close</el-button>
          </div>
        </div>

        <div class="debug-panel__summary">
          <div><strong>ready</strong><span>{{ appReady }}</span></div>
          <div><strong>view</strong><span>{{ activeView }}</span></div>
          <div><strong>locale</strong><span>{{ settingsStore.currentLocale }}</span></div>
          <div><strong>tracked</strong><span>{{ accountsStore.summary.filteredAccounts }}</span></div>
          <div><strong>pending</strong><span>{{ accountsStore.summary.pendingCount }}</span></div>
          <div><strong>history</strong><span>{{ safeHistoryCount }}</span></div>
          <div><strong>page</strong><span>{{ safeRecordCount }}/{{ accountsStore.totalRecords }}</span></div>
          <div><strong>mode</strong><span>{{ shellMode }}</span></div>
          <div><strong>rev</strong><span>{{ shellRevision }}/{{ viewRevision }}</span></div>
        </div>

        <div class="debug-panel__logs">
          <article v-for="entry in safeDebugEntries" :key="`${entry.timestamp}-${entry.source}-${entry.message}`" class="debug-panel__entry" :data-level="entry.level">
            <div class="debug-panel__entry-head">
              <strong>{{ entry.source }}</strong>
              <span>{{ entry.timestamp }}</span>
            </div>
            <p>{{ entry.message }}</p>
            <pre v-if="entry.detail">{{ entry.detail }}</pre>
          </article>
        </div>
      </aside>
    </div>
  </el-config-provider>
</template>
