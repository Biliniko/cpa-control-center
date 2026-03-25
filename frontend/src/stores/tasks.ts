import { defineStore } from 'pinia'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'
import { CancelScan, RunMaintain, RunScan } from '../../wailsjs/go/main/App'
import { i18n } from '@/i18n'
import type { CodexQuotaSnapshot, LogEntry, MaintainOptions, TaskFinished, TaskKind, TaskProgress } from '@/types'
import { toErrorMessage } from '@/utils/errors'
import { useAccountsStore } from '@/stores/accounts'
import { useQuotasStore } from '@/stores/quotas'
import { taskPhaseLabel } from '@/utils/status'

interface TaskTracker {
  active: boolean
  phase: string
  current: number
  total: number
  message: string
}

interface TasksState {
  scan: TaskTracker
  maintain: TaskTracker
  inventory: TaskTracker
  quota: TaskTracker
  import: TaskTracker
  inventoryQueued: boolean
  logs: LogEntry[]
  initialised: boolean
}

function emptyTracker(): TaskTracker {
  return {
    active: false,
    phase: 'idle',
    current: 0,
    total: 0,
    message: '',
  }
}

function progressEntryId(kind: TaskKind): string {
  return `${kind}:progress`
}

function progressMessage(payload: TaskProgress): string {
  const phase = taskPhaseLabel(payload.phase)
  if (payload.total > 0) {
    return payload.message ? `${phase} ${payload.current}/${payload.total} · ${payload.message}` : `${phase} ${payload.current}/${payload.total}`
  }
  return payload.message || phase
}

