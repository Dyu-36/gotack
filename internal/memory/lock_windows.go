//go:build windows

package memory

import (
	"errors"
	"math"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

func tryLockFile(file *os.File) error {
	handle := windows.Handle(file.Fd())
	var overlapped windows.Overlapped
	err := windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		math.MaxUint32,
		math.MaxUint32,
		&overlapped,
	)
	runtime.KeepAlive(&overlapped)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errLockContended
	}
	return err
}

func unlockFile(file *os.File) error {
	return windows.UnlockFileEx(
		windows.Handle(file.Fd()),
		0,
		math.MaxUint32,
		math.MaxUint32,
		new(windows.Overlapped),
	)
}

func replaceFile(from, to string) error {
	fromPtr, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	toPtr, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(fromPtr, toPtr, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
