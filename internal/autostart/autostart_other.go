//go:build !windows

package autostart

import "errors"

// Enabled always reports false on non-Windows hosts; the release targets
// Windows only and the login-launch entry is a registry feature.
func Enabled() bool { return false }

// SetEnabled is rejected on non-Windows hosts.
func SetEnabled(bool) error { return errors.New("autostart: only supported on Windows") }
