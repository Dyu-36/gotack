package contextseed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Seeder) MigrationStatus() MigrationStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationStatus{Mode: MigrationPending, Version: 1}
	}
	return status
}

func (s *Seeder) seedLayered(sourceDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceDir = sourceDir
	manifest, err := loadStockManifest(sourceDir)
	if err != nil {
		return err
	}
	core, err := os.ReadFile(filepath.Join(sourceDir, managedCoreName))
	if err != nil {
		return fmt.Errorf("read managed context core: %w", err)
	}
	defaultUser, err := os.ReadFile(filepath.Join(sourceDir, userContextName))
	if err != nil {
		return fmt.Errorf("read default user context: %w", err)
	}
	status, err := s.loadMigrationStatus()
	if err != nil {
		return err
	}
	if status.Mode == MigrationStaged {
		if status, err = s.recoverStage(status); err != nil {
			return err
		}
	}
	legacy, legacyExists, err := optionalFile(filepath.Join(s.ContextDir(), legacyContextName))
	if err != nil {
		return fmt.Errorf("read legacy context: %w", err)
	}
	user, userExists, err := optionalFile(filepath.Join(s.ContextDir(), userContextName))
	if err != nil {
		return fmt.Errorf("read user context: %w", err)
	}
	originalUser, originalUserExists := user, userExists
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), managedCoreName), core, 0o644); err != nil {
		return fmt.Errorf("update managed context core: %w", err)
	}
	if !userExists {
		if err := atomicWriteFile(filepath.Join(s.ContextDir(), userContextName), defaultUser, 0o644); err != nil {
			return fmt.Errorf("create user context: %w", err)
		}
		user, userExists = defaultUser, true
	}
	if status.Mode == MigrationRolledBack && legacyExists {
		next := status
		next.CoreHash, next.UserHash, next.LegacyHash = bytesSHA256(core), bytesSHA256(user), bytesSHA256(legacy)
		return s.saveMigrationStatus(s.nextStatus(status, next))
	}
	if !legacyExists {
		next := MigrationStatus{Mode: MigrationCommitted, Version: manifest.Version, CoreHash: bytesSHA256(core), UserHash: bytesSHA256(user), BackupToken: status.BackupToken, LegacyHash: status.LegacyHash}
		return s.saveMigrationStatus(s.nextStatus(status, next))
	}
	legacyHash := bytesSHA256(legacy)
	if manifest.contains(legacyHash) {
		return s.beginAndCommit(status, core, user, legacyHash, originalUser, originalUserExists)
	}
	baseHash := s.recordedLegacyBaseHash(manifest)
	next := MigrationStatus{Mode: MigrationPending, Version: manifest.Version, LegacyHash: legacyHash, UserHash: bytesSHA256(user), CoreHash: bytesSHA256(core), BaseHash: baseHash, BackupToken: status.BackupToken}
	return s.saveMigrationStatus(s.nextStatus(status, next))
}

func (s *Seeder) recordedLegacyBaseHash(manifest stockManifest) string {
	data, err := os.ReadFile(filepath.Join(s.ContextDir(), ".seed-report.json"))
	if err != nil {
		return ""
	}
	var report struct {
		Hashes map[string]string `json:"hashes"`
	}
	if json.Unmarshal(data, &report) != nil {
		return ""
	}
	hash := report.Hashes[legacyContextName]
	if manifest.contains(hash) {
		return strings.ToLower(hash)
	}
	return ""
}

func (s *Seeder) beginAndCommit(previous MigrationStatus, core, user []byte, expectedLegacy string, originalUser []byte, originalUserExists bool) error {
	token := fmt.Sprintf("migration-%d-%s", time.Now().UnixNano(), expectedLegacy[:8])
	stage := migrationStage{Token: token, PreviousMode: previous.Mode, ExpectedLegacyHash: expectedLegacy, UserExisted: originalUserExists, TargetCoreHash: bytesSHA256(core), TargetUserHash: bytesSHA256(user)}
	if originalUserExists {
		stage.ExpectedUserHash = bytesSHA256(originalUser)
	}
	stageDir := s.stageDir(token)
	if err := os.MkdirAll(stageDir, 0o700); err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(stageDir, managedCoreName), core, 0o644); err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(stageDir, userContextName), user, 0o644); err != nil {
		return err
	}
	staged := s.nextStatus(previous, MigrationStatus{Mode: MigrationStaged, Version: previous.Version, LegacyHash: expectedLegacy, UserHash: stage.ExpectedUserHash, CoreHash: stage.TargetCoreHash, Stage: &stage})
	if err := s.saveMigrationStatus(staged); err != nil {
		return err
	}
	_, recoverErr := s.recoverStage(staged)
	return recoverErr
}

