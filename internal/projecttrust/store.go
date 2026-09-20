package projecttrust

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type Status struct {
	Path          string
	Trusted       bool
	Decided       bool
	Required      bool
	InheritedFrom string
	Resources     []string
}

type fileState struct {
	Decisions map[string]bool `json:"decisions"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store { return &Store{path: path} }

func (s *Store) Inspect(projectPath string) (Status, error) {
	canonical, err := canonicalPath(projectPath)
	if err != nil {
		return Status{}, err
	}
	resources, err := protectedResources(canonical)
	if err != nil {
		return Status{}, err
	}
	if len(resources) == 0 {
		return Status{Path: canonical, Trusted: true}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadLocked()
	if err != nil {
		return Status{}, err
	}
	trusted, source, decided := closestDecision(state.Decisions, canonical)
	return Status{
		Path:          canonical,
		Trusted:       trusted,
		Decided:       decided,
		Required:      !decided,
		InheritedFrom: source,
		Resources:     resources,
	}, nil
}

func (s *Store) Set(projectPath string, trusted bool) error {
	canonical, err := canonicalPath(projectPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadLocked()
	if err != nil {
		return err
	}
	if state.Decisions == nil {
		state.Decisions = make(map[string]bool)
	}
	state.Decisions[pathKey(canonical)] = trusted
	return s.saveLocked(state)
}

func (s *Store) Clear(projectPath string) error {
	canonical, err := canonicalPath(projectPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadLocked()
	if err != nil {
		return err
	}
	delete(state.Decisions, pathKey(canonical))
	return s.saveLocked(state)
}

func (s *Store) loadLocked() (fileState, error) {
	state := fileState{Decisions: make(map[string]bool)}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read project trust: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, fmt.Errorf("parse project trust: %w", err)
	}
	if state.Decisions == nil {
		state.Decisions = make(map[string]bool)
	}
	return state, nil
}

func (s *Store) saveLocked(state fileState) error {
	if s.path == "" {
		return errors.New("project trust store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create project trust directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project trust: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".project-trust-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("replace project trust: %w", err)
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("project path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = filepath.Clean(resolved)
	}
	return absolute, nil
}

func pathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func closestDecision(decisions map[string]bool, path string) (bool, string, bool) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		if trusted, ok := decisions[pathKey(current)]; ok {
			return trusted, current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return false, "", false
}

func protectedResources(projectPath string) ([]string, error) {
	candidates := []string{
		filepath.Join(projectPath, ".pi", "settings.json"),
		filepath.Join(projectPath, ".pi", "extensions"),
		filepath.Join(projectPath, ".pi", "skills"),
		filepath.Join(projectPath, ".pi", "prompts"),
		filepath.Join(projectPath, ".pi", "themes"),
		filepath.Join(projectPath, ".pi", "SYSTEM.md"),
		filepath.Join(projectPath, ".pi", "APPEND_SYSTEM.md"),
		filepath.Join(projectPath, ".tack", "SYSTEM.md"),
		filepath.Join(projectPath, ".tack", "APPEND_SYSTEM.md"),
		filepath.Join(projectPath, ".agents", "skills"),
	}
	resources := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		_, err := os.Lstat(candidate)
		switch {
		case err == nil:
			rel, relErr := filepath.Rel(projectPath, candidate)
			if relErr != nil {
				return nil, relErr
			}
			resources = append(resources, filepath.ToSlash(rel))
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("inspect protected project resource %s: %w", candidate, err)
		}
	}
	sort.Strings(resources)
	return resources, nil
}
