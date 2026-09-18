package contextseed

import (
	"errors"
	"time"
)

const (
	snapshotLockWaitTimeout = 30 * time.Second
	snapshotLockRetryStart  = 2 * time.Millisecond
	snapshotLockRetryMax    = 25 * time.Millisecond
)

var errSnapshotLockTimeout = errors.New("snapshot lock acquisition timed out")

func lockSnapshotFileBounded(try func() (bool, error)) error {
	deadline := time.Now().Add(snapshotLockWaitTimeout)
	backoff := snapshotLockRetryStart
	for {
		acquired, err := try()
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errSnapshotLockTimeout
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > snapshotLockRetryMax {
			backoff = snapshotLockRetryMax
		}
	}
}
