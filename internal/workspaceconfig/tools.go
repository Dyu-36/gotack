package workspaceconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

const (
	MemoryMCPName = "gotack-memory"
	SkillsMCPName = "gotack-skills"
	RecallMCPName = "gotack-recall"
)

func BinaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func ResolveBinary(name string) string {
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", name),
			filepath.Join(root, name),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return ""
}

func MemoryEntry(command string) map[string]any {
	return map[string]any{"command": command, "type": "stdio", "timeout": 30}
}

func SkillsEntry(command, root string) map[string]any {
	return map[string]any{
		"command": command,
		"args":    []string{"--root", root},
		"type":    "stdio",
		"timeout": 30,
	}
}

func RecallEntry(command, dataDir, indexDir string) map[string]any {
	return map[string]any{
		"command": command,
		"args":    []string{"--data-dir", dataDir, "--index-dir", indexDir},
		"type":    "stdio",
		"timeout": 30,
	}
}

func RegisterMemory(base context.Context, api *crushapi.Client, workspaceID, command string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()
	key := "mcp_servers." + MemoryMCPName
	if command == "" {
		if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key); err != nil {
			return fmt.Errorf("memory registration removal: %w", err)
		}
		return nil
	}
	if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key, MemoryEntry(command)); err != nil {
		return fmt.Errorf("memory registration: %w", err)
	}
	return nil
}

func RegisterSkills(base context.Context, api *crushapi.Client, workspaceID, command, root string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()
	key := "mcp_servers." + SkillsMCPName
	if command == "" {
		if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key); err != nil {
			return fmt.Errorf("skills registration removal: %w", err)
		}
		return nil
	}
	if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key, SkillsEntry(command, root)); err != nil {
		return fmt.Errorf("skills registration: %w", err)
	}
	return nil
}

func RegisterRecall(base context.Context, api *crushapi.Client, workspaceID string, desc workspace.Descriptor, command, indexRoot string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()
	key := "mcp_servers." + RecallMCPName
	if command == "" || desc.WorkspaceID != workspaceID || desc.DataDir == "" {
		if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key); err != nil {
			return fmt.Errorf("recall registration removal: %w", err)
		}
		return nil
	}
	entry := RecallEntry(command, desc.DataDir, filepath.Join(indexRoot, workspaceID))
	if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key, entry); err != nil {
		return fmt.Errorf("recall registration: %w", err)
	}
	return nil
}

func registrationContext(base context.Context) (context.Context, context.CancelFunc) {
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, 10*time.Second)
}
