export type LocaleCode = 'en-US' | 'zh-CN'
export type ScheduleMode = 'scan' | 'maintain'
export type ScanStrategy = 'full' | 'incremental'
export type TaskKind = 'scan' | 'maintain' | 'inventory' | 'quota' | 'import'

export type AccountStateKey =
  | 'pending'
  | 'normal'
  | 'invalid_401'
  | 'quota_limited'
  | 'recovered'
  | 'error'
  | 'untracked'

export interface AppSettings {
  baseUrl: string
  managementToken: string
  locale: string
  detailedLogs: boolean
  targetType: string
  provider: string
  scanStrategy: ScanStrategy
  scanBatchSize: number
  skipKnown401: boolean
  probeWorkers: number
  actionWorkers: number
  quotaWorkers: number
  timeoutSeconds: number
  retries: number
  userAgent: string
  quotaAction: string
  quotaCheckFree: boolean
  quotaCheckPlus: boolean
  quotaCheckPro: boolean
  quotaCheckTeam: boolean
  quotaCheckBusiness: boolean
  quotaCheckEnterprise: boolean
  quotaFreeMaxAccounts: number
  quotaAutoRefreshEnabled: boolean
  quotaAutoRefreshCron: string
  delete401: boolean
  autoReenable: boolean
  exportDirectory: string
  authImport: AuthImportSettings
  schedule: ScheduleSettings
}

export interface AuthImportSettings {
  sourceDirectory: string
  archiveDirectory: string
  moveImported: boolean
  autoEnabled: boolean
  autoCron: string
}

export interface ScheduleSettings {
  enabled: boolean
  mode: ScheduleMode
  cron: string
}

export interface SchedulerStatus {
  enabled: boolean
  mode: string
  cron: string
  valid: boolean
  validationMessage: string
  running: boolean
  nextRunAt: string
  lastStartedAt: string
  lastFinishedAt: string
  lastStatus: string
  lastMessage: string
}

export interface ConnectionResult {
  ok: boolean
  message: string
  accountCount: number
  checkedAt: string
}

export interface AccountFilter {
  query: string
  state: string
  provider: string
  type: string
  planType: string
  disabled?: boolean
}

export interface AccountRecord {
  name: string
  authIndex: string
  email: string
  provider: string
  type: string
  planType: string
  account: string
  source: string
  status: string
  statusMessage: string
  state: string
  stateKey: string
  disabled: boolean
  unavailable: boolean
  runtimeOnly: boolean
  allowed?: boolean | null
  limitReached?: boolean | null
  invalid401: boolean
  quotaLimited: boolean
  recovered: boolean
  error: boolean
  apiHttpStatus?: number | null
  apiStatusCode?: number | null
  probeErrorKind: string
  probeErrorText: string
  managedReason: string
  lastAction: string
  lastActionStatus: string
  lastActionError: string
  lastSeenAt: string
  lastProbedAt: string
  updatedAt: string
  chatgptAccountId: string
  idTokenPlanType: string
  authUpdatedAt: string
  authModTime: string
  authLastRefresh: string
}

export interface DashboardSummary {
  totalAccounts: number
  filteredAccounts: number
  pendingCount: number
  normalCount: number
  invalid401Count: number
  quotaLimitedCount: number
  recoveredCount: number
  errorCount: number
  lastScanAt: string
}

export interface DashboardSnapshot {
  summary: DashboardSummary
  history: ScanSummary[]
}

export interface QuotaBucketSummary {
  supported: boolean
  totalRemainingPercent?: number | null
  resetAt: string
  successCount: number
  failedCount: number
}

export interface QuotaBucketDetail {
  supported: boolean
  remainingPercent?: number | null
  resetAt: string
}

export interface TokenUsageDetail {
  inputTokens?: number | null
  outputTokens?: number | null
  totalTokens?: number | null
}

export interface UsageTokenTotals {
  inputTokens: number
  outputTokens: number
  reasoningTokens: number
  cachedTokens: number
  totalTokens: number
}

