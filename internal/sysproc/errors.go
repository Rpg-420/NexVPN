package sysproc

import "errors"

// ErrMTUUnsupported is returned when an MTU change is requested on a platform
// whose tunnel adapter cannot be configured through this package.
//
// It is declared here, deliberately without a build constraint, because callers
// (for example app_extra.go) compare against it with errors.Is on every
// platform. Declaring it in sysproc_other.go instead made the symbol disappear
// from the Windows build, where the file is excluded by its //go:build tag.
var ErrMTUUnsupported = errors.New("MTU configuration is only supported on Windows (Wintun)")
