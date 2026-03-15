package backend

type AppSettings struct {
	BaseURL                 string           `json:"baseUrl"`
	ManagementToken         string           `json:"managementToken"`
	Locale                  string           `json:"locale"`
	DetailedLogs            bool             `json:"detailedLogs"`
	TargetType              string           `json:"targetType"`
	Provider                string           `json:"provider"`
	ScanStrategy            string           `json:"scanStrategy"`
	ScanBatchSize           int              `json:"scanBatchSize"`
	SkipKnown401            bool             `json:"skipKnown401"`
	ProbeWorkers            int              `json:"probeWorkers"`
	ActionWorkers           int              `json:"actionWorkers"`
	QuotaWorkers            int              `json:"quotaWorkers"`
	TimeoutSeconds          int              `json:"timeoutSeconds"`
	Retries                 int              `json:"retries"`
	UserAgent               string           `json:"userAgent"`
	QuotaAction             string           `json:"quotaAction"`
	QuotaCheckFree          bool             `json:"quotaCheckFree"`
	QuotaCheckPlus          bool             `json:"quotaCheckPlus"`
	QuotaCheckPro           bool             `json:"quotaCheckPro"`
	QuotaCheckTeam          bool             `json:"quotaCheckTeam"`
	QuotaCheckBusiness      bool             `json:"quotaCheckBusiness"`
	QuotaCheckEnterprise    bool             `json:"quotaCheckEnterprise"`
	QuotaFreeMaxAccounts    int              `json:"quotaFreeMaxAccounts"`
	QuotaAutoRefreshEnabled bool             `json:"quotaAutoRefreshEnabled"`
	QuotaAutoRefreshCron    string           `json:"quotaAutoRefreshCron"`
	Delete401               bool             `json:"delete401"`
	AutoReenable            bool             `json:"autoReenable"`
	ExportDirectory         string           `json:"exportDirectory"`
	Schedule                ScheduleSettings `json:"schedule"`
}

type ScheduleSettings struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Cron    string `json:"cron"`
}

type SchedulerStatus struct {
	Enabled           bool   `json:"enabled"`
	Mode              string `json:"mode"`
	Cron              string `json:"cron"`
	Valid             bool   `json:"valid"`
	ValidationMessage string `json:"validationMessage"`
	Running           bool   `json:"running"`
	NextRunAt         string `json:"nextRunAt"`
	LastStartedAt     string `json:"lastStartedAt"`
	LastFinishedAt    string `json:"lastFinishedAt"`
	LastStatus        string `json:"lastStatus"`
	LastMessage       string `json:"lastMessage"`
}

type ConnectionResult struct {
	OK           bool   `json:"ok"`
	Message      string `json:"message"`
	AccountCount int    `json:"accountCount"`
	CheckedAt    string `json:"checkedAt"`
}

type AccountFilter struct {
	Query    string `json:"query"`
	State    string `json:"state"`
	Provider string `json:"provider"`
	Type     string `json:"type"`
	PlanType string `json:"planType"`
	Disabled *bool  `json:"disabled"`
}

type AccountRecord struct {
	Name             string `json:"name"`
	AuthIndex        string `json:"authIndex"`
	Email            string `json:"email"`
	Provider         string `json:"provider"`
	Type             string `json:"type"`
	PlanType         string `json:"planType"`
	Account          string `json:"account"`
	Source           string `json:"source"`
	Status           string `json:"status"`
	StatusMessage    string `json:"statusMessage"`
	State            string `json:"state"`
	StateKey         string `json:"stateKey"`
	Disabled         bool   `json:"disabled"`
	Unavailable      bool   `json:"unavailable"`
	RuntimeOnly      bool   `json:"runtimeOnly"`
	Allowed          *bool  `json:"allowed"`
	LimitReached     *bool  `json:"limitReached"`
	Invalid401       bool   `json:"invalid401"`
	QuotaLimited     bool   `json:"quotaLimited"`
	Recovered        bool   `json:"recovered"`
	Error            bool   `json:"error"`
	APIHTTPStatus    *int   `json:"apiHttpStatus"`
	APIStatusCode    *int   `json:"apiStatusCode"`
	ProbeErrorKind   string `json:"probeErrorKind"`
	ProbeErrorText   string `json:"probeErrorText"`
	ManagedReason    string `json:"managedReason"`
	LastAction       string `json:"lastAction"`
	LastActionStatus string `json:"lastActionStatus"`
	LastActionError  string `json:"lastActionError"`
	LastSeenAt       string `json:"lastSeenAt"`
	LastProbedAt     string `json:"lastProbedAt"`
	UpdatedAt        string `json:"updatedAt"`
	ChatGPTAccountID string `json:"chatgptAccountId"`
	IDTokenPlanType  string `json:"idTokenPlanType"`
	AuthUpdatedAt    string `json:"authUpdatedAt"`
	AuthModTime      string `json:"authModTime"`
	AuthLastRefresh  string `json:"authLastRefresh"`
}

type DashboardSummary struct {
	TotalAccounts     int    `json:"totalAccounts"`
	FilteredAccounts  int    `json:"filteredAccounts"`
	PendingCount      int    `json:"pendingCount"`
	NormalCount       int    `json:"normalCount"`
	Invalid401Count   int    `json:"invalid401Count"`
	QuotaLimitedCount int    `json:"quotaLimitedCount"`
	RecoveredCount    int    `json:"recoveredCount"`
	ErrorCount        int    `json:"errorCount"`
	LastScanAt        string `json:"lastScanAt"`
}

type DashboardSnapshot struct {
	Summary DashboardSummary `json:"summary"`
	History []ScanSummary    `json:"history"`
}

