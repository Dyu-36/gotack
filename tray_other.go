//go:build !windows

package main

// startTray is a no-op off Windows: the release targets Windows only and the
// notification-area icon is a shell feature. The Linux CI job still compiles
// the package-main surface through this stub.
func startTray(*App) {}
