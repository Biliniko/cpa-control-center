[English](./README.md) | [简体中文](./README.zh-CN.md)

# CPA Control Center

Desktop operations tool for CPA-managed Codex auth pools.

`CPA Control Center` packages the existing CPA management endpoints into a focused desktop app. It is designed for operators who do not want to manage auth pools through browser tabs, localhost pages, or terminal scripts once the pool grows large.

You connect it to a CPA instance with a `Base URL` and a `Management Token`, then handle inventory sync, scanning, maintenance, scheduler runs, logs, history, and exports from one native window.

## Acknowledgement and Intended Backend

- This project is explicitly inspired by and borrows workflow ideas from [`fantasticjoe/cpa-warden`](https://github.com/fantasticjoe/cpa-warden).
- This desktop tool is intended to be used with [`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI) as the CPA backend exposing the management endpoints consumed by the app.

## Overview

- Native desktop app built with Wails, Go, Vue 3, and TypeScript
- Only requires `Base URL` and `Management Token`
- Inventory-first startup flow for large pools
- Dedicated Codex quota workspace with plan overview, account matrix, recovery board, and detail inspection
- Server-paged accounts and scan details
- Unified state model: `Pending`, `Normal`, `401 Invalid`, `Quota Limited`, `Recovered`, `Error`
- Scan modes: `Full` and `Incremental`
- Built-in in-app scheduler for automatic `Scan` or `Maintain` tasks
- Separate in-app quota auto-refresh cron for recurring Codex quota snapshots while the app stays open
- Live task logs and scan history
- CSV / JSON export for `401 Invalid` and `Quota Limited`
- Built-in bilingual interface: English and Simplified Chinese

## Who This Is For

This project is a good fit if:

- you already deployed CPA and enabled management endpoints
- you maintain a Codex-focused auth pool
- you want a desktop app that works immediately without extra deployment
- you want scan, maintenance, logs, and exports in one place

This project is not currently focused on:

- creating or importing auth files from the GUI
- running OAuth login flows inside the desktop app
- becoming a full general-purpose CPA admin panel

## What Problem It Solves

Large auth pools usually do not fail all at once. They drift into mixed operational states:

- some accounts are already `401 Invalid`
- some accounts hit quota limits
- some accounts are temporarily probe-failed but recover on retry
- some accounts were disabled earlier and are now recoverable
- some accounts are known locally but have not been probed yet

The app is built around two goals:

1. Give you a fast and reliable picture of pool health.
2. Let you apply repeatable maintenance rules on top of the latest scan result.

## Current Workflow

The current product flow is intentionally split into stages:

1. Save CPA connection settings.
2. Sync inventory from CPA into the local snapshot.
3. Show the pool as tracked inventory even before the first scan.
4. Run a scan when you want health states to be established.
5. Run maintenance only after reviewing the latest scan result.
6. Optionally enable the in-app scheduler for recurring scan or maintenance runs while the app is open.

This matters for large pools. A first connection no longer needs to immediately probe thousands of accounts just to make the UI usable.

## First Connection Behavior

After **Test & Save** succeeds, the app now performs a single remote inventory fetch that both validates the CPA connection and syncs the local inventory snapshot.

That means:

- the app no longer fetches the full auth list twice during first-time setup
- newly seen records are marked as `Pending`
- the dashboard can show tracked counts immediately
- the account table is available immediately
- health classification is still incomplete until the first scan finishes

## State Model

The app uses a compact state model across dashboard, list views, maintenance, and export:

- `Pending`: inventory synced locally, not probed yet
- `Normal`
- `401 Invalid`
- `Quota Limited`
- `Recovered`
- `Error`

`Pending` is especially important for large pools and first-time setup. It distinguishes “known inventory, not yet scanned” from actual probe outcomes.

## Core Capabilities

### 1. Connect to CPA

You only need:

- `Base URL`
- `Management Token`

The connection test itself does not trigger probing, but **Test & Save** now also syncs the inventory snapshot so the UI becomes usable immediately.

### 2. Sync Inventory

Inventory sync:

- fetches auth metadata from CPA
- persists the local tracked pool
- prepares the app for large-pool usage
- avoids a blank dashboard on first use

### 3. Scan the Pool

When you click **Scan Now**, the app:

1. loads the latest auth inventory from CPA
2. applies `targetType` and `provider` filters
3. probes matching accounts concurrently
4. writes the latest snapshot
5. updates the latest Codex quota snapshot for the probed accounts without issuing a separate quota refresh
6. records a scan-history entry

Two scan strategies are available:

- `Full`: probe the entire filtered pool
- `Incremental`: only probe one batch at a time, prioritizing `Pending` accounts first and then the oldest last-probed records

`Incremental Batch Size` controls how many accounts a single incremental run will probe.

### 4. Run Maintenance

When you click **Run Maintain**, the app first performs a fresh scan and then applies the configured rules:

- delete `401 Invalid` accounts
- `disable` or `delete` quota-limited accounts
- re-enable recovered accounts

All destructive operations require confirmation first.

Maintenance always uses a fresh full scan. It does not use incremental batching.

### 5. Review History and Logs

The app keeps:

- the current account snapshot
- recent scan history
- paginated scan details
- live task logs and progress events

If **Detailed Logs** is enabled, the task log also shows per-account scan and maintenance details.

### 6. Inspect Codex Quotas

The quota workspace is no longer limited to pooled plan cards. It now includes:

- plan-level overview cards for pooled quota health
- an account matrix for successful and failed quota snapshots
- a recovery board that can group accounts by earliest, `5h`, or weekly reset windows
- an account detail panel for inspecting bucket-level state and failure reasons

Quota snapshots can be updated in four ways:

- automatically after scans
- automatically after maintenance runs
- manually from the quota page
- through the dedicated in-app quota auto-refresh cron while the app remains open

### 7. Schedule Automatic Tasks

The app includes one built-in scheduler:

- one global schedule per app instance
- action mode can be `Scan` or `Maintain`
- uses standard 5-field cron expressions in local system time
- reloads immediately after you save settings
- skips the run if another scan or maintenance task is already active

Important scope limits:

- scheduled tasks only run while the app is open
- missed runs are not replayed after restart
- the first version does not support multiple schedules or OS-level task integration

### 8. Export Problem Sets

You can export:

- current `401 Invalid` accounts
- current `Quota Limited` accounts

Formats:

- JSON
- CSV

## Page Structure

### Dashboard

- pool health overview
- recent scan history
- paginated scan detail drawer
- one-click scan and maintenance actions

### Codex Quotas

- plan overview cards grouped by quota plan
- account matrix with result filters, quota sorting, and row-based pagination
- recovery board with selectable reset buckets (`Earliest`, `5h`, `Weekly`)
- account detail drawer for bucket status, reset timing, and failure reasons
- manual refresh plus optional in-app quota auto-refresh cron

### Accounts

- server-paged account table
- full-dataset search handled on the backend
- state and provider filters
- single-account probe, disable/enable, and delete actions

### Logs

- live task stream
- current progress is always visible
- optional detailed per-account logs

### Settings

- CPA connection parameters
- language switching
- scan strategy and incremental batch size
- concurrency and timeout settings
- retry count
- quota-handling strategy
- in-app scheduler enable/mode/cron
- export directory
- detailed-log toggle
- inline info popovers for advanced parameters

## Large Pool Notes

The current implementation is designed to behave better on pools with thousands of auth files:

- first connection performs inventory sync instead of immediate full probing
- **Test & Save** validates the connection and syncs inventory in one remote fetch
- dashboard no longer ships the full account list to the frontend
- accounts use backend pagination
- scan details use backend pagination
- the app keeps `Pending` states for synced-but-unscanned records
- `Incremental` scan mode can spread probing across multiple runs instead of hitting the entire pool at once

This is not the final performance ceiling, but it is already a large step up from the earlier full-snapshot frontend model.

## CPA Endpoints Used

The app intentionally stays focused and only depends on a small set of CPA management endpoints:

- `GET /v0/management/auth-files`
- `POST /v0/management/api-call`
- `DELETE /v0/management/auth-files?name=...`
- `PATCH /v0/management/auth-files/status`

Pool health probing goes through CPA and targets:

- `https://chatgpt.com/backend-api/wham/usage`

## Default Behavior

| Setting | Default |
| --- | --- |
| Locale | normalized system locale (`en-US` / `zh-CN`) |
| Target type | `codex` |
| Scan strategy | `full` |
| Incremental batch size | `1000` |
| Probe workers | `40` |
| Action workers | `20` |
| Timeout | `15s` |
| Retries | `3` |
| Quota action | `disable` |
| Delete 401 | enabled |
| Auto re-enable recovered accounts | enabled |
| Scheduler | disabled |
| Detailed logs | disabled |

## Retry Model

Retries are split into two layers:

- request-level retries for outer request failures and transient CPA errors such as `408`, `429`, and `5xx`
- probe-level retries for recoverable probe anomalies such as temporary upstream failures, invalid payloads, or retryable upstream status codes

The app does not blindly retry final business outcomes such as:

- `401 Invalid`
- `Quota Limited`
- clearly missing account metadata

## Local Data Storage

The app stores local state under your OS user configuration directory in:

`CPA Control Center/`

Typical contents:

- `settings.json`
- `state.db`
- `app.log`
- `exports/`

The current implementation keeps the latest snapshot and the most recent `30` scan runs.

## Project Structure

```text
cpa-control-center/
|- frontend/                     # Vue 3 + TypeScript frontend
|- internal/backend/             # CPA client, state store, task orchestration
|- build/                        # Wails build assets and platform packaging config
|- scripts/build-macos.sh        # macOS build helper
|- .github/workflows/            # CI / Release workflows
|- app.go                        # Wails binding layer
|- main.go                       # shared entry point
|- platform_options_windows.go   # Windows window configuration
|- platform_options_darwin.go    # macOS window configuration
`- wails.json                    # Wails project configuration
```

## Development and Build

### Requirements

- Go `1.24+`
- Node.js `22+` recommended
- Wails CLI `v2.11.0`

### Development Mode

```bash
wails dev
```

### Windows Build

```bash
wails build
```

### macOS Build

Run this on a Mac or a `macos-latest` GitHub Actions runner:

```bash
bash ./scripts/build-macos.sh
```

## Safety Notes

- This is an operations tool, not a demo page. Maintenance actions can delete or disable accounts.
- Review the latest scan result before running maintenance.
- If you are validating a new CPA environment, consider turning off `delete401` first.
- Detailed logs are useful for troubleshooting, but they can become noisy on large pools.
- On very large pools, prefer `Incremental` scan mode unless you intentionally want a full sweep.

## Current Scope

This project currently focuses on:

- managing existing CPA auth files
- health probing and maintenance for Codex pools
- a Windows-first desktop experience while also providing a macOS build path

It does not currently include:

- auth import wizards
- in-app login / OAuth acquisition
- multi-node orchestration
- advanced analytics beyond local snapshots and history views

## FAQ

### Does it open a browser?

No. It is a Wails desktop application and opens in its own native window.

### Is it a full CPA admin panel?

No. It is a focused desktop operations tool for auth-pool health and maintenance.

### Why do I see tracked accounts before the first scan?

Because the app syncs inventory first. It can show tracked records immediately, while health states remain `Pending` until probing completes.

### What is the difference between `Full` and `Incremental` scan?

`Full` scans the entire filtered pool. `Incremental` scans only one batch at a time, prioritizing unprobed accounts and then the oldest records. Incremental mode is useful for very large pools.

### Does “Run Maintain” scan first?

Yes. Maintenance always starts from a fresh scan and then applies the maintenance rules.

### Can scan details handle large result sets?

Yes. Scan details are paginated on the backend and are not loaded into the drawer all at once.

### Is there a debug mode?

Yes, but it is hidden. Press `Ctrl + Shift + D` to open the internal debug panel used for startup and dashboard troubleshooting.

### Do scheduled tasks run while the app is closed?

No. The built-in scheduler only runs while the desktop app is open. Missed runs are not replayed after restart.

### Is macOS supported?

The build path is already in place. Production macOS artifacts should be generated on a Mac or a `macos-latest` GitHub Actions runner.

## Roadmap

- configurable threshold-based rules, for example disabling accounts once they fall below a defined percentage
- support more auth-channel maintenance
- add richer statistics and trend views
- add signing / notarization workflows when external distribution requires it

## Current Status

The app is already usable for real CPA pool operations, but it is still an evolving practical tool. The priority remains reliability, clarity, and maintainability over feature sprawl.