func (s *Seeder) recoverStage(status MigrationStatus) (MigrationStatus, error) {
	stage := status.Stage
	if stage == nil || !safeToken(stage.Token) {
		return status, errors.New("invalid staged context migration")
	}
	stageDir := s.stageDir(stage.Token)
	core, coreExists, err := optionalFile(filepath.Join(stageDir, managedCoreName))
	if err != nil || !coreExists || bytesSHA256(core) != stage.TargetCoreHash {
		return status, errors.New("staged context core is missing or corrupt")
	}
	user, userExists, err := optionalFile(filepath.Join(stageDir, userContextName))
	if err != nil || !userExists || bytesSHA256(user) != stage.TargetUserHash {
		return status, errors.New("staged user context is missing or corrupt")
	}
	legacyPath := filepath.Join(s.ContextDir(), legacyContextName)
	backupPath := filepath.Join(s.backupDir(), stage.Token)
	backupLegacy := filepath.Join(backupPath, legacyContextName)
	legacy, legacyExists, err := optionalFile(legacyPath)
	if err != nil {
		return status, err
	}
	backed, backupExists, err := optionalFile(backupLegacy)
	if err != nil {
		return status, err
	}
	if legacyExists {
		if bytesSHA256(legacy) != stage.ExpectedLegacyHash {
			return status, ErrMigrationConflict
		}
		if stage.UserExisted {
			currentUser, exists, readErr := optionalFile(filepath.Join(s.ContextDir(), userContextName))
			if readErr != nil || !exists || bytesSHA256(currentUser) != stage.ExpectedUserHash {
				return status, ErrMigrationConflict
			}
		}
		if err := os.MkdirAll(backupPath, 0o700); err != nil {
			return status, err
		}
		meta, _ := json.Marshal(backupMetadata{LegacyHash: stage.ExpectedLegacyHash, UserHash: stage.ExpectedUserHash, UserExisted: stage.UserExisted})
		if err := atomicWriteFile(filepath.Join(backupPath, "backup.json"), meta, 0o600); err != nil {
			return status, err
		}
		if stage.UserExisted {
			currentUser, _, _ := optionalFile(filepath.Join(s.ContextDir(), userContextName))
			if err := atomicWriteFile(filepath.Join(backupPath, userContextName), currentUser, 0o600); err != nil {
				return status, err
			}
		}
		if err := os.Rename(legacyPath, backupLegacy); err != nil {
			return status, fmt.Errorf("retire legacy context: %w", err)
		}
		backed, backupExists = legacy, true
	}
	if !backupExists || bytesSHA256(backed) != stage.ExpectedLegacyHash {
		return status, errors.New("staged migration has no valid legacy backup")
	}
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), managedCoreName), core, 0o644); err != nil {
		return status, err
	}
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), userContextName), user, 0o644); err != nil {
		return status, err
	}
	committed := s.nextStatus(status, MigrationStatus{Mode: MigrationCommitted, Version: status.Version, LegacyHash: stage.ExpectedLegacyHash, UserHash: stage.TargetUserHash, CoreHash: stage.TargetCoreHash, BackupToken: stage.Token})
	if err := s.saveMigrationStatus(committed); err != nil {
		return status, err
	}
	_ = os.RemoveAll(stageDir)
	return committed, nil
}

func (s *Seeder) PreviewMigration() (MigrationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationPreview{}, err
	}
	if status.Mode == MigrationStaged {
		if status, err = s.recoverStage(status); err != nil {
			return MigrationPreview{}, err
		}
	}
	preview := MigrationPreview{Status: status, Legacy: readOptionalText(filepath.Join(s.ContextDir(), legacyContextName)), ManagedCore: readOptionalText(filepath.Join(s.ContextDir(), managedCoreName)), UserContext: readOptionalText(filepath.Join(s.ContextDir(), userContextName))}
	if status.BaseHash != "" && s.sourceDir != "" {
		manifest, loadErr := loadStockManifest(s.sourceDir)
		if loadErr == nil {
			if base, ok := manifest.base(s.sourceDir, status.BaseHash); ok {
				preview.KnownBase, preview.BaseKnown = string(base), true
			}
		}
	}
	if status.Mode == MigrationPending || status.Mode == MigrationLegacy || status.Mode == MigrationRolledBack {
		preview.RequiresResolution = true
		if preview.BaseKnown {
			preview.CandidateUser, preview.HasConflicts = threeWayConflict(preview.UserContext, preview.KnownBase, preview.Legacy), true
		} else {
			preview.CandidateUser = preview.UserContext
		}
	}
	return preview, nil
}
func threeWayConflict(user, base, legacy string) string {
	return "<<<<<<< USER.md\n" + user + "\n||||||| known TACK.md base\n" + base + "\n=======\n" + legacy + "\n>>>>>>> modified TACK.md\n"
}
func readOptionalText(path string) string { data, _ := os.ReadFile(path); return string(data) }

