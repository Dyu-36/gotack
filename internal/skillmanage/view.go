package skillmanage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/Dyu-36/gotack/internal/mcp"
)

// ViewToolName is the safety-handshake tool for the managed skill writer.
// Crush remains the canonical source for the catalog and ordinary skill reads;
// this tool exists only because a separate skills MCP process must remember
// which exact file a background review inspected before it may mutate it.
const ViewToolName = "skill_view"

// ViewResult is intentionally compact. FilePath is relative to the selected
// skill directory (SKILL.md when omitted in the request), so it can be copied
// directly into a later skill_manage operation.
type ViewResult struct {
	Success  bool   `json:"success"`
	Name     string `json:"name,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
}

type viewRequest struct {
	Name             string `json:"name"`
	FilePath         string `json:"file_path"`
	SessionID        string `json:"_session_id,omitempty"`
	BackgroundReview bool   `json:"_background_review,omitempty"`
}

// View reads one managed skill file. A background call records a digest in
// process-local state; ApplyWithMeta consumes that mark and rechecks the file
// before writing, so a stale view cannot authorize a changed file.
func (m *Manager) View(ctx context.Context, name, filePath string, meta RequestMeta) (ViewResult, error) {
	if err := ctx.Err(); err != nil {
		return ViewResult{}, err
	}
	if err := validateName(name); err != nil {
		return ViewResult{}, err
	}
	if meta.BackgroundReview && strings.TrimSpace(meta.SessionID) == "" {
		return ViewResult{}, errors.New("background review session is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	skillDir, err := m.requireSkill(name)
	if err != nil {
		return ViewResult{}, err
	}
	requested := strings.TrimSpace(filePath)
	target := filepathJoinSkillFile(skillDir, requested)
	if requested != "" {
		target, err = m.resolveSupportTarget(skillDir, requested)
		if err != nil {
			return ViewResult{}, err
		}
	}
	data, err := m.readRegularFile(target)
	if err != nil {
		return ViewResult{}, err
	}
	if err := validateViewSize(target, data); err != nil {
		return ViewResult{}, err
	}
	if meta.BackgroundReview {
		m.markRead(meta.SessionID, target, viewDigest(string(data)))
	}
	relative := "SKILL.md"
	if requested != "" {
		relative, _ = normalizeSupportPath(requested)
	}
	return ViewResult{Success: true, Name: name, FilePath: relative, Content: string(data)}, nil
}

func filepathJoinSkillFile(skillDir, requested string) string {
	if requested == "" {
		return filepath.Join(skillDir, "SKILL.md")
	}
	return filepath.Join(skillDir, requested)
}

func validateViewSize(path string, data []byte) error {
	if strings.EqualFold(filepath.Base(path), "SKILL.md") {
		if !utf8.Valid(data) {
			return errors.New("SKILL.md is not valid UTF-8")
		}
		if utf8.RuneCount(data) > MaxSkillContent {
			return fmt.Errorf("SKILL.md exceeds %d characters", MaxSkillContent)
		}
		return nil
	}
	if len(data) > MaxSupportFileBytes {
		return fmt.Errorf("support file exceeds %d bytes", MaxSupportFileBytes)
	}
	if !utf8.Valid(data) {
		return errors.New("support file is not valid UTF-8")
	}
	return nil
}

func (m *Manager) markRead(sessionID, target, digest string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	if m.readMarks == nil {
		m.readMarks = make(map[string]map[string]string)
	}
	if m.readMarkOrder == nil {
		m.readMarkOrder = make(map[string][]string)
	}
	marks, exists := m.readMarks[sessionID]
	if !exists {
		if len(m.readSessionOrder) >= maxReadSessions {
			oldest := m.readSessionOrder[0]
			m.readSessionOrder = m.readSessionOrder[1:]
			delete(m.readMarks, oldest)
			delete(m.readMarkOrder, oldest)
		}
		marks = make(map[string]string)
		m.readMarks[sessionID] = marks
		m.readSessionOrder = append(m.readSessionOrder, sessionID)
	}
	key := readPathKey(target)
	if _, exists := marks[key]; !exists {
		order := m.readMarkOrder[sessionID]
		if len(order) >= maxReadMarksPerSession {
			oldest := order[0]
			order = order[1:]
			delete(marks, oldest)
		}
		m.readMarkOrder[sessionID] = append(order, key)
	}
	marks[key] = digest
}

func (m *Manager) takeReadMarks(sessionID string) map[string]string {
	marks := m.readMarks[sessionID]
	if marks == nil {
		return make(map[string]string)
	}
	copyMarks := make(map[string]string, len(marks))
	for path, digest := range marks {
		copyMarks[path] = digest
	}
	delete(m.readMarks, sessionID)
	delete(m.readMarkOrder, sessionID)
	for i, id := range m.readSessionOrder {
		if id == sessionID {
			m.readSessionOrder = append(m.readSessionOrder[:i], m.readSessionOrder[i+1:]...)
			break
		}
	}
	return copyMarks
}

func readPathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(path)
	}
	return path
}

func viewDigest(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func viewTool(manager *Manager) mcp.Tool {
	return mcp.Tool{
		Name: ViewToolName,
		Description: "Read one managed skill file and record an exact background-review safety mark. " +
			"Crush supplies the catalog and canonical view; use this handshake only before skill_manage changes.",
		Schema: json.RawMessage(`{
			"type":"object",
			"additionalProperties":false,
			"properties":{
				"name":{"type":"string","description":"Managed skill name."},
				"file_path":{"type":"string","description":"Optional support file below references/, templates/, scripts/, or assets/."}
			},
			"required":["name"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var request viewRequest
			if err := decodeStrict(args, &request); err != nil {
				return encodeViewResult(ViewResult{Success: false, Error: fmt.Sprintf("decode arguments: %v", err)})
			}
			result, err := manager.View(ctx, request.Name, request.FilePath, RequestMeta{
				SessionID: request.SessionID, BackgroundReview: request.BackgroundReview,
			})
			if err != nil {
				return encodeViewResult(ViewResult{Success: false, Error: err.Error()})
			}
			return encodeViewResult(result)
		},
	}
}

func encodeViewResult(result ViewResult) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode skill view result: %w", err)
	}
	return string(data), nil
}
