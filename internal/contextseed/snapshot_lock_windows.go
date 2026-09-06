//go:build windows

package contextseed

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockSnapshotFileShared(f *os.File) error {
	var ol windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), 0, 0, 1, 0, &ol)
}

func lockSnapshotFileExclusive(f *os.File) error {
	var ol windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol)
}

func tryLockSnapshotFileExclusive(f *os.File) (bool, error) {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err == nil {
		return true, nil
	}
	if err == windows.ERROR_LOCK_VIOLATION {
		return false, nil
	}
	return false, err
}

func unlockSnapshotFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
