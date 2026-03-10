package backend

import (
	"fmt"
	"os"
	"strings"
)

const (
	localeEnglish = "en-US"
	localeChinese = "zh-CN"

	statePending      = "pending"
	stateNormal       = "normal"
	stateInvalid401   = "invalid_401"
	stateQuotaLimited = "quota_limited"
	stateRecovered    = "recovered"
	stateError        = "error"
	stateUntracked    = "untracked"
)

var translations = map[string]map[string]string{
	localeEnglish: {
		"common.yes":                       "yes",
		"common.no":                        "no",
		"settings.summary":                 "type=%s provider=%s scan=%s batch=%d probe workers=%d action workers=%d timeout=%ds retries=%d quota action=%s delete 401=%s auto re-enable=%s",
		"settings.saved":                   "Saved settings for %s",
		"connection.success":               "Connection successful.",
		"task.inventory.synced":            "Synced %d filtered accounts from %d inventory records.",
		"error.schedule_invalid_mode":      "Schedule mode must be scan or maintain.",
		"error.schedule_cron_required":     "Schedule cron expression is required.",
		"error.schedule_invalid_cron":      "Schedule cron expression is invalid",
		"error.scan_invalid_strategy":      "Scan strategy must be full or incremental.",
		"error.scan_batch_size_invalid":    "Incremental scan batch size must be at least 1.",
		"error.base_url_required":          "Base URL is required.",
		"error.management_token_required":  "Management token is required.",
		"error.response_missing_files":     "Management response missing files field.",
		"error.response_files_not_list":    "Management response files field is not a list.",
		"error.response_invalid_json":      "Management response is not valid JSON.",
		"error.body_invalid_json":          "API-call body is not valid JSON.",
		"error.body_not_object":            "API-call body is not a JSON object.",
		"error.missing_status_code":        "Missing status_code in API-call response.",
		"error.missing_chatgpt_account_id": "Missing ChatGPT account ID.",
		"error.unexpected_upstream_status": "Unexpected upstream status_code=%d.",
		"error.management_api_http":        "Management API HTTP %d: %s",
		"error.account_not_found":          "Account not found: %s",
		"error.unsupported_export_format":  "Unsupported export format: %s",
		"error.task_already_running":       "%s is already running.",
		"task.scan.started":                "Starting %s: %s",
		"task.scan.loading_inventory":      "Loading auth inventory",
		"task.scan.loaded_auth_files":      "Loaded %d auth files",
		"task.scan.prepared_candidates":    "Prepared %d candidate accounts from %d inventory records",
		"task.scan.incremental_selected":   "Selected %d accounts from %d candidates using incremental scan (batch size %d)",
		"task.scan.saved_snapshot":         "Saved snapshot and scan history",
		"task.scan.completed":              "Scanned %d filtered accounts",
		"task.scan.completed_partial":      "Scanned %d accounts from %d filtered candidates",
		"task.scan.no_candidates":          "No accounts matched the active filter",
		"task.scan.probed_account":         "Probed %s",
		"task.scan.single_probe":           "Probed account %s -> %s",
		"task.scan.retry_probe":            "Retrying account %s (%d/%d) because %s",
		"task.scan.failed_auth_files":      "Failed to load auth files: %v",
		"task.scan.stopped":                "%s stopped: %v",
		"task.schedule.triggered":          "Scheduled %s triggered by cron %s",
		"task.schedule.skipped_active":     "Scheduled %s skipped because %s is already running",
		"task.schedule.completed":          "Scheduled %s completed",
		"task.schedule.failed":             "Scheduled %s failed: %v",
		"task.maintain.delete_invalid":     "Deleting %d invalid accounts",
		"task.maintain.disable_quota":      "Disabling %d quota-limited accounts",
		"task.maintain.delete_quota":       "Deleting %d quota-limited accounts",
		"task.maintain.reenable":           "Re-enabling %d recovered accounts",
		"task.maintain.completed":          "Maintenance completed",
		"task.action.none_queued":          "No accounts queued",
		"task.action.success":              "%s %s succeeded",
		"task.action.failed":               "%s %s: %s",
		"task.action.probe":                "Probe",
		"task.action.delete":               "Delete",
		"task.action.disable":              "Disable",
		"task.action.enable":               "Enable",
		"task.account.set_disabled":        "Set account %s disabled=%s",
		"task.account.deleted":             "Deleted account %s",
		"task.account.batch_summary":       "%s accounts summary: requested=%d, processed=%d, succeeded=%d, failed=%d, skipped=%d",
		"task.export.completed":            "Exported %d %s accounts to %s",
		"task.name.scan":                   "Scan",
		"task.name.maintain":               "Maintain",
		"export.kind.invalid401":           "401-invalid",
		"export.kind.quotaLimited":         "quota-limited",
		"csv.header.name":                  "name",
		"csv.header.email":                 "email",
		"csv.header.provider":              "provider",
		"csv.header.type":                  "type",
		"csv.header.plan_type":             "plan_type",
		"csv.header.state":                 "state",
		"csv.header.disabled":              "disabled",
		"csv.header.status_message":        "status_message",
		"csv.header.probe_error_text":      "probe_error_text",
		"csv.header.last_probed_at":        "last_probed_at",
		"csv.header.last_action":           "last_action",
		"csv.header.last_action_status":    "last_action_status",
		"state.pending":                    "Pending",
		"state.normal":                     "Normal",
		"state.invalid_401":                "401 Invalid",
		"state.quota_limited":              "Quota Limited",
		"state.recovered":                  "Recovered",
		"state.error":                      "Error",
		"state.untracked":                  "Untracked",
	},
	localeChinese: {
		"common.yes":                       "是",
		"common.no":                        "否",
		"settings.summary":                 "类型=%s 提供方=%s 扫描策略=%s 批次=%d 探测并发=%d 动作并发=%d 超时=%d秒 重试=%d 限额动作=%s 删除401=%s 自动恢复=%s",
		"settings.saved":                   "已保存 %s 的设置",
		"connection.success":               "连接成功。",
		"task.inventory.synced":            "已同步库存：%d 个过滤后账号，来自 %d 条清单记录。",
		"error.schedule_invalid_mode":      "定时任务动作只能是 scan 或 maintain。",
		"error.schedule_cron_required":     "启用定时任务后必须填写 cron 表达式。",
		"error.schedule_invalid_cron":      "cron 表达式无效",
		"error.scan_invalid_strategy":      "扫描策略只能是 full 或 incremental。",
		"error.scan_batch_size_invalid":    "增量扫描批次大小至少为 1。",
		"error.base_url_required":          "必须填写 Base URL。",
		"error.management_token_required":  "必须填写 Management Token。",
		"error.response_missing_files":     "管理接口返回缺少 files 字段。",
		"error.response_files_not_list":    "管理接口的 files 字段不是列表。",
		"error.response_invalid_json":      "管理接口返回不是合法 JSON。",
		"error.body_invalid_json":          "api-call 的 body 不是合法 JSON。",
		"error.body_not_object":            "api-call 的 body 不是 JSON 对象。",
		"error.missing_status_code":        "api-call 返回缺少 status_code。",
		"error.missing_chatgpt_account_id": "缺少 ChatGPT Account ID。",
		"error.unexpected_upstream_status": "收到异常的上游 status_code=%d。",
		"error.management_api_http":        "管理接口 HTTP %d：%s",
		"error.account_not_found":          "未找到账号：%s",
		"error.unsupported_export_format":  "不支持的导出格式：%s",
		"error.task_already_running":       "%s 任务正在执行中。",
		"task.scan.started":                "开始执行%s：%s",
		"task.scan.loading_inventory":      "正在加载 auth 清单",
		"task.scan.loaded_auth_files":      "已加载 %d 个 auth 文件",
		"task.scan.prepared_candidates":    "已从 %d 条清单中整理出 %d 个候选账号",
		"task.scan.incremental_selected":   "增量扫描本次选择 %d 个账号，候选总数 %d，批次大小 %d",
		"task.scan.saved_snapshot":         "已保存当前快照和扫描历史",
		"task.scan.completed":              "已扫描 %d 个过滤后的账号",
		"task.scan.completed_partial":      "已从 %d 个过滤后候选账号中扫描 %d 个",
		"task.scan.no_candidates":          "当前过滤条件下没有匹配账号",
		"task.scan.probed_account":         "已探测 %s",
		"task.scan.single_probe":           "已探测账号 %s -> %s",
		"task.scan.retry_probe":            "账号 %s 第 %d/%d 次重试，原因：%s",
		"task.scan.failed_auth_files":      "拉取 auth 文件失败：%v",
		"task.scan.stopped":                "%s 已停止：%v",
		"task.schedule.triggered":          "定时%s已触发，cron：%s",
		"task.schedule.skipped_active":     "定时%s已跳过，因为%s正在执行",
		"task.schedule.completed":          "定时%s已完成",
		"task.schedule.failed":             "定时%s执行失败：%v",
		"task.maintain.delete_invalid":     "正在删除 %d 个失效账号",
		"task.maintain.disable_quota":      "正在禁用 %d 个限额账号",
		"task.maintain.delete_quota":       "正在删除 %d 个限额账号",
		"task.maintain.reenable":           "正在恢复启用 %d 个已恢复账号",
		"task.maintain.completed":          "维护流程已完成",
		"task.action.none_queued":          "当前没有待处理账号",
		"task.action.success":              "%s %s 成功",
		"task.action.failed":               "%s %s 失败：%s",
		"task.action.probe":                "探测",
		"task.action.delete":               "删除",
		"task.action.disable":              "禁用",
		"task.action.enable":               "启用",
		"task.account.set_disabled":        "已设置账号 %s disabled=%s",
		"task.account.deleted":             "已删除账号 %s",
		"task.account.batch_summary":       "%s账号批量操作：请求=%d，处理=%d，成功=%d，失败=%d，跳过=%d",
		"task.export.completed":            "已将 %d 条%s账号导出到 %s",
		"task.name.scan":                   "扫描",
		"task.name.maintain":               "维护",
		"export.kind.invalid401":           "401失效",
		"export.kind.quotaLimited":         "限额",
		"csv.header.name":                  "名称",
		"csv.header.email":                 "邮箱",
		"csv.header.provider":              "提供方",
		"csv.header.type":                  "类型",
		"csv.header.plan_type":             "套餐",
		"csv.header.state":                 "状态",
		"csv.header.disabled":              "是否禁用",
		"csv.header.status_message":        "状态说明",
		"csv.header.probe_error_text":      "探测错误",
		"csv.header.last_probed_at":        "最近探测时间",
		"csv.header.last_action":           "最近动作",
		"csv.header.last_action_status":    "最近动作结果",
		"state.pending":                    "待探测",
		"state.normal":                     "正常",
		"state.invalid_401":                "401 失效",
		"state.quota_limited":              "额度用尽",
		"state.recovered":                  "可恢复",
		"state.error":                      "错误",
		"state.untracked":                  "未探测",
	},
}

