// Package terminal is an IPC frontend for the same runtime used by the desktop.
package terminal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/buildinfo"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/projecttrust"
	"github.com/Dyu-36/gotack/internal/workspace"
	"github.com/Dyu-36/gotack/internal/workspaceconfig"
	"github.com/google/uuid"
)

type inputLine struct {
	text string
	err  error
}

func Run(ctx context.Context, args []string, in io.Reader, out, diagnostic io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, "Usage: gotack chat|run [--workspace PATH] [--session ID] [--provider ID --model ID] [--yolo] [PROMPT]\n       gotack serve [--host ADDRESS] [--data-dir PATH] [--debug]\nChat commands: /quit, /new, /sessions; /cancel during a run. Ctrl+C cancels and exits.")
		return nil
	}
	mode := args[0]
	if mode != "chat" && mode != "run" {
		return fmt.Errorf("unknown command %q; use gotack help", mode)
	}
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	dir := flags.String("workspace", ".", "workspace directory")
	sessionID := flags.String("session", "", "resume an existing session")
	provider := flags.String("provider", "", "provider ID")
	model := flags.String("model", "", "model ID")
	yolo := flags.Bool("yolo", false, "approve tool execution automatically")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if (*provider == "") != (*model == "") {
		return errors.New("--provider and --model must be supplied together")
	}
	if mode == "run" && flags.NArg() == 0 {
		return errors.New("gotack run requires a prompt")
	}
	if mode == "chat" && flags.NArg() != 0 {
		return errors.New("gotack chat accepts prompts interactively")
	}
	path, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
		return fmt.Errorf("workspace directory unavailable: %s", path)
	}
	cfg, err := appconfig.Load()
	if err != nil {
		return err
	}
	sup := engine.NewSupervisor(nil, cfg.EngineBinary)
	expected := buildinfo.Revision()
	if cfg.EngineBinary != "" {
		expected = ""
	}
	link := engine.NewLink(sup, expected)
	scope, _ := link.BeginConnect(ctx)
	defer link.Disconnect()
	var api *engineapi.Client
	if err := link.Connect(scope, func(_ context.Context, client *engineapi.Client, _ engineapi.Endpoint, _ string) error {
		api = client
		return nil
	}); err != nil {
		_ = sup.Stop()
		return err
	}
	// Creating an already open workspace attaches this client; it does not replace its data.
	dataDir := ""
	if path == filepath.VolumeName(path)+string(filepath.Separator) {
		dataDir = filepath.Join(appconfig.Dir(), "default-workspace-data")
	}
	ws, err := api.CreateWorkspaceWithDataDir(ctx, path, dataDir, *yolo)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = api.ReleaseWorkspace(cleanup, ws.ID)
	}()
	events, stop, err := api.Stream(ctx, ws.ID)
	if err != nil {
		return err
	}
	defer stop()
	trust, err := projecttrust.New(filepath.Join(appconfig.Dir(), "project-trust.json")).Inspect(path)
	if err != nil {
		return err
	}
	manager := workspaceconfig.NewManager(workspaceconfig.Options{UserSkillsDir: filepath.Join(appconfig.Dir(), "skills"), ManagedRoot: appconfig.Dir()})
	if err := manager.Apply(ctx, api, workspace.Descriptor{Path: path, WorkspaceID: ws.ID, DataDir: ws.DataDir}, trust.Trusted); err != nil {
		return err
	}
	if *model != "" {
		if err := api.SetPreferredModelPair(ctx, ws.ID, engineapi.ConfigScopeWorkspace, engineapi.SelectedModel{Provider: *provider, Model: *model}); err != nil {
			return err
		}
	}
	if err := api.EnsureAgent(ctx, ws.ID, true); err != nil {
		return err
	}
	if *sessionID == "" {
		s, err := api.CreateSession(ctx, ws.ID, "Terminal chat")
		if err != nil {
			return err
		}
		*sessionID = s.ID
	} else {
		if _, err := api.GetSession(ctx, ws.ID, *sessionID); err != nil {
			return err
		}
		messages, err := api.Messages(ctx, ws.ID, *sessionID)
		if err != nil {
			return err
		}
		for _, message := range messages {
			if text := engineapi.ExtractParts(message.Parts).Text; text != "" {
				fmt.Fprintf(out, "%s: %s\n", message.Role, text)
			}
		}
	}
	if err := api.SetCurrentSession(ctx, ws.ID, *sessionID); err != nil {
		return err
	}
	lines := make(chan inputLine)
	inputCtx, cancelInput := context.WithCancel(ctx)
	defer cancelInput()
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		defer close(lines)
		for scanner.Scan() {
			select {
			case lines <- inputLine{text: scanner.Text()}:
			case <-inputCtx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- inputLine{err: err}:
			case <-inputCtx.Done():
			}
		}
	}()
	if mode == "run" {
		return runPrompt(ctx, api, ws.ID, *sessionID, strings.Join(flags.Args(), " "), events, lines, out, diagnostic)
	}
	fmt.Fprintf(diagnostic, "Session %s\n", *sessionID)
	for {
		fmt.Fprint(out, "\n> ")
		line, err := readLine(ctx, lines)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch strings.TrimSpace(line) {
		case "":
			continue
		case "/quit", "/exit":
			return nil
		case "/sessions":
			sessions, err := api.ListSessions(ctx, ws.ID)
			if err != nil {
				return err
			}
			for _, s := range sessions {
				fmt.Fprintf(out, "%s  %s\n", s.ID, s.Title)
			}
			continue
		case "/new":
			s, err := api.CreateSession(ctx, ws.ID, "Terminal chat")
			if err != nil {
				return err
			}
			*sessionID = s.ID
			if err := api.SetCurrentSession(ctx, ws.ID, s.ID); err != nil {
				return err
			}
			continue
		}
		if err := runPrompt(ctx, api, ws.ID, *sessionID, line, events, lines, out, diagnostic); err != nil {
			if ctx.Err() != nil {
				return err
			}
			fmt.Fprintln(diagnostic, err)
		}
	}
}

