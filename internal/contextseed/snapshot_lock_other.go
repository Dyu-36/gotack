//go:build !windows

package contextseed

import (
	"errors"
	"os"
	"syscall"
)

func tryLockSnapshotFileShared(f *os.File) (bool, error) {
	return tryLockSnapshotFile(f, syscall.LOCK_SH|syscall.LOCK_NB)
}

func tryLockSnapshotFileExclusive(f *os.File) (bool, error) {
	return tryLockSnapshotFile(f, syscall.LOCK_EX|syscall.LOCK_NB)
}

func tryLockSnapshotFile(f *os.File, how int) (bool, error) {
	err := syscall.Flock(int(f.Fd()), how)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
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
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
