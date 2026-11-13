package main

// AppVersion is the single source of truth for the application version.
// All version strings across the codebase must reference this constant.
const AppVersion = "1.4.2"

// NOTE: version ordering lives in compareVersions (app_insights.go); the
// update check (CheckForUpdates in app.go) uses it so older releases and
// pre-release tags no longer register as "update available".
