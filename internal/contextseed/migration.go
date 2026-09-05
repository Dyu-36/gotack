package contextseed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/bundleseed"
)

const (
	managedCoreName      = "TACK_CORE.md"
	userContextName      = "USER.md"
	legacyContextName    = "TACK.md"
	stockManifestName    = "stock-manifest.json"
	migrationStateName   = "context-migration.json"
	contextBackupDirName = "context-backups"
)

type MigrationMode string

const (
	MigrationLayered MigrationMode = "layered"
	MigrationPending MigrationMode = "migration_pending"
	MigrationLegacy  MigrationMode = "legacy"
)

type MigrationStatus struct {
	Mode        MigrationMode `json:"mode"`
	Version     int           `json:"version"`
	LegacyHash  string        `json:"legacy_hash,omitempty"`
	BackupToken string        `json:"backup_token,omitempty"`
}

type MigrationPreview struct {
	Status      MigrationStatus `json:"status"`
	Legacy      string          `json:"legacy,omitempty"`
	ManagedCore string          `json:"managed_core,omitempty"`
	UserContext string          `json:"user_context,omitempty"`
}

type stockManifest struct {
	Version           int      `json:"version"`
	LegacyStockSHA256 []string `json:"legacy_stock_sha256"`
}

func (s *Seeder) migrationStatePath() string {
	return filepath.Join(s.dataDir, migrationStateName)
}

func (s *Seeder) backupDir() string {
	return filepath.Join(s.dataDir, contextBackupDirName)
}
func loadStockManifest(sourceDir string) (stockManifest, error) {
	path := filepath.Join(sourceDir, stockManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		return stockManifest{}, fmt.Errorf("read context stock manifest: %w", err)
	}
	var manifest stockManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return stockManifest{}, fmt.Errorf("parse context stock manifest: %w", err)
	}
	if manifest.Version < 1 || len(manifest.LegacyStockSHA256) == 0 {
		return stockManifest{}, errors.New("context stock manifest is empty")
	}
	return manifest, nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func containsHash(hashes []string, hash string) bool {
	for _, candidate := range hashes {
		if strings.EqualFold(candidate, hash) {
			return true
		}
	}
	return false
}
func (s *Seeder) loadMigrationStatus() (MigrationStatus, error) {
	data, err := os.ReadFile(s.migrationStatePath())
	if err != nil {
		if !os.IsNotExist(err) {
			return MigrationStatus{}, err
		}
		if _, statErr := os.Stat(filepath.Join(s.ContextDir(), legacyContextName)); statErr == nil {
			return MigrationStatus{Mode: MigrationLegacy, Version: 1}, nil
		}
		return MigrationStatus{Mode: MigrationLayered, Version: 1}, nil
	}
	var status MigrationStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return MigrationStatus{}, fmt.Errorf("parse context migration state: %w", err)
	}
	if status.Mode == "" {
		return MigrationStatus{}, errors.New("context migration state has no mode")
	}
	return status, nil
}

func (s *Seeder) saveMigrationStatus(status MigrationStatus) error {
	status.Version = 1
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return atomicWriteFile(s.migrationStatePath(), data, 0o600)
}

