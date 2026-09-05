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
)

const (
	managedCoreName      = "TACK_CORE.md"
	userContextName      = "USER.md"
	legacyContextName    = "TACK.md"
	stockManifestName    = "stock-manifest.json"
	migrationStateName   = "context-migration.json"
	contextBackupDirName = "context-backups"
	contextStageDirName  = "context-migration-stage"
)

type MigrationMode string

const (
	MigrationLegacy     MigrationMode = "legacy"
	MigrationPending    MigrationMode = "pending"
	MigrationStaged     MigrationMode = "staged"
	MigrationCommitted  MigrationMode = "committed-layered"
	MigrationRolledBack MigrationMode = "rolled-back"
	MigrationLayered                  = MigrationCommitted
)

type MigrationStatus struct {
	Mode        MigrationMode   `json:"mode"`
	Version     int             `json:"version"`
	Generation  uint64          `json:"generation"`
	LegacyHash  string          `json:"legacy_hash,omitempty"`
	UserHash    string          `json:"user_hash,omitempty"`
	CoreHash    string          `json:"core_hash,omitempty"`
	BaseHash    string          `json:"base_hash,omitempty"`
	BackupToken string          `json:"backup_token,omitempty"`
	UpdatedAt   int64           `json:"updated_at"`
	Stage       *migrationStage `json:"stage,omitempty"`
}

type migrationStage struct {
	Token              string        `json:"token"`
	PreviousMode       MigrationMode `json:"previous_mode"`
	ExpectedLegacyHash string        `json:"expected_legacy_hash"`
	ExpectedUserHash   string        `json:"expected_user_hash,omitempty"`
	UserExisted        bool          `json:"user_existed"`
	TargetCoreHash     string        `json:"target_core_hash"`
	TargetUserHash     string        `json:"target_user_hash"`
}

type MigrationPreview struct {
	Status             MigrationStatus `json:"status"`
	Legacy             string          `json:"legacy,omitempty"`
	KnownBase          string          `json:"known_base,omitempty"`
	ManagedCore        string          `json:"managed_core,omitempty"`
	UserContext        string          `json:"user_context,omitempty"`
	CandidateUser      string          `json:"candidate_user,omitempty"`
	BaseKnown          bool            `json:"base_known"`
	HasConflicts       bool            `json:"has_conflicts"`
	RequiresResolution bool            `json:"requires_resolution"`
}

type AcceptMigrationRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
	ExpectedLegacyHash string `json:"expected_legacy_hash"`
	ExpectedUserHash   string `json:"expected_user_hash,omitempty"`
	ExpectedCoreHash   string `json:"expected_core_hash"`
	ResolvedUser       string `json:"resolved_user"`
}

type RollbackMigrationRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
	Token              string `json:"token"`
}

type stockManifest struct {
	Version           int           `json:"version"`
	LegacyStockSHA256 []string      `json:"legacy_stock_sha256,omitempty"`
	LegacyStocks      []legacyStock `json:"legacy_stocks,omitempty"`
}
type legacyStock struct {
	SHA256 string `json:"sha256"`
	Path   string `json:"path"`
}
type backupMetadata struct {
	LegacyHash  string `json:"legacy_hash"`
	UserHash    string `json:"user_hash,omitempty"`
	UserExisted bool   `json:"user_existed"`
}

var ErrMigrationConflict = errors.New("context migration changed since preview")

func (s *Seeder) migrationStatePath() string { return filepath.Join(s.dataDir, migrationStateName) }
func (s *Seeder) backupDir() string          { return filepath.Join(s.dataDir, contextBackupDirName) }
func (s *Seeder) stageDir(token string) string {
	return filepath.Join(s.dataDir, contextStageDirName, token)
}

func loadStockManifest(sourceDir string) (stockManifest, error) {
	data, err := os.ReadFile(filepath.Join(sourceDir, stockManifestName))
	if err != nil {
		return stockManifest{}, fmt.Errorf("read context stock manifest: %w", err)
	}
	var manifest stockManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return stockManifest{}, fmt.Errorf("parse context stock manifest: %w", err)
	}
	if manifest.Version < 1 || len(manifest.LegacyStockSHA256)+len(manifest.LegacyStocks) == 0 {
		return stockManifest{}, errors.New("context stock manifest is empty")
	}
	for _, stock := range manifest.LegacyStocks {
		if len(stock.SHA256) != sha256.Size*2 || stock.Path == "" || filepath.IsAbs(stock.Path) || filepath.Clean(stock.Path) != stock.Path || strings.HasPrefix(stock.Path, "..") {
			return stockManifest{}, errors.New("context stock manifest contains an invalid legacy base")
		}
		hash, hashErr := fileSHA256(filepath.Join(sourceDir, stock.Path))
		if hashErr != nil || !strings.EqualFold(hash, stock.SHA256) {
			return stockManifest{}, fmt.Errorf("context legacy base %q does not match manifest hash", stock.Path)
		}
	}
	return manifest, nil
}
func (m stockManifest) contains(hash string) bool {
	for _, candidate := range m.LegacyStockSHA256 {
		if strings.EqualFold(candidate, hash) {
			return true
		}
	}
	for _, candidate := range m.LegacyStocks {
		if strings.EqualFold(candidate.SHA256, hash) {
			return true
		}
	}
	return false
}
func (m stockManifest) base(sourceDir, hash string) ([]byte, bool) {
	for _, candidate := range m.LegacyStocks {
		if strings.EqualFold(candidate.SHA256, hash) {
			data, err := os.ReadFile(filepath.Join(sourceDir, candidate.Path))
			return data, err == nil
		}
	}
	return nil, false
}
func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return bytesSHA256(data), nil
}
func bytesSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func optionalFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
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
		return MigrationStatus{Mode: MigrationCommitted, Version: 1}, nil
	}
	var status MigrationStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return MigrationStatus{}, fmt.Errorf("parse context migration state: %w", err)
	}
	switch status.Mode {
	case MigrationLegacy, MigrationPending, MigrationStaged, MigrationCommitted, MigrationRolledBack:
	default:
		return MigrationStatus{}, fmt.Errorf("context migration state has invalid mode %q", status.Mode)
	}
	if status.Mode == MigrationStaged && status.Stage == nil {
		return MigrationStatus{}, errors.New("staged context migration has no transaction")
	}
	return status, nil
}
func (s *Seeder) saveMigrationStatus(status MigrationStatus) error {
	if status.Version == 0 {
		status.Version = 1
	}
	status.UpdatedAt = time.Now().UnixMilli()
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return atomicWriteFile(s.migrationStatePath(), data, 0o600)
}
func (s *Seeder) nextStatus(previous, next MigrationStatus) MigrationStatus {
	generation := previous.Generation
	next.Generation = 0
	previous.Generation, previous.UpdatedAt = 0, 0
	next.UpdatedAt = 0
	left, _ := json.Marshal(previous)
	right, _ := json.Marshal(next)
	if string(left) != string(right) {
		generation++
	}
	if generation == 0 {
		generation = 1
	}
	next.Generation = generation
	return next
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
func safeToken(token string) bool {
	return token != "" && filepath.Base(token) == token && !strings.ContainsAny(token, `/\\`)
}
