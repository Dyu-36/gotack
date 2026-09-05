package crushapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type StreamEvent struct {
	Kind    string
	Event   string
	Payload json.RawMessage
}

type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type innerEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (c *Client) Stream(ctx context.Context, wsID string, kinds ...string) (<-chan StreamEvent, func(), error) {
	path := expandPath(eventsPath, "id", wsID) + "?client_id=" + url.QueryEscape(c.clientID)

	streamCtx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, requestURL(path), nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.hc.Do(req)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		cancel()
		return nil, nil, decodeError(resp)
	}

	allow := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		allow[k] = struct{}{}
	}
	out := make(chan StreamEvent, 32)

	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			_ = resp.Body.Close()
		})
	}
	go func() {
		defer stop()
		c.readEvents(streamCtx, resp, out, allow)
	}()

	return out, stop, nil
}

func (c *Client) readEvents(ctx context.Context, resp *http.Response, out chan<- StreamEvent, allow map[string]struct{}) {
	defer close(out)
	defer resp.Body.Close()

	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')

		if payload, ok := ssePayload(line); ok {

			if ev, emit := decodeEnvelope(payload, allow); emit {
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
		if err != nil {

			return
		}
	}
}

func ssePayload(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	payload := strings.TrimSpace(line[len("data:"):])
	return payload, payload != ""
}

func decodeEnvelope(line string, allow map[string]struct{}) (StreamEvent, bool) {
	var env envelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		return StreamEvent{}, false
	}
	if _, ok := allow[env.Type]; len(allow) > 0 && !ok {
		return StreamEvent{}, false
	}

	// Most lifecycle events are wrapped as {type, payload:{type,payload}}, but
	// terminal and permission events are intentionally flat. Decoding a flat
	// JSON object into innerEvent succeeds with zero-valued fields, so success
	// alone cannot distinguish the two shapes. Only unwrap when both wrapper
	// fields are actually present; otherwise preserve the original payload.
	var inner innerEvent
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &inner); err == nil && inner.Type != "" && len(inner.Payload) > 0 {
			return StreamEvent{Kind: env.Type, Event: inner.Type, Payload: inner.Payload}, true
		}
	}
	return StreamEvent{Kind: env.Type, Event: "", Payload: env.Payload}, true
}
