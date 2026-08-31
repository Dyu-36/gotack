//go:build windows

package recall

import (
	"errors"
	"fmt"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

// tryLockFile attempts a non-blocking exclusive advisory lock on f, mirroring
// the LockFileEx call Crush makes in internal/lock/lock_windows.go. The
// returned function drops the lock.
func tryLockFile(f *os.File) (func(), error) {
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(h, flags, 0, math.MaxUint32, math.MaxUint32, ol); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, errLockContended
		}
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}
	return func() {
		unlock := new(windows.Overlapped)
		_ = windows.UnlockFileEx(h, 0, math.MaxUint32, math.MaxUint32, unlock)
	}, nil
}
