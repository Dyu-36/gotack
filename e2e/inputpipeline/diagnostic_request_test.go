//go:build e2e

package inputpipeline

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestE2EUnexpectedRequestClassification(t *testing.T) {
	p := newFakeProvider()
	p.server.Close()
	var mu sync.Mutex
	classes := map[string]int{}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class := r.Method + ":"
		if r.URL.IsAbs() {
			class += "absolute:"
		} else {
			class += "relative:"
		}
		switch r.URL.Path {
		case "/v1/responses":
			class += "responses"
		case "/v1/models":
			class += "models"
		case "/v2/providers":
			class += "catalog"
		default:
			class += "other"
		}
		if r.Header.Get("Authorization") != "" {
			class += ":auth"
		} else {
			class += ":noauth"
		}
		mu.Lock()
		classes[class]++
		mu.Unlock()
		p.serve(w, r)
	}))
	t.Cleanup(p.server.Close)
	h := startEngine(t, t.TempDir(), p, false)
	_, _ = sendTurn(t, h, p, freshSession(t, h), modeText)
	mu.Lock()
	defer mu.Unlock()
	for class, count := range classes {
		t.Logf("request_class %s=%d", class, count)
	}
}
