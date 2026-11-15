// Package buildinfo holds build-time metadata.
// Version is the default for local/dev builds; release builds override it
// with -ldflags "-X tui-workflow/internal/buildinfo.Version=x.y.z".
package buildinfo

// Version is the application version (semver without leading v).
var Version = "0.3.1"