export interface ManagementUsageSummary {
  fetchedAt: string
  lastRequestAt: string
  totalRequests: number
  successCount: number
  failureCount: number
  failedRequests: number
  todayRequests: number
  todayTokens: number
  apiKeyCount: number
  modelCount: number
  tokens: UsageTokenTotals
}

export interface CodexQuotaAccountDetail {
  name: string
  email: string
  planType: string
  provider: string
  success: boolean
  error: string
  fetchedAt: string
  earliestResetAt: string
  tokenUsage?: TokenUsageDetail
  fiveHour: QuotaBucketDetail
  weekly: QuotaBucketDetail
  codeReviewWeekly: QuotaBucketDetail
}

export interface CodexPlanQuotaSummary {
  planType: string
  accountCount: number
  fiveHour: QuotaBucketSummary
  weekly: QuotaBucketSummary
  codeReviewWeekly: QuotaBucketSummary
}

export interface CodexQuotaSnapshot {
  plans: CodexPlanQuotaSummary[]
  accounts: CodexQuotaAccountDetail[]
  source: string
  coverage: string
  coveredAccounts: number
  fetchedAt: string
  totalAccounts: number
  successfulAccounts: number
  failedAccounts: number
}

export type QuotaViewMode = 'overview' | 'matrix' | 'recovery'
export type QuotaResultFilter = 'all' | 'success' | 'failed'
export type QuotaSortMode = 'plan' | 'total' | 'fiveHour' | 'weekly' | 'reset' | 'name'
export type QuotaRecoveryMode = 'earliest' | 'fiveHour' | 'weekly'

export interface AccountPage {
  records: AccountRecord[]
  totalRecords: number
  page: number
  pageSize: number
  providerOptions: string[]
  planOptions: string[]
}

export interface InventorySyncResult {
  totalAccounts: number
  filteredAccounts: number
  syncedAt: string
}

export interface AuthImportFileResult {
  path: string
  name: string
  ok: boolean
  error: string
  archived: boolean
  archiveError: string
}

export interface AuthImportResult {
  requested: number
  uploaded: number
  failed: number
  skipped: number
  archived: number
  archiveFailed: number
  archiveDirectory: string
  synced: boolean
  syncError: string
  inventory: InventorySyncResult
  results: AuthImportFileResult[]
}

export interface MaintainOptions {
  delete401: boolean
  quotaAction: string
  autoReenable: boolean
}

export interface ActionResult {
  name: string
  ok: boolean
  action: string
  disabled?: boolean | null
  statusCode?: number | null
  error: string
}

export interface BulkAccountActionResult {
  action: string
  requested: number
  processed: number
  succeeded: number
  failed: number
  skipped: number
  results: ActionResult[]
}

export interface ExportResult {
  kind: string
  format: string
  path: string
  exported: number
}

export interface ScanSummary {
  runId: number
  status: string
  startedAt: string
  finishedAt: string
  totalAccounts: number
  filteredAccounts: number
  probedAccounts: number
  normalCount: number
  invalid401Count: number
  quotaLimitedCount: number
  recoveredCount: number
  errorCount: number
  delete401: boolean
  quotaAction: string
  autoReenable: boolean
  probeWorkers: number
  actionWorkers: number
  timeoutSeconds: number
  retries: number
  message: string
}

export interface ScanDetail {
  summary: ScanSummary
  records: AccountRecord[]
}

export interface ScanDetailPage {
  summary: ScanSummary
  records: AccountRecord[]
  totalRecords: number
  page: number
  pageSize: number
}

export interface MaintainResult {
  scan: ScanSummary
  delete401Results: ActionResult[]
  quotaActionResults: ActionResult[]
  reenableResults: ActionResult[]
}

export interface TaskProgress {
  kind: TaskKind
  phase: string
  current: number
  total: number
  message: string
  done: boolean
}

export interface TaskFinished {
  kind: TaskKind
  status: string
  message: string
}

export interface LogEntry {
  id?: string
  kind: TaskKind
  level: string
  message: string
  timestamp: string
  progress?: boolean
}

export interface AccountUpdate {
  action: string
  removed: boolean
  record: AccountRecord
}

export type ViewKey = 'dashboard' | 'accounts' | 'quotas' | 'logs' | 'settings'
