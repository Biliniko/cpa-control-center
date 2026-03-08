# Changelog

All notable changes to this project will be documented in this file.

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
