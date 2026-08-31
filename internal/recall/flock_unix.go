//go:build !windows

package recall

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// tryLockFile attempts a non-blocking exclusive advisory flock on f,
// mirroring Crush's internal/lock/lock_unix.go. The returned function drops
// the lock.
func tryLockFile(f *os.File) (func(), error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errLockContended
		}
		return nil, fmt.Errorf("flock: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}, nil
}
