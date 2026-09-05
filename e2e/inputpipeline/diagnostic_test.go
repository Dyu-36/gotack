//go:build e2e

package inputpipeline

import (
	"context"
	"errors"
	"testing"
)

func TestE2EProviderAuxiliaryDiagnostics(t *testing.T) {
	p := newFakeProvider()
	t.Cleanup(p.server.Close)
	h := startEngine(t, t.TempDir(), p, false)
	session := freshSession(t, h)
	run := newID(t)
	p.arm(run, modeText)
	must(t, h.client.SendPromptWithAttachments(h.ctx, h.workspace, session, "Run the synthetic fixture.", run, nil), "prompt_submit_failed")
	_, err := waitTerminal(h.ctx, func(ctx context.Context) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-h.events:
			if !ok {
				return nil, errors.New("stream_closed")
			}
			return event.Payload, nil
		}
	}, run, session)
	must(t, err, "matching_terminal_missing")
	p.mu.Lock()
	auxiliary, rejected := p.auxiliary, p.rejected
	p.mu.Unlock()
	counts := p.counts(run)
	t.Logf("provider_diagnostic requests=%d invalid=%d auxiliary=%d rejected=%d", counts.Requests, counts.Invalid, auxiliary, rejected)
}
