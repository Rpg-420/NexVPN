# Optional follow-ups (November 2026 audit)

Everything the audit classified as a real defect is fixed, verified and shipped
(see `CHANGELOG_DEV.md` and `docs/reviews/2026-11-full-repo-review.md`). The
items below are deliberate non-blockers: each one is either a UX consistency
refactor, a platform upgrade with a working fallback in place, or a design
decision that needs a product call.

## 1. Frontend hook adoption is partial (consistency, not correctness)
`useProfileList` and `ReliabilityBadge` are wired into V2RayPage and
WarpWireGuardPage; `useToast` into DiscoveryHub, V2RayPage and WarpWireGuardPage.
OpenVPNPage and AetherPage still carry their own copies of `loadProfiles` and
their own toast timer, and DashboardPage uses `useTimers` directly.

Why it was left: those two pages work, and rewriting two large components in an
unattended pass is exactly the kind of change that silently regresses a UI. Any
one of them can be migrated in isolation with `npx tsc --noEmit && npm test`.

## 2. Logs page has two update sources
`Logs.tsx` merges a 3-second `getLogs()` poll with the push listener, so an entry
delivered between the poll request and its response can be reordered or briefly
missing, and autoscroll keeps fighting a user who has scrolled up to read.

Fix shape: subscribe to events only, and use the poll purely as reconciliation
(merge by timestamp), plus "pause autoscroll while the user is scrolled up".

## 3. Settings has no unsaved-changes indicator
Settings auto-saves on a 500 ms debounce *and* offers a Save button, so it is
impossible to tell whether pressing Save does anything.

## 4. Sub-project: replace `netsh` with the Native Wifi API
`app_watchdog.go` shells out to `netsh wlan show interfaces` every few seconds
(now under `chcp 65001`, so Persian SSIDs are correct). `wlanapi.dll` would give
UTF-16 SSIDs and event-driven change notification instead of polling, removing a
process spawn per tick. Working as intended today — this is an optimisation.

## 5. Backup format v2 (passphrase-encrypted bundle)
The backup/diagnostics exports no longer *leak* secrets (credentials are
stripped), and a passphrase-encrypted backup already exists as a separate
feature. What is missing is a single encrypted bundle that still restores
URI-based profiles. The AES-GCM helper already lives in `internal/secret`.

## 6. Wails frontend bindings are stale for three new methods
`GetProfileReliabilityScores`, `GetProtocolRadarHistory` and `CancelProtocolRadar`
are callable at runtime — `api.ts` resolves them dynamically from
`window.go.main.App[...]` and the Go side dispatches them — but the generated
TypeScript wrappers in `frontend/wailsjs/` do not list them yet, because
`wails generate module` builds and runs a helper that inherits the app's
"requireAdministrator" manifest, so it must be run from an elevated shell:

```powershell
# elevated PowerShell, once
cd D:\App1; wails generate module
```

## 7. Dependabot PRs are open and unmerged
Eleven PRs (actions, npm, go modules) are queued. They were left for review on
purpose: after the CI repairs in this branch, the lint job pins golangci-lint
v2.15.0 and the CI is green, so a major action bump should be merged one at a
time with a green run to confirm.
