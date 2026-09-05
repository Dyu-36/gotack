//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	// valueName is the registry value under runKeyPath owned by Tack. One
	// value per product keeps coexistence with other apps in the shared
	// Run key unambiguous.
	valueName = "Tack"

	hiddenFlag = "--hidden"
)

// commandLine is the launch command the Run entry stores: the current
// executable, quoted for paths with spaces, followed by the hidden-start flag.
func commandLine(exe string) string {
	return fmt.Sprintf(`"%s" %s`, exe, hiddenFlag)
}

// Enabled reports whether Tack currently has a launch-at-login entry.
func Enabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(valueName)
	return err == nil && value != ""
}

// SetEnabled creates or removes the launch-at-login entry. Enabling stores the
// resolved path of the running executable, so a relocated install refreshes
// the entry on the next toggle rather than keeping a stale path.
func SetEnabled(on bool) error {
	if on {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("autostart: resolve executable: %w", err)
		}
		// A fresh Windows profile is not required to already contain the Run
		// key. CreateKey opens it when present and creates it when absent.
		key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("autostart: open or create run key: %w", err)
		}
		defer key.Close()
		if err := key.SetStringValue(valueName, commandLine(exe)); err != nil {
			return fmt.Errorf("autostart: write run entry: %w", err)
		}
		return nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		// Disabling is idempotent. A missing shared Run key already means the
		// Tack value is absent and must not be treated as a failure.
		return nil
	}
	if err != nil {
		return fmt.Errorf("autostart: open run key: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("autostart: delete run entry: %w", err)
	}
	return nil
}