func (s *Seeder) MigrationStatus() MigrationStatus {
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationStatus{Mode: MigrationPending, Version: 1}
	}
	return status
}
func (s *Seeder) seedLayered(sourceDir string) error {
	manifest, err := loadStockManifest(sourceDir)
	if err != nil {
		return err
	}
	legacyPath := filepath.Join(s.ContextDir(), legacyContextName)
	status := MigrationStatus{Mode: MigrationLayered, Version: manifest.Version}
	if _, err := os.Stat(legacyPath); err == nil {
		hash, hashErr := fileSHA256(legacyPath)
		if hashErr != nil {
			return fmt.Errorf("hash legacy context: %w", hashErr)
		}
		status.LegacyHash = hash
		if containsHash(manifest.LegacyStockSHA256, hash) {
			token, backupErr := s.backupLegacy(legacyPath)
			if backupErr != nil {
				return backupErr
			}
			status.BackupToken = token
			if err := os.Remove(legacyPath); err != nil {
				return fmt.Errorf("remove stock legacy context: %w", err)
			}
		} else {
			status.Mode = MigrationPending
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat legacy context: %w", err)
	}

	options := bundleseed.Options{
		ExistingFiles: bundleseed.UserEditableFiles,
		OnPreserve:    s.logPreserved,
	}
	if err := bundleseed.CopyIfChanged(sourceDir, s.ContextDir(), options); err != nil {
		return fmt.Errorf("copy layered context tree: %w", err)
	}
	core, err := os.ReadFile(filepath.Join(sourceDir, managedCoreName))
	if err != nil {
		return fmt.Errorf("read managed context core: %w", err)
	}
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), managedCoreName), core, 0o644); err != nil {
		return fmt.Errorf("update managed context core: %w", err)
	}
	if err := s.saveMigrationStatus(status); err != nil {
		return fmt.Errorf("save context migration state: %w", err)
	}
	return nil
}

func (s *Seeder) backupLegacy(legacyPath string) (string, error) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return "", fmt.Errorf("read legacy context for backup: %w", err)
	}
	digest := sha256.Sum256(data)
	token := fmt.Sprintf("legacy-%d-%s.md", time.Now().UnixNano(), hex.EncodeToString(digest[:4]))
	path := filepath.Join(s.backupDir(), token)
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("backup legacy context: %w", err)
	}
	return token, nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".context-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func (s *Seeder) PreviewMigration() (MigrationPreview, error) {
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationPreview{}, err
	}
	preview := MigrationPreview{Status: status}
	preview.Legacy = readOptionalText(filepath.Join(s.ContextDir(), legacyContextName))
	preview.ManagedCore = readOptionalText(filepath.Join(s.ContextDir(), managedCoreName))
	preview.UserContext = readOptionalText(filepath.Join(s.ContextDir(), userContextName))
	return preview, nil
}

func readOptionalText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// AcceptMigration commits user-reviewed USER.md content and retires legacy
// TACK.md. No heuristic extraction is performed.
func (s *Seeder) AcceptMigration(resolvedUser string) (MigrationStatus, error) {
	legacyPath := filepath.Join(s.ContextDir(), legacyContextName)
	if _, err := os.Stat(legacyPath); err != nil {
		return MigrationStatus{}, fmt.Errorf("legacy context unavailable: %w", err)
	}
	hash, err := fileSHA256(legacyPath)
	if err != nil {
		return MigrationStatus{}, err
	}
	token, err := s.backupLegacy(legacyPath)
	if err != nil {
		return MigrationStatus{}, err
	}
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), userContextName), []byte(resolvedUser), 0o644); err != nil {
		return MigrationStatus{}, fmt.Errorf("write resolved user context: %w", err)
	}
	if err := os.Remove(legacyPath); err != nil {
		return MigrationStatus{}, fmt.Errorf("retire legacy context: %w", err)
	}
	status := MigrationStatus{Mode: MigrationLayered, Version: 1, LegacyHash: hash, BackupToken: token}
	if err := s.saveMigrationStatus(status); err != nil {
		return MigrationStatus{}, err
	}
	return status, nil
}

func (s *Seeder) RollbackMigration(token string) (MigrationStatus, error) {
	if token == "" || filepath.Base(token) != token || strings.ContainsAny(token, `/\\`) {
		return MigrationStatus{}, errors.New("invalid context rollback token")
	}
	backupPath := filepath.Join(s.backupDir(), token)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("read context rollback backup: %w", err)
	}
	legacyPath := filepath.Join(s.ContextDir(), legacyContextName)
	if err := atomicWriteFile(legacyPath, data, 0o644); err != nil {
		return MigrationStatus{}, fmt.Errorf("restore legacy context: %w", err)
	}
	hash := sha256.Sum256(data)
	status := MigrationStatus{Mode: MigrationLegacy, Version: 1, LegacyHash: hex.EncodeToString(hash[:]), BackupToken: token}
	if err := s.saveMigrationStatus(status); err != nil {
		return MigrationStatus{}, err
	}
	return status, nil
}