func localeOrDefault(locale string) string {
	normalized := normalizeLocaleCode(locale)
	if normalized != "" {
		return normalized
	}

	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		if candidate := normalizeLocaleCode(os.Getenv(key)); candidate != "" {
			return candidate
		}
	}
	return localeEnglish
}

func normalizeLocaleCode(locale string) string {
	value := strings.TrimSpace(strings.ToLower(locale))
	if value == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(value, "zh"):
		return localeChinese
	default:
		return localeEnglish
	}
}

func msg(locale string, key string, args ...any) string {
	code := localeOrDefault(locale)
	if message, ok := translations[code][key]; ok {
		return fmt.Sprintf(message, args...)
	}
	if message, ok := translations[localeEnglish][key]; ok {
		return fmt.Sprintf(message, args...)
	}
	return key
}

func normalizeStateKey(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case statePending, "待探测":
		return statePending
	case stateNormal, "正常":
		return stateNormal
	case stateInvalid401, "401 invalid", "401 失效", "401失效":
		return stateInvalid401
	case stateQuotaLimited, "quota limited", "额度用尽":
		return stateQuotaLimited
	case stateRecovered, "可恢复":
		return stateRecovered
	case stateError, "错误":
		return stateError
	case stateUntracked, "未探测":
		return stateUntracked
	default:
		return stateUntracked
	}
}

func stateLabel(locale string, stateKey string) string {
	return msg(locale, "state."+normalizeStateKey(stateKey))
}

func taskName(locale string, taskKind string) string {
	switch taskKind {
	case "maintain":
		return msg(locale, "task.name.maintain")
	default:
		return msg(locale, "task.name.scan")
	}
}

func exportKindLabel(locale string, kind string) string {
	switch kind {
	case "quotaLimited":
		return msg(locale, "export.kind.quotaLimited")
	default:
		return msg(locale, "export.kind.invalid401")
	}
}

func boolLabel(locale string, value bool) string {
	if value {
		return msg(locale, "common.yes")
	}
	return msg(locale, "common.no")
}
