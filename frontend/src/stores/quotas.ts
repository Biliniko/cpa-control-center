import { defineStore } from 'pinia'
import { GetCodexQuotaSnapshot } from '../../wailsjs/go/main/App'
import type { CodexQuotaSnapshot, QuotaRecoveryMode, QuotaResultFilter, QuotaSortMode, QuotaViewMode } from '@/types'
import { toErrorMessage } from '@/utils/errors'

interface QuotasState {
  snapshot: CodexQuotaSnapshot | null
  loading: boolean
  error: string
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
    loading: false,
    error: '',
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
