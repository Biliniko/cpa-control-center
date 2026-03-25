import { defineStore } from 'pinia'
import { GetCodexQuotaSnapshot, GetManagementUsageSummary } from '../../wailsjs/go/main/App'
import type { CodexQuotaSnapshot, ManagementUsageSummary, QuotaRecoveryMode, QuotaResultFilter, QuotaSortMode, QuotaViewMode } from '@/types'
import { i18n } from '@/i18n'
import { toErrorMessage } from '@/utils/errors'
import { normalizeManagementUsageSummary } from '@/utils/usage'

interface QuotasState {
  snapshot: CodexQuotaSnapshot | null
  usageSummary: ManagementUsageSummary | null
  loading: boolean
  usageLoading: boolean
  error: string
  usageError: string
  hasRequested: boolean
  activeView: QuotaViewMode
  planFilter: string
  resultFilter: QuotaResultFilter
  sortMode: QuotaSortMode
  matrixPage: number
  matrixRows: number
  recoveryPage: number
  recoveryRows: number
  recoveryMode: QuotaRecoveryMode
  selectedAccountName: string
}

export const useQuotasStore = defineStore('quotasStore', {
  state: (): QuotasState => ({
    snapshot: null,
    usageSummary: null,
    loading: false,
    usageLoading: false,
    error: '',
    usageError: '',
    hasRequested: false,
    activeView: 'overview',
    planFilter: 'all',
    resultFilter: 'all',
    sortMode: 'plan',
    matrixPage: 1,
    matrixRows: 3,
    recoveryPage: 1,
    recoveryRows: 3,
    recoveryMode: 'earliest',
    selectedAccountName: '',
  }),
  getters: {
    plans: (state) => state.snapshot?.plans ?? [],
    accountDetails: (state) => state.snapshot?.accounts ?? [],
    hasData: (state) => (state.snapshot?.plans?.length ?? 0) > 0,
    hasDetailData: (state) => (state.snapshot?.accounts?.length ?? 0) > 0,
    lastFetchedAt: (state) => state.snapshot?.fetchedAt ?? '',
    usageFetchedAt: (state) => state.usageSummary?.fetchedAt ?? '',
    selectedAccount: (state) => (
      state.snapshot?.accounts?.find((account) => account.name === state.selectedAccountName) ?? null
    ),
  },
  actions: {
    applySnapshot(snapshot: CodexQuotaSnapshot) {
      this.snapshot = snapshot
      this.error = ''
      this.hasRequested = true
      if (!snapshot.accounts.some((account) => account.name === this.selectedAccountName)) {
        this.selectedAccountName = ''
      }
    },
    async refreshSnapshot() {
      this.loading = true
      this.error = ''
      this.hasRequested = true
      try {
        const snapshot = await GetCodexQuotaSnapshot() as unknown as CodexQuotaSnapshot
        this.applySnapshot(snapshot)
        return snapshot
      } catch (error) {
        const message = toErrorMessage(error)
        this.error = message
        if (!this.snapshot) {
          this.snapshot = null
        }
        throw new Error(message)
      } finally {
        this.loading = false
      }
    },
    applyUsageSummary(summary?: Record<string, unknown> | null) {
      const normalized = normalizeManagementUsageSummary(summary)
      if (!normalized) {
        throw new Error(i18n.global.t('usage.invalidResponse'))
      }
      this.usageSummary = normalized
      this.usageError = ''
      return normalized
    },
    async refreshUsageSummary(silent = false) {
      this.usageLoading = true
      if (!silent) {
        this.usageError = ''
      }
      try {
        const summary = await GetManagementUsageSummary() as unknown as Record<string, unknown>
        return this.applyUsageSummary(summary)
      } catch (error) {
        const message = toErrorMessage(error)
        this.usageError = message
        throw new Error(message)
      } finally {
        this.usageLoading = false
      }
    },
    setActiveView(view: QuotaViewMode) {
      this.activeView = view
    },
    setPlanFilter(value: string) {
      this.planFilter = value
      this.matrixPage = 1
      this.recoveryPage = 1
      this.selectedAccountName = ''
    },
    setResultFilter(value: QuotaResultFilter) {
      this.resultFilter = value
      this.matrixPage = 1
      this.recoveryPage = 1
      this.selectedAccountName = ''
    },
    setSortMode(value: QuotaSortMode) {
      this.sortMode = value
      this.matrixPage = 1
      this.recoveryPage = 1
    },
    setMatrixPage(value: number) {
      this.matrixPage = Math.max(1, value)
    },
    setMatrixRows(value: number) {
      this.matrixRows = Math.max(1, value)
      this.matrixPage = 1
    },
    setRecoveryPage(value: number) {
      this.recoveryPage = Math.max(1, value)
    },
    setRecoveryRows(value: number) {
      this.recoveryRows = Math.max(1, value)
      this.recoveryPage = 1
    },
    setRecoveryMode(value: QuotaRecoveryMode) {
      this.recoveryMode = value
      this.recoveryPage = 1
    },
    setSelectedAccount(name: string) {
      this.selectedAccountName = name
    },
  },
})
