// office is the gotack Office MCP server. It exposes Word, Excel and
// PowerPoint operations to coding agents over the Model Context Protocol
// stdio transport. Gotack registers this binary in the Crush engine config as
// mcp_servers.gotack-office when a workspace opens.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyu-36/gotack/internal/mcp"
	"github.com/Dyu-36/gotack/internal/office"
)

const (
	serverName    = "gotack-office"
	serverVersion = "0.1.0"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &mcp.Server{
		Name:    serverName,
		Version: serverVersion,
		Tools: []mcp.Tool{toolInfo(), toolRead(), toolCreate(), toolEdit()},
	}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "office: %v\n", err)
		os.Exit(1)
	}
}

func toolInfo() mcp.Tool {
	return mcp.Tool{
		Name:        "office_info",
		Description: "Summarize the structure of a Word (.docx), Excel (.xlsx) or PowerPoint (.pptx) file.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {"path": {"type": "string", "description": "Absolute path to the office file"}},
			"required": ["path"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := office.MarshalArgs[struct {
				Path string `json:"path"`
			}](args)
			if err != nil {
				return "", err
			}
			return office.Info(req.Path)
		},
	}
}

func toolRead() mcp.Tool {
	return mcp.Tool{
		Name:        "office_read",
		Description: "Extract content from a Word, Excel or PowerPoint file as text. Excel sheets become TSV; Word and PowerPoint render as plain text and slide text.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute path to the office file"},
				"sheet": {"type": "string", "description": "Excel only: sheet name; defaults to the first sheet"}
			},
			"required": ["path"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := office.MarshalArgs[struct {
				Path  string `json:"path"`
				Sheet string `json:"sheet"`
			}](args)
			if err != nil {
				return "", err
			}
			return office.Read(req.Path, req.Sheet)
		},
	}
}

func toolCreate() mcp.Tool {
	return mcp.Tool{
		Name:        "office_create",
		Description: "Create a new office file. Word (.docx) and PowerPoint (.pptx) take markdown: '# Title' headings, '## / ###' sub-headings, '- ' bullets, '1. ' numbered items, '---' dividers, '| a | b |' tables (Word), **bold** and *italic*. Excel (.xlsx) takes TSV rows; one row per line, cells separated by tabs. The first '# Heading' starts each PowerPoint slide.",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute path of the file to create (.docx, .xlsx or .pptx)"},
				"content": {"type": "string", "description": "Markdown for .docx/.pptx, TSV for .xlsx"}
			},
			"required": ["path", "content"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := office.MarshalArgs[office.CreateRequest](args)
			if err != nil {
				return "", err
			}
			return office.Create(req)
		},
	}
}

func toolEdit() mcp.Tool {
	return mcp.Tool{
		Name:        "office_edit",
		Description: "Edit an existing office file. Operations: replace_text (find/replace inside .docx or .pptx text), set_cell (write one Excel cell), append_rows (append TSV rows to an Excel sheet).",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"op": {"type": "string", "enum": ["replace_text", "set_cell", "append_rows"]},
				"path": {"type": "string", "description": "Absolute path to the office file"},
				"find": {"type": "string", "description": "replace_text: text to find"},
				"replace": {"type": "string", "description": "replace_text: replacement text"},
				"sheet": {"type": "string", "description": "Excel only: sheet name; defaults to the first sheet"},
				"cell": {"type": "string", "description": "set_cell: cell address such as B2"},
				"value": {"type": "string", "description": "set_cell: new cell value"},
				"rows": {"type": "string", "description": "append_rows: TSV rows, one row per line"}
			},
			"required": ["op", "path"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			req, err := office.MarshalArgs[office.EditRequest](args)
			if err != nil {
				return "", err
			}
			return office.Edit(req)
		},
	}
}
