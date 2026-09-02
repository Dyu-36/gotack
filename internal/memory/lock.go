package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errLockContended = errors.New("memory lock is held by another process")

func acquireFileLock(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = tryLockFile(file)
		if err == nil {
			return func() {
				_ = unlockFile(file)
				_ = file.Close()
			}, nil
		}
		if !errors.Is(err, errLockContended) {
			_ = file.Close()
			return nil, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			_ = file.Close()
			return nil, fmt.Errorf("wait for lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