type QuotaBucketSummary struct {
	Supported             bool     `json:"supported"`
	TotalRemainingPercent *float64 `json:"totalRemainingPercent"`
	ResetAt               string   `json:"resetAt"`
	SuccessCount          int      `json:"successCount"`
	FailedCount           int      `json:"failedCount"`
}

type QuotaBucketDetail struct {
	Supported        bool     `json:"supported"`
	RemainingPercent *float64 `json:"remainingPercent"`
	ResetAt          string   `json:"resetAt"`
}

type CodexQuotaAccountDetail struct {
	Name             string            `json:"name"`
	Email            string            `json:"email"`
	PlanType         string            `json:"planType"`
	Provider         string            `json:"provider"`
	Success          bool              `json:"success"`
	Error            string            `json:"error"`
	FetchedAt        string            `json:"fetchedAt"`
	EarliestResetAt  string            `json:"earliestResetAt"`
	FiveHour         QuotaBucketDetail `json:"fiveHour"`
	Weekly           QuotaBucketDetail `json:"weekly"`
	CodeReviewWeekly QuotaBucketDetail `json:"codeReviewWeekly"`
}

type CodexPlanQuotaSummary struct {
	PlanType         string             `json:"planType"`
	AccountCount     int                `json:"accountCount"`
	FiveHour         QuotaBucketSummary `json:"fiveHour"`
	Weekly           QuotaBucketSummary `json:"weekly"`
	CodeReviewWeekly QuotaBucketSummary `json:"codeReviewWeekly"`
}

type CodexQuotaSnapshot struct {
	Plans              []CodexPlanQuotaSummary   `json:"plans"`
	Accounts           []CodexQuotaAccountDetail `json:"accounts"`
	FetchedAt          string                    `json:"fetchedAt"`
	TotalAccounts      int                       `json:"totalAccounts"`
	SuccessfulAccounts int                       `json:"successfulAccounts"`
	FailedAccounts     int                       `json:"failedAccounts"`
}

type AccountPage struct {
	Records         []AccountRecord `json:"records"`
	TotalRecords    int             `json:"totalRecords"`
	Page            int             `json:"page"`
	PageSize        int             `json:"pageSize"`
	ProviderOptions []string        `json:"providerOptions"`
	PlanOptions     []string        `json:"planOptions"`
}

type InventorySyncResult struct {
	TotalAccounts    int    `json:"totalAccounts"`
	FilteredAccounts int    `json:"filteredAccounts"`
	SyncedAt         string `json:"syncedAt"`
}

type MaintainOptions struct {
	Delete401    bool   `json:"delete401"`
	QuotaAction  string `json:"quotaAction"`
	AutoReenable bool   `json:"autoReenable"`
}

type MaintainResult struct {
	Scan               ScanSummary    `json:"scan"`
	Delete401Results   []ActionResult `json:"delete401Results"`
	QuotaActionResults []ActionResult `json:"quotaActionResults"`
	ReenableResults    []ActionResult `json:"reenableResults"`
}

type ActionResult struct {
	Name       string `json:"name"`
	OK         bool   `json:"ok"`
	Action     string `json:"action"`
	Disabled   *bool  `json:"disabled"`
	StatusCode *int   `json:"statusCode"`
	Error      string `json:"error"`
}

type BulkAccountActionResult struct {
	Action    string         `json:"action"`
	Requested int            `json:"requested"`
	Processed int            `json:"processed"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Skipped   int            `json:"skipped"`
	Results   []ActionResult `json:"results"`
}

type ExportRequest struct {
	Kind   string `json:"kind"`
	Format string `json:"format"`
	Path   string `json:"path"`
}

type ExportResult struct {
	Kind     string `json:"kind"`
	Format   string `json:"format"`
	Path     string `json:"path"`
	Exported int    `json:"exported"`
}

type ScanSummary struct {
	RunID             int64  `json:"runId"`
	Status            string `json:"status"`
	StartedAt         string `json:"startedAt"`
	FinishedAt        string `json:"finishedAt"`
	TotalAccounts     int    `json:"totalAccounts"`
	FilteredAccounts  int    `json:"filteredAccounts"`
	ProbedAccounts    int    `json:"probedAccounts"`
	NormalCount       int    `json:"normalCount"`
	Invalid401Count   int    `json:"invalid401Count"`
	QuotaLimitedCount int    `json:"quotaLimitedCount"`
	RecoveredCount    int    `json:"recoveredCount"`
	ErrorCount        int    `json:"errorCount"`
	Delete401         bool   `json:"delete401"`
	QuotaAction       string `json:"quotaAction"`
	AutoReenable      bool   `json:"autoReenable"`
	ProbeWorkers      int    `json:"probeWorkers"`
	ActionWorkers     int    `json:"actionWorkers"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	Retries           int    `json:"retries"`
	Message           string `json:"message"`
}

type ScanDetail struct {
	Summary ScanSummary     `json:"summary"`
	Records []AccountRecord `json:"records"`
}

type ScanDetailPage struct {
	Summary      ScanSummary     `json:"summary"`
	Records      []AccountRecord `json:"records"`
	TotalRecords int             `json:"totalRecords"`
	Page         int             `json:"page"`
	PageSize     int             `json:"pageSize"`
}

type TaskProgress struct {
	Kind    string `json:"kind"`
	Phase   string `json:"phase"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
	Done    bool   `json:"done"`
}

type TaskFinished struct {
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type LogEntry struct {
	Kind      string `json:"kind"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type AccountUpdate struct {
	Action  string        `json:"action"`
	Removed bool          `json:"removed"`
	Record  AccountRecord `json:"record"`
}