export const useTasksStore = defineStore('tasksStore', {
  state: (): TasksState => ({
    scan: emptyTracker(),
    maintain: emptyTracker(),
    inventory: emptyTracker(),
    quota: emptyTracker(),
    import: emptyTracker(),
    inventoryQueued: false,
    logs: [],
    initialised: false,
  }),
  getters: {
    hasActiveTask: (state) => state.scan.active || state.maintain.active || state.inventory.active || state.quota.active || state.import.active,
  },
  actions: {
    initEventBridge() {
      if (this.initialised) {
        return
      }

      EventsOn('scan:log', (entry: LogEntry) => this.pushLog(entry))
      EventsOn('maintain:log', (entry: LogEntry) => this.pushLog(entry))
      EventsOn('inventory:log', (entry: LogEntry) => this.pushLog(entry))
      EventsOn('quota:log', (entry: LogEntry) => this.pushLog(entry))
      EventsOn('import:log', (entry: LogEntry) => this.pushLog(entry))
      EventsOn('scan:progress', (payload: TaskProgress) => {
        const message = progressMessage(payload)
        this.scan = {
          active: !payload.done,
          phase: payload.phase,
          current: payload.current,
          total: payload.total,
          message,
        }
        this.upsertProgressLog('scan', payload, message)
      })
      EventsOn('maintain:progress', (payload: TaskProgress) => {
        const message = progressMessage(payload)
        this.maintain = {
          active: !payload.done,
          phase: payload.phase,
          current: payload.current,
          total: payload.total,
          message,
        }
        this.upsertProgressLog('maintain', payload, message)
      })
      EventsOn('inventory:progress', (payload: TaskProgress) => {
        const message = progressMessage(payload)
        this.inventory = {
          active: !payload.done,
          phase: payload.phase,
          current: payload.current,
          total: payload.total,
          message,
        }
        this.upsertProgressLog('inventory', payload, message)
      })
      EventsOn('quota:progress', (payload: TaskProgress) => {
        const message = progressMessage(payload)
        this.quota = {
          active: !payload.done,
          phase: payload.phase,
          current: payload.current,
          total: payload.total,
          message,
        }
        this.upsertProgressLog('quota', payload, message)
      })
      EventsOn('import:progress', (payload: TaskProgress) => {
        const message = progressMessage(payload)
        this.import = {
          active: !payload.done,
          phase: payload.phase,
          current: payload.current,
          total: payload.total,
          message,
        }
        this.upsertProgressLog('import', payload, message)
      })
      EventsOn('quota:snapshot', (snapshot: CodexQuotaSnapshot) => {
        useQuotasStore().applySnapshot(snapshot)
      })
      EventsOn('task:finished', (payload: TaskFinished) => {
        if (payload.kind === 'scan') {
          this.scan.active = false
        } else if (payload.kind === 'maintain') {
          this.maintain.active = false
        } else if (payload.kind === 'inventory') {
          this.inventory.active = false
        } else if (payload.kind === 'quota') {
          this.quota.active = false
        } else if (payload.kind === 'import') {
          this.import.active = false
        }
        if (payload.kind !== 'quota') {
          void useAccountsStore().refreshAll()
        }
        if (this.inventoryQueued && !this.scan.active && !this.maintain.active && !this.inventory.active && !this.quota.active && !this.import.active) {
          this.inventoryQueued = false
          void this.runInventory().catch(() => {})
        }
      })

      this.initialised = true
    },
    destroyEventBridge() {
      if (!this.initialised) {
        return
      }
      EventsOff('scan:log')
      EventsOff('maintain:log')
      EventsOff('inventory:log')
      EventsOff('quota:log')
      EventsOff('import:log')
      EventsOff('scan:progress')
      EventsOff('maintain:progress')
      EventsOff('inventory:progress')
      EventsOff('quota:progress')
      EventsOff('import:progress')
      EventsOff('quota:snapshot')
      EventsOff('task:finished')
      this.initialised = false
    },
    pushLog(entry: LogEntry) {
      if (entry.id) {
        const existing = this.logs.findIndex((item) => item.id === entry.id)
        if (existing >= 0) {
          this.logs.splice(existing, 1)
        }
      }
      this.logs.unshift(entry)
      this.logs = this.logs.slice(0, 500)
    },
    upsertProgressLog(kind: TaskKind, payload: TaskProgress, message: string) {
      this.pushLog({
        id: progressEntryId(kind),
        kind,
        level: 'info',
        message,
        timestamp: new Date().toISOString(),
        progress: true,
      })
    },
    async runScan() {
      const message = i18n.global.t('tasks.queuedScan')
      this.scan = { ...emptyTracker(), active: true, phase: 'queued', message }
      this.upsertProgressLog('scan', { kind: 'scan', phase: 'queued', current: 0, total: 0, message, done: false }, message)
      try {
        return await RunScan()
      } catch (error) {
        this.pushLog({
          kind: 'scan',
          level: 'error',
          message: toErrorMessage(error),
          timestamp: new Date().toISOString(),
        })
        throw error
      } finally {
        this.scan.active = false
      }
    },
    async runMaintain(options: MaintainOptions) {
      const message = i18n.global.t('tasks.queuedMaintain')
      this.maintain = { ...emptyTracker(), active: true, phase: 'queued', message }
      this.upsertProgressLog('maintain', { kind: 'maintain', phase: 'queued', current: 0, total: 0, message, done: false }, message)
      try {
        return await RunMaintain(options)
      } catch (error) {
        this.pushLog({
          kind: 'maintain',
          level: 'error',
          message: toErrorMessage(error),
          timestamp: new Date().toISOString(),
        })
        throw error
      } finally {
        this.maintain.active = false
      }
    },
    scheduleInventorySync() {
      if (this.scan.active || this.maintain.active || this.inventory.active || this.quota.active || this.import.active) {
        this.inventoryQueued = true
        return 'queued' as const
      }
      this.inventoryQueued = false
      void this.runInventory().catch(() => {})
      return 'started' as const
    },
    async runInventory() {
      const accountsStore = useAccountsStore()
      const message = i18n.global.t('tasks.queuedInventory')
      this.inventory = { ...emptyTracker(), active: true, phase: 'queued', message }
      this.upsertProgressLog('inventory', { kind: 'inventory', phase: 'queued', current: 0, total: 0, message, done: false }, message)
      try {
        return await accountsStore.syncInventory()
      } catch (error) {
        this.pushLog({
          kind: 'inventory',
          level: 'error',
          message: toErrorMessage(error),
          timestamp: new Date().toISOString(),
        })
        throw error
      } finally {
        this.inventory.active = false
      }
    },
    async cancelCurrentTask() {
      return await CancelScan()
    },
  },
})
