package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

const protocolVersion = "2024-11-05"

type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Handler     func(context.Context, json.RawMessage) (string, error)
}

type Server struct {
	Name    string
	Version string
	Tools   []Tool
}

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader, encoder := bufio.NewReader(in), json.NewEncoder(out)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if response := s.handle(ctx, line); response != nil {
				if err := encoder.Encode(response); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, line []byte) json.RawMessage {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		log.Printf("mcp: skipping malformed line: %v", err)
		return nil
	}
	if req.ID == nil {
		return nil
	}

	switch req.Method {
	case "initialize":
		return reply(req.ID, "result", map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		})
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, tool := range s.Tools {
			tools = append(tools, map[string]any{
				"name": tool.Name, "description": tool.Description, "inputSchema": tool.Schema,
			})
		}
		return reply(req.ID, "result", map[string]any{"tools": tools})
	case "tools/call":
		return s.callTool(ctx, req.ID, req.Params)
	case "ping":
		return reply(req.ID, "result", map[string]any{})
	default:
		return rpcError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) callTool(ctx context.Context, id, params json.RawMessage) json.RawMessage {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return rpcError(id, -32602, "invalid params: "+err.Error())
	}

	for _, tool := range s.Tools {
		if tool.Name != req.Name {
			continue
		}
		text, err := tool.Handler(ctx, req.Arguments)
		result := map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
		if err != nil {
			result["content"] = []map[string]string{{"type": "text", "text": err.Error()}}
			result["isError"] = true
		}
		return reply(id, "result", result)
	}
	return rpcError(id, -32602, fmt.Sprintf("unknown tool: %s", req.Name))
}

func rpcError(id json.RawMessage, code int, message string) json.RawMessage {
	return reply(id, "error", map[string]any{"code": code, "message": message})
}

func reply(id json.RawMessage, key string, payload any) json.RawMessage {
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, key: payload})
	if err != nil {
		log.Printf("mcp: encode response: %v", err)
		return nil
	}
	return raw
}
