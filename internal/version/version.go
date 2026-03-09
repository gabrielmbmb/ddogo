// Package version holds the build-time version string for ddogo.
//
//revive:disable-next-line:var-naming
package version

// Version is set at build time via -ldflags; defaults to "dev".
var Version = "dev"