func (s *Seeder) AcceptMigration(req AcceptMigrationRequest) (MigrationStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationStatus{}, err
	}
	if status.Generation != req.ExpectedGeneration || !strings.EqualFold(status.LegacyHash, req.ExpectedLegacyHash) || !strings.EqualFold(status.UserHash, req.ExpectedUserHash) || !strings.EqualFold(status.CoreHash, req.ExpectedCoreHash) {
		return status, ErrMigrationConflict
	}
	if status.Mode != MigrationPending && status.Mode != MigrationLegacy && status.Mode != MigrationRolledBack {
		return status, errors.New("context migration is not awaiting acceptance")
	}
	if strings.Contains(req.ResolvedUser, "<<<<<<<") || strings.Contains(req.ResolvedUser, ">>>>>>>") || strings.Contains(req.ResolvedUser, "|||||||") {
		return status, errors.New("resolve all context merge markers before accepting")
	}
	legacyHash, err := fileSHA256(filepath.Join(s.ContextDir(), legacyContextName))
	if err != nil || !strings.EqualFold(legacyHash, req.ExpectedLegacyHash) {
		return status, ErrMigrationConflict
	}
	user, userExists, err := optionalFile(filepath.Join(s.ContextDir(), userContextName))
	if err != nil {
		return status, err
	}
	if userExists && !strings.EqualFold(bytesSHA256(user), req.ExpectedUserHash) {
		return status, ErrMigrationConflict
	}
	core, err := os.ReadFile(filepath.Join(s.ContextDir(), managedCoreName))
	if err != nil || !strings.EqualFold(bytesSHA256(core), req.ExpectedCoreHash) {
		return status, ErrMigrationConflict
	}
	if err := s.beginAndCommit(status, core, []byte(req.ResolvedUser), legacyHash, user, userExists); err != nil {
		return status, err
	}
	return s.loadMigrationStatus()
}

func (s *Seeder) RollbackMigration(req RollbackMigrationRequest) (MigrationStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.loadMigrationStatus()
	if err != nil {
		return MigrationStatus{}, err
	}
	if status.Generation != req.ExpectedGeneration || req.Token != status.BackupToken || !safeToken(req.Token) {
		return status, ErrMigrationConflict
	}
	if status.Mode != MigrationCommitted {
		return status, errors.New("context migration is not committed")
	}
	backupPath := filepath.Join(s.backupDir(), req.Token)
	metaBytes, err := os.ReadFile(filepath.Join(backupPath, "backup.json"))
	if err != nil {
		return status, fmt.Errorf("read context rollback metadata: %w", err)
	}
	var meta backupMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return status, fmt.Errorf("parse context rollback metadata: %w", err)
	}
	legacy, err := os.ReadFile(filepath.Join(backupPath, legacyContextName))
	if err != nil || bytesSHA256(legacy) != meta.LegacyHash {
		return status, errors.New("context rollback backup is missing or corrupt")
	}
	if err := atomicWriteFile(filepath.Join(s.ContextDir(), legacyContextName), legacy, 0o644); err != nil {
		return status, err
	}
	if meta.UserExisted {
		user, readErr := os.ReadFile(filepath.Join(backupPath, userContextName))
		if readErr != nil || bytesSHA256(user) != meta.UserHash {
			return status, errors.New("context rollback user backup is missing or corrupt")
		}
		if err := atomicWriteFile(filepath.Join(s.ContextDir(), userContextName), user, 0o644); err != nil {
			return status, err
		}
	} else {
		_ = os.Remove(filepath.Join(s.ContextDir(), userContextName))
	}
	rolled := s.nextStatus(status, MigrationStatus{Mode: MigrationRolledBack, Version: status.Version, LegacyHash: meta.LegacyHash, CoreHash: status.CoreHash, UserHash: meta.UserHash, BackupToken: req.Token})
	if err := s.saveMigrationStatus(rolled); err != nil {
		return status, err
	}
	return rolled, nil
}
