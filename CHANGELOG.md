# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

## v1.5.1

### Highlights

- Reused scan and maintenance usage probes to update Codex quota snapshots instead of requiring a separate quota refresh request for the same accounts.
- Persisted the latest Codex quota snapshot in the local store and pushed live snapshot updates to the frontend when scan, maintain, or quota refresh completes.
- Added snapshot source and coverage metadata so the quota workspace can distinguish full refreshes from partial updates driven by scan coverage.

### Notes

- Scans and maintenance runs now update quota data for the accounts they actually probe, while manual quota refresh remains available for a full quota-specific pass.
- The quota page still does not backfill account scan results on its own; quota refresh only updates quota snapshots.

## v1.5.0

### Highlights

- Rebuilt the Codex quota page into a dedicated workspace with three coordinated views: plan overview cards, an account matrix, and a recovery board.
- Added account-level quota details to the backend snapshot so the desktop app can inspect successful and failed quota fetches without flattening everything into plan-only summaries.
- Added richer quota interactions including result filters, plan filters, matrix sorting, row-based pagination, detail side panels, and recovery-mode filtering for earliest, `5h`, and weekly reset windows.
- Added in-app quota auto-refresh support driven by a separate cron setting, with task progress flowing through the existing quota task stream.
- Tightened quota semantics so unsupported `5h` buckets sort correctly, weekly-exhausted accounts no longer show misleading residual `5h` availability, and failure-only snapshots route users toward account-level failure data instead of looking like empty filter results.

### Notes

- The quota workspace is still desktop-first and tuned for dense operational review rather than a lightweight mobile-style card feed.
- Quota auto-refresh only runs while the application is open; it is not an OS-level background job.

## v1.4.0

### Highlights

- Rebuilt the accounts page around a batch-action toolbar with table multi-select, removing the old per-row action column and making probe, enable, disable, and delete workflows faster on large pools.
- Added codex-specific filters for `Plan` and `Disabled` state, plus refreshed account detail presentation with compact pills, truncation, and dialog-based full-detail viewing.
- Added backend batch account APIs for probe, enable, disable, and delete, and now record those account management actions into the task log stream with batch summaries and per-account failures.
- Hardened managed account actions so path-like CPA auth names such as `codex/...json` are normalized correctly for delete and toggle requests while preserving compatibility with the official CPA interfaces.
- Changed Settings `Test Connection` and `Test & Save` to avoid blocking on a full inventory rebuild in the foreground, then continue inventory sync as a background task with visible progress and log output.
- Added a clear info hint on `Target Type` to explain that it filters account type rather than model name, helping prevent users from accidentally hiding their entire pool with values like `gpt-5.2`.

### Notes

- Batch account actions currently apply to the selected rows on the current page only; cross-page bulk selection is still intentionally unsupported.
- The logs view now includes inventory-sync task status so long-running refreshes on pools with tens of thousands of accounts are visible instead of looking frozen.
- On newer CPA builds, connection testing now uses a lightweight management endpoint first and only falls back to fetching `auth-files` when the older endpoint shape requires it.
- Inventory refresh after saving settings is now queued behind active scan or maintenance work instead of surfacing a misleading warning when another task is already running.

## v1.3.0

### Highlights

- Added configurable skipping for known `401` responses so scans can avoid repeatedly reprocessing accounts that are already known to fail with unauthorized results.
- Improved scan record persistence by upserting duplicate records within the same run instead of producing conflicting duplicates.
- Rebuilt the desktop shell into window-driven `wide`, `desktop`, and `compact` layout modes so the app now expands on large screens and stays readable on smaller windows without the old fixed-canvas scaling behavior.
- Reworked the dashboard, sidebar, account/log/settings layouts, and scan detail drawer to follow the new shell modes with tighter compact layouts and better internal scrolling behavior.
- Updated the pool health donut to size from its container, keep the chart centered across shell modes, and stay stable during first render and resize changes.
- Changed startup window sizing on Windows and macOS to prefer the best desktop size while shrinking to the current screen work area when the display is smaller.

### Notes

- Known `401` skip behavior is now configurable instead of being hard-wired into scan handling.
- Scan result storage is more resilient when the same account record is encountered multiple times during one run.
- The app no longer relies on whole-window scale transforms for primary layout behavior.
- Smaller screens now prioritize vertical scrolling and readable panel density over fitting the entire dashboard into a single static viewport.
- Startup window sizing uses the operating system work area when available, so the first window should open closer to the best usable size on both Windows and macOS.

## v1.2.0

### Highlights

- Added an in-app scheduler that can trigger recurring `Scan` or `Maintain` runs while the desktop app is open.
- Added `Full` and `Incremental` scan modes with configurable incremental batch size.
- Reduced large-pool setup pressure by merging connection validation and inventory sync into a single remote fetch during **Test & Save**.
- Extended the settings UI with scheduler mode, cron expression, next-run status, last-run result details, and advanced parameter help popovers.
- Adjusted task completion refresh handling so manual and scheduled runs refresh the UI once instead of duplicating large-pool reloads.

### Notes

- Scheduled tasks use local system time and standard 5-field cron expressions.
- The scheduler does not replay missed runs after the app restarts.
- Incremental scans prioritize `Pending` accounts first, then the oldest last-probed records.
- The default retry count is now `3`.

## v1.1.0

### Highlights

- Added inventory-first startup for large pools, so first-time connections can sync tracked auth records before the first full scan.
- Moved the account table and scan details to backend pagination to reduce frontend pressure on pools with thousands of auth files.
- Stabilized dashboard startup and donut rendering to address blank first-load states and improve large-pool reliability.
- Improved retry handling and retry visibility for transient probe failures.

### Notes

- Existing local settings and state are preserved across upgrades.
- macOS users may need to right-click the app and choose `Open` on first launch.
- This release focuses on large-pool startup, inventory sync, dashboard stability, and paged data loading.