func readLine(ctx context.Context, lines <-chan inputLine) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case line, ok := <-lines:
		if !ok {
			return "", io.EOF
		}
		return line.text, line.err
	}
}

func cancelPrompt(api *engineapi.Client, ws, session string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = api.CancelPrompt(ctx, ws, session)
}

func runPrompt(ctx context.Context, api *engineapi.Client, ws, session, prompt string, events <-chan engineapi.StreamEvent, lines <-chan inputLine, out, diagnostic io.Writer) error {
	runID := uuid.NewString()
	if err := api.SendPromptWithAttachments(ctx, ws, session, prompt, runID, nil); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			cancelPrompt(api, ws, session)
		}
	}()
	texts := map[string]string{}
	tools := map[string]bool{}
	var pending *engineapi.PermissionRequest
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-lines:
			if !ok {
				lines = nil
				if pending != nil {
					if err := api.GrantPermission(ctx, ws, *pending, "deny"); err != nil {
						return err
					}
					pending = nil
				}
				continue
			}
			if line.err != nil {
				return line.err
			}
			if strings.TrimSpace(line.text) == "/cancel" {
				cancelPrompt(api, ws, session)
				pending = nil
			} else if pending != nil {
				action := "deny"
				switch strings.ToLower(strings.TrimSpace(line.text)) {
				case "y", "yes":
					action = "allow"
				case "a":
					action = "allow_session"
				}
				if err := api.GrantPermission(ctx, ws, *pending, action); err != nil {
					return err
				}
				pending = nil
			} else {
				fmt.Fprintln(diagnostic, "A run is active. Use /cancel or wait for completion.")
			}
		case event, ok := <-events:
			if !ok {
				return errors.New("engine event stream disconnected")
			}
			switch event.Kind {
			case "question_batch_request":
				if err := answerQuestions(ctx, api, ws, session, event.Payload, lines, diagnostic); err != nil {
					return err
				}
			case "message":
				var message engineapi.Message
				if err := json.Unmarshal(event.Payload, &message); err != nil {
					return err
				}
				if message.SessionID != session {
					continue
				}
				parts := engineapi.ExtractParts(message.Parts)
				if message.Role == "assistant" {
					previous := texts[message.ID]
					if strings.HasPrefix(parts.Text, previous) {
						fmt.Fprint(out, strings.TrimPrefix(parts.Text, previous))
					}
					texts[message.ID] = parts.Text
				}
				for _, tool := range parts.ToolCalls {
					if !tools[tool.ID] {
						fmt.Fprintf(diagnostic, "\nTool: %s %s\n", tool.Name, tool.Input)
						tools[tool.ID] = true
					}
				}
			case "permission_request":
				var request engineapi.PermissionRequest
				if err := json.Unmarshal(event.Payload, &request); err != nil {
					return err
				}
				if request.SessionID != session {
					continue
				}
				fmt.Fprintf(diagnostic, "\n%s: %s\n%s %s\nAllow? [y] once, [a] session, [n] deny: ", request.ToolName, request.Description, request.Action, request.Path)
				pending = &request
				if lines == nil {
					if err := api.GrantPermission(ctx, ws, request, "deny"); err != nil {
						return err
					}
					pending = nil
				}
			case "permission_notification":
				var notification struct {
					ToolCallID string `json:"tool_call_id"`
					Granted    bool   `json:"granted"`
					Denied     bool   `json:"denied"`
				}
				if err := json.Unmarshal(event.Payload, &notification); err != nil {
					return err
				}
				if pending != nil && pending.ToolCallID == notification.ToolCallID && (notification.Granted || notification.Denied) {
					pending = nil
				}
			case "run_complete":
				var result engineapi.RunComplete
				if err := json.Unmarshal(event.Payload, &result); err != nil {
					return err
				}
				if result.SessionID != session || result.RunID != runID {
					continue
				}
				complete = true
				if len(texts) == 0 {
					fmt.Fprint(out, result.Text)
				}
				fmt.Fprintln(out)
				if result.Error != "" {
					return errors.New(result.Error)
				}
				if result.Cancelled {
					return errors.New("run cancelled")
				}
				return nil
			}
		}
	}
}
