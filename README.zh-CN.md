[English](./README.md) | [简体中文](./README.zh-CN.md)

# CPA Control Center

面向 CPA 管理的 Codex auth 池桌面运维工具。

`CPA Control Center` 把现有 CPA 管理接口整合成一个专注的桌面应用，适合那些在 auth 池规模变大后，不想继续依赖浏览器页面、localhost 面板或终端脚本来维护池子的操作人员。

你只需要填写 `Base URL` 和 `Management Token`，就可以在一个原生窗口里完成库存同步、扫描、维护、定时任务、日志、历史记录和导出。

## 致谢与目标后端

- 本项目明确借鉴并参考了 [`fantasticjoe/cpa-warden`](https://github.com/fantasticjoe/cpa-warden) 的工作流设计。
- 这个桌面工具当前主要面向 [`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI) 作为 CPA 后端使用。

## 概览

- 基于 Wails、Go、Vue 3、TypeScript 的原生桌面应用
- 只需要 `Base URL` 和 `Management Token`
- 大池子优先走 inventory 同步的启动流程
- 独立的 Codex quota 工作区，包含套餐总览、账号矩阵、恢复时间板和详情面板
- 账号列表与扫描详情都支持后端分页
- 统一状态模型：`Pending`、`Normal`、`401 Invalid`、`Quota Limited`、`Recovered`、`Error`
- 支持 `Full` 与 `Incremental` 两种扫描模式
- 内置应用内调度器，可自动执行 `Scan` 或 `Maintain`
- 独立的 quota 自动刷新 cron，可在应用打开期间周期性刷新 Codex 额度快照
- 实时任务日志与扫描历史
- 支持导出 `401 Invalid` 和 `Quota Limited` 为 CSV / JSON
- 内置中英文双语界面

## 适合谁使用

如果你符合下面这些情况，这个项目就适合你：

- 已经部署了 CPA 并启用了管理接口
- 维护的是一个以 Codex 为主的 auth 池
- 希望用桌面应用直接操作，而不是额外部署一个后台
- 希望把扫描、维护、日志和导出集中在一个工具中

它当前不聚焦这些方向：

- 在 GUI 里创建或导入 auth 文件
- 在桌面端内部完成 OAuth 登录流程
- 做成一个覆盖 CPA 全部能力的通用管理后台

## 它解决什么问题

大 auth 池通常不是“整体瞬间失效”，而是逐步进入混合状态：

- 有些账号已经是 `401 Invalid`
- 有些账号触发了额度限制
- 有些账号探测失败，但重试后会恢复
- 有些账号之前被禁用，现在已经可恢复
- 有些账号已经同步到本地，但还没做过探测

这个应用围绕两个目标构建：

1. 让你快速、稳定地看清当前池子的健康状况。
2. 让你基于最新扫描结果执行可重复、可控的维护规则。

## 当前工作流

当前产品流程被有意拆成几个阶段：

1. 保存 CPA 连接配置。
2. 先从 CPA 同步 inventory 到本地快照。
3. 在第一次扫描前，也能先看到已追踪账号。
4. 需要健康状态时再执行扫描。
5. 看过最新扫描结果后再执行维护。
6. 如果需要，可在应用打开期间启用定时扫描或定时维护。

这对大池子很重要。首次连接不再需要一上来就探测成千上万个账号，界面也不需要等全量扫描结束后才可用。

## 首次连接行为

现在 **Test & Save** 成功后，应用会执行一次远端 inventory 拉取，同时完成：

- 连接有效性验证
- 本地 inventory 快照同步

这意味着：

- 首次配置时不会重复拉取两次完整 auth 列表
- 新出现的记录会被标记为 `Pending`
- 仪表盘可以立刻显示已追踪数量
- 账号列表可以立刻使用
- 完整健康分类仍然要等第一次扫描完成后才建立

## 状态模型

应用在仪表盘、账号列表、维护和导出中统一使用一套紧凑状态模型：

- `Pending`：已同步到本地，但尚未探测
- `Normal`
- `401 Invalid`
- `Quota Limited`
- `Recovered`
- `Error`

`Pending` 在大池子和首次接入时非常关键。它表示“库存已知但尚未扫描”，而不是探测失败或空数据。

## 核心能力

### 1. 连接到 CPA

只需要填写：

- `Base URL`
- `Management Token`

连接测试本身不会触发 usage 探测，但 **Test & Save** 会同步 inventory 快照，让界面立即可用。

### 2. 同步 Inventory

inventory 同步会：

- 从 CPA 拉取 auth 元数据
- 建立本地追踪池
- 为大池子启动做准备
- 避免首次使用时仪表盘完全空白

### 3. 扫描账号池

点击 **Scan Now** 后，应用会：

1. 从 CPA 重新拉取最新 auth inventory
2. 按 `targetType` 和 `provider` 过滤
3. 并发探测匹配账号
4. 写入最新本地快照
5. 复用本次探测结果，为已探测到的 Codex 账号同步更新额度快照，不再额外发起一次独立的额度刷新请求
6. 生成一条扫描历史记录

当前支持两种扫描模式：

- `Full`：探测当前过滤范围内全部账号
- `Incremental`：每次只探测一个批次，优先处理 `Pending` 账号，其次处理最久未探测账号

`Incremental Batch Size` 用来控制一次增量扫描最多探测多少个账号。

### 4. 执行维护

点击 **Run Maintain** 后，应用会先执行一次新的扫描，然后按配置执行：

- 删除 `401 Invalid` 账号
- 对 `Quota Limited` 账号执行 `disable` 或 `delete`
- 对 `Recovered` 账号重新启用

所有破坏性动作都需要先确认。

维护始终基于一次新的全量扫描，不会走增量批次。

### 5. 查看历史与日志

应用会保留：

- 当前账号快照
- 最近扫描历史
- 分页扫描详情
- 实时任务日志与进度事件

如果启用 **Detailed Logs**，任务日志还会显示逐账号扫描和逐账号维护细节。

### 6. 查看 Codex Quota

quota 工作区不再只显示套餐聚合卡片，现在包含：

- 套餐级总览卡片，用于查看池化后的 quota 健康度
- 账号矩阵，可查看成功与失败的额度快照
- 恢复时间板，可按 `最早恢复`、`5h` 或 `周额度` 的重置时间窗口分组
- 账号详情面板，可查看 bucket 级状态与失败原因

quota 快照现在有四种更新方式：

- 扫描完成后自动更新
- 维护完成后自动更新
- 在额度页手动刷新
- 通过独立的 quota 自动刷新 cron 在应用打开期间定时更新

### 7. 自动任务调度

应用内置一个调度器：

- 每个应用实例只有一个全局计划
- 动作类型可以是 `Scan` 或 `Maintain`
- 使用本地系统时间和标准 5 段 cron 表达式
- 保存设置后立即热重载
- 如果当前已有扫描或维护任务在跑，本次调度会跳过

当前边界：

- 调度只在应用打开期间生效
- 错过的执行不会在重启后补跑
- 第一版不支持多个计划，也不接入操作系统级定时任务

### 8. 导出问题账号

你可以导出当前：

- `401 Invalid` 账号
- `Quota Limited` 账号

支持格式：

- JSON
- CSV

## 页面结构

### Dashboard

- 池子健康总览
- 最近扫描历史
- 分页扫描详情抽屉
- 一键扫描与一键维护入口

### Codex Quotas

- 按套餐分组的总览卡片
- 账号矩阵，支持结果过滤、额度排序和按行分页
- 恢复时间板，支持切换恢复类别（`最早恢复`、`5h`、`周额度`）
- 账号详情抽屉，用于查看 bucket 状态、重置时间和失败原因
- 手动刷新和可选的应用内 quota 自动刷新 cron

### Accounts

- 后端分页账号表
- 由后端处理的全量搜索
- 状态与 provider 过滤
- 单账号探测、禁用/启用、删除操作

### Logs

- 实时任务流
- 当前进度始终可见
- 可选的逐账号详细日志

### Settings

- CPA 连接参数
- 语言切换
- 扫描模式与增量批次大小
- 并发与超时配置
- 重试次数
- quota 处理策略
- 应用内调度器启用/模式/cron
- quota 自动刷新开关与 cron
- 导出目录
- 详细日志开关
- 高级参数说明气泡

## 大池子说明

当前实现已经针对几千到几万账号规模做过一轮优化：

- 首次连接优先同步 inventory，而不是立即全量探测
- **Test & Save** 用一次远端请求同时完成连接验证和 inventory 同步
- 仪表盘不再把全量账号塞到前端
- 账号列表改为后端分页
- 扫描详情改为后端分页
- 已同步但未探测的账号稳定保留为 `Pending`
- 可通过 `Incremental` 模式把一次性探测压力拆成多个批次

这还不是最终性能上限，但已经明显比早期“前端全量快照”模式稳定得多。

## 使用到的 CPA 接口

当前应用只依赖少量管理接口：

- `GET /v0/management/auth-files`
- `POST /v0/management/api-call`
- `DELETE /v0/management/auth-files?name=...`
- `PATCH /v0/management/auth-files/status`

健康探测通过 CPA 转发请求到：

- `https://chatgpt.com/backend-api/wham/usage`

## 默认行为

| 配置项 | 默认值 |
| --- | --- |
| 语言 | 归一化系统语言（`en-US` / `zh-CN`） |
| 目标类型 | `codex` |
| 扫描模式 | `full` |
| 增量批次大小 | `1000` |
| 探测并发 | `40` |
| 动作并发 | `20` |
| 超时 | `15s` |
| 重试次数 | `3` |
| quota 处理 | `disable` |
| 删除 401 | 开启 |
| 自动恢复已恢复账号 | 开启 |
| 调度器 | 关闭 |
| 详细日志 | 关闭 |

## 重试模型

重试分成两层：

- 请求层重试：处理外层请求失败，以及 CPA 侧的瞬时错误，例如 `408`、`429`、`5xx`
- 探测层重试：处理可恢复的探测异常，例如临时上游错误、返回体不完整、可重试状态码

不会盲目重试的场景包括：

- `401 Invalid`
- `Quota Limited`
- 明确缺失账号元数据

## 本地数据存储

应用会把本地状态保存在系统用户配置目录下的：

`CPA Control Center/`

目录通常包含：

- `settings.json`
- `state.db`
- `app.log`
- `exports/`

当前实现会保留最新快照，以及最近 `30` 次扫描历史。

## 项目结构

```text
cpa-control-center/
|- frontend/                     # Vue 3 + TypeScript 前端
|- internal/backend/             # CPA 客户端、状态存储、任务编排
|- build/                        # Wails 构建资源与平台打包配置
|- scripts/build-macos.sh        # macOS 构建脚本
|- .github/workflows/            # CI / Release 工作流
|- app.go                        # Wails 绑定层
|- main.go                       # 共享入口
|- platform_options_windows.go   # Windows 窗口配置
|- platform_options_darwin.go    # macOS 窗口配置
`- wails.json                    # Wails 项目配置
```

## 开发与构建

### 环境要求

- Go `1.24+`
- 推荐 Node.js `22+`
- Wails CLI `v2.11.0`

### 开发模式

```bash
wails dev
```

### Windows 构建

```bash
wails build
```

### macOS 构建

请在 Mac 或 `macos-latest` GitHub Actions runner 上执行：

```bash
bash ./scripts/build-macos.sh
```

## 安全建议

- 这是运维工具，不是演示页面。维护动作可能删除或禁用账号。
- 执行维护前，最好先看一遍最新扫描结果。
- 如果你在验证新的 CPA 环境，建议先关闭 `delete401`。
- 详细日志适合排障，但在大池子下会更嘈杂。
- 对超大池子，优先使用 `Incremental` 模式，除非你明确要做一次全量健康检查。

## 当前范围

当前项目聚焦于：

- 管理已存在的 CPA auth 文件
- Codex 池健康探测与维护
- Windows 优先的桌面体验，同时补齐 macOS 构建链路

当前不包含：

- auth 导入向导
- 应用内登录 / OAuth 获取
- 多节点统一编排
- 超出本地快照和历史视图之外的高级分析能力

## FAQ

### 会打开浏览器吗？

不会。它是一个 Wails 桌面应用，会直接打开自己的原生窗口。

### 它是完整的 CPA 后台吗？

不是。它是一个专注于 auth 池健康和维护的桌面运维工具。

### 为什么还没扫描就能看到已追踪账号？

因为现在会先做 inventory 同步。这样可以先看到已追踪库存，而健康状态会在首次扫描完成后从 `Pending` 变成真实探测结果。

### `Full` 和 `Incremental` 扫描有什么区别？

`Full` 会扫描当前过滤范围内全部账号；`Incremental` 每次只扫描一个批次，优先处理未探测账号，再处理最久未探测账号，适合大池子降压。

### “Run Maintain” 会先扫描吗？

会。维护一定会先跑一次新的扫描，再应用维护规则。

### 扫描详情支持大结果集吗？

支持。扫描详情已经改成后端分页，不会一次性把所有数据都塞进抽屉。

### 有调试模式吗？

有，但默认隐藏。按 `Ctrl + Shift + D` 可以打开内部调试面板，用于排查启动和仪表盘问题。

### 定时任务会在应用关闭时继续运行吗？

不会。内置调度器和 quota 自动刷新都只在桌面应用打开期间生效，关闭应用后不会补跑。

### 支持 macOS 吗？

构建链路已经补齐。正式的 macOS 产物建议在 Mac 或 `macos-latest` GitHub Actions runner 上生成。

## 路线图

- 增加可配置阈值规则，例如低于指定百分比时自动禁用账号
- 支持更多 auth 渠道维护
- 增加更丰富的统计与趋势视图
- 在需要对外分发时补齐签名 / notarization 流程

## 当前状态

它已经可以用于真实的 CPA 池运维，但仍然是一个持续迭代中的实用工具。当前优先级始终是可靠、清晰、可维护，而不是盲目堆功能。
