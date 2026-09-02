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

	Handler func(ctx context.Context, args json.RawMessage) (string, error)
}

type Server struct {
	Name    string
	Version string
	Tools   []Tool
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	encoder := json.NewEncoder(out)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if response := s.handle(ctx, line); response != nil {
				if encodeErr := encoder.Encode(response); encodeErr != nil {
					return encodeErr
				}
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (s *Server) handle(ctx context.Context, line []byte) json.RawMessage {
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(line, &request); err != nil {
		log.Printf("mcp: skipping malformed line: %v", err)
		return nil
	}
	if request.ID == nil {
		return nil
	}

	switch request.Method {
	case "initialize":
		return s.result(request.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		})
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, tool := range s.Tools {
			tools = append(tools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.Schema,
			})
		}
		return s.result(request.ID, map[string]any{"tools": tools})
	case "tools/call":
		return s.callTool(ctx, request.ID, request.Params)
	case "ping":
		return s.result(request.ID, map[string]any{})
	default:
		return s.error(request.ID, -32601, fmt.Sprintf("method not found: %s", request.Method))
	}
}

func (s *Server) callTool(ctx context.Context, id json.RawMessage, params json.RawMessage) json.RawMessage {
	var request struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return s.error(id, -32602, "invalid params: "+err.Error())
	}
	for _, tool := range s.Tools {
		if tool.Name != request.Name {
			continue
		}
		text, err := tool.Handler(ctx, request.Arguments)
		if err != nil {
			return s.result(id, map[string]any{
				"content": []map[string]string{{"type": "text", "text": err.Error()}},
				"isError": true,
			})
		}
		return s.result(id, map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
		})
	}
	return s.error(id, -32602, fmt.Sprintf("unknown tool: %s", request.Name))
}

func (s *Server) result(id json.RawMessage, payload any) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  payload,
	})
	if err != nil {
		log.Printf("mcp: encode response: %v", err)
		return nil
	}
	return raw
}

func (s *Server) error(id json.RawMessage, code int, message string) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
	if err != nil {
		log.Printf("mcp: encode error response: %v", err)
		return nil
	}
	return raw
}
