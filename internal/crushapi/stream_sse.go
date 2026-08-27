// stream_sse.go -- role: read the SSE stream and produce typed events.
//
// Token deltas, tool activity, permission and question requests. Streaming is
// the only path state takes to the UI: never poll a REST route instead.
package crushapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// StreamEvent is one decoded Server-Sent Event. The Crush server emits a
// single data line per envelope:
//
//	data: {"type":"<kind>","payload":{"type":"created|updated|deleted","payload":{...}}}
//
// Kind is the outer PayloadType (e.g. "message", "permission_request",
// "run_complete"). Event is the inner lifecycle type ("created", "updated",
// "deleted"). Payload is the raw JSON of the inner payload so callers can
// decode it with the right proto type.
type StreamEvent struct {
	Kind    string
	Event   string
	Payload json.RawMessage
}

// envelope is the outer JSON shape on the wire: a PayloadType discriminator
// plus the inner pubsub.Event encoded as raw JSON.
type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// innerEvent is the pubsub.Event[proto.T] shape inside the envelope: a
// lifecycle type plus the proto payload. Only Type is read here; the rest
// is handed off as Payload.
type innerEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Stream subscribes to /v1/workspaces/{id}/events?client_id=<uuid>. The
// returned channel is closed when the body EOFs, the context is cancelled,
// or an unrecoverable read error occurs. The cancel function cancels the
// request context, which makes the underlying transport close the stream.
func (c *Client) Stream(ctx context.Context, wsID string) (<-chan StreamEvent, func(), error) {
	path := expandPath(eventsPath, "id", wsID) + "?client_id=" + url.QueryEscape(c.clientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL(path), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		return nil, nil, decodeError(resp)
	}
	streamCtx, cancel := context.WithCancel(ctx)
	out := make(chan StreamEvent, 32)
	go c.readEvents(streamCtx, resp, out)
	return out, cancel, nil
}

// readEvents drains the SSE body into out. bufio.Reader is used rather than
// bufio.Scanner on purpose: Scanner requires an explicit maximum token size,
// and an envelope larger than that cap stopped Scan() mid-stream with the
// reason discarded, so the UI silently froze with no error anywhere. Reader
// grows only as far as the longest line actually needs and has no such mode.
func (c *Client) readEvents(ctx context.Context, resp *http.Response, out chan<- StreamEvent) {
	defer close(out)
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		// A final line without a trailing newline is still a complete event, so
		// handle the payload before reacting to err.
		if payload, ok := ssePayload(line); ok {
			// A bad envelope is skipped rather than tearing down the stream:
			// the server recovers on the next event.
			if ev, decErr := decodeEnvelope(payload); decErr == nil {
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
		if err != nil {
			// EOF and the "use of closed network connection" family are the
			// expected shutdowns. The closed channel is the only signal
			// callers get, per the spec.
			return
		}
	}
}

// ssePayload extracts the data of one SSE line. Blank lines (the event
// terminator), comments and non-data fields report false.
func ssePayload(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	payload := strings.TrimSpace(line[len("data:"):])
	return payload, payload != ""
}

// decodeEnvelope parses one SSE data line into a StreamEvent.
func decodeEnvelope(line string) (StreamEvent, error) {
	var env envelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		return StreamEvent{}, err
	}
	var inner innerEvent
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &inner); err != nil {
			// Some payloads (e.g. config_changed) are flat objects
			// without an inner Event wrapper. Fall back to the
			// raw payload and let the caller decide what to do.
			return StreamEvent{Kind: env.Type, Event: "", Payload: env.Payload}, nil
		}
	}
	return StreamEvent{Kind: env.Type, Event: inner.Type, Payload: inner.Payload}, nil
}
