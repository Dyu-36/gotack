//go:build windows

package contextseed

import (
	"os"

	"golang.org/x/sys/windows"
)

func tryLockSnapshotFileShared(f *os.File) (bool, error) {
	return tryLockSnapshotFile(f, windows.LOCKFILE_FAIL_IMMEDIATELY)
}

func tryLockSnapshotFileExclusive(f *os.File) (bool, error) {
	return tryLockSnapshotFile(f, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY)
}

func tryLockSnapshotFile(f *os.File, flags uint32) (bool, error) {
	var ol windows.Overlapped
	err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &ol)
	if err == nil {
		return true, nil
	}
	if err == windows.ERROR_LOCK_VIOLATION {
		return false, nil
	}
	return false, err
}

func lockSnapshotFileShared(f *os.File) error {
	return lockSnapshotFileBounded(func() (bool, error) { return tryLockSnapshotFileShared(f) })
}

func lockSnapshotFileExclusive(f *os.File) error {
	return lockSnapshotFileBounded(func() (bool, error) { return tryLockSnapshotFileExclusive(f) })
}

func unlockSnapshotFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
