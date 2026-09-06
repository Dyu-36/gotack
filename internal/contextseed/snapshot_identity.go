package contextseed

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// loadOrCreateSnapshotIdentityKey publishes only a fully written key and never
// replaces another publisher's identity. Seeder.mu cannot protect other Seeder
// instances or processes sharing the same profile.
func loadOrCreateSnapshotIdentityKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return parseSnapshotIdentityKey(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("read identity key")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("generate identity key")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".snapshot-key-*")
	if err != nil {
		return nil, errors.New("stage identity key")
	}
	defer os.Remove(file.Name())
	if _, err := file.Write([]byte(hex.EncodeToString(key))); err != nil {
		_ = file.Close()
		return nil, errors.New("write identity key")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, errors.New("sync identity key")
	}
	if err := file.Close(); err != nil {
		return nil, errors.New("close identity key")
	}
	// Same-directory hard linking is an atomic, no-replace publication on
	// the supported NTFS path. Unsupported filesystems fail closed.
	if err := os.Link(file.Name(), path); err != nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, errors.New("publish identity key")
		}
		return parseSnapshotIdentityKey(data)
	}
	return key, nil
}

func parseSnapshotIdentityKey(data []byte) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid identity key")
	}
	return key, nil
}
