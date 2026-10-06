package modelcatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testBody = `{"test-provider":{"test-model":{"id":"test-model","name":"Test Model","api":"openai-completions","provider":"test-provider","baseUrl":"https://example.test/v1","reasoning":true,"input":["text","image"],"cost":{"input":1,"output":2,"cacheRead":0.1,"cacheWrite":0.2},"contextWindow":64000,"maxTokens":8192,"type":"chat","futureField":{"enabled":true}}}}`

func TestLoadCachesAndRevalidatesWithETag(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.Header.Get("If-None-Match"); got == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Tue, 06 Oct 2026 12:00:00 GMT")
		w.Header().Set("x-pi-model-catalog-revision", "sha256-test")
		_, _ = w.Write([]byte(testBody))
	}))
	defer server.Close()

	cachePath := filepath.Join(t.TempDir(), "catalog.json")
	client := NewWithOptions(Options{CachePath: cachePath, URL: server.URL, RefreshInterval: time.Hour})
	first, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if got := first["test-provider"]["test-model"].Name; got != "Test Model" {
		t.Fatalf("unexpected first model name %q", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("first load made %d requests, want 1", requests.Load())
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache was not written: %v", err)
	}

	if _, err := client.Load(context.Background()); err != nil {
		t.Fatalf("fresh cache load: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("fresh cache made %d requests, want 1", requests.Load())
	}

	client.current.FetchedAt = time.Now().Add(-2 * time.Hour)
	if _, err := client.Load(context.Background()); err != nil {
		t.Fatalf("conditional refresh: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("conditional refresh made %d requests, want 2", requests.Load())
	}
	if client.current.Revision != "sha256-test" {
		t.Fatalf("304 lost prior revision: %q", client.current.Revision)
	}
}

func TestLoadAcceptsRevisedSnapshotWithHTTP200(t *testing.T) {
	var mu sync.Mutex
	body := testBody
	etag := `"v1"`
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewWithOptions(Options{URL: server.URL, RefreshInterval: time.Hour})
	if _, err := client.Load(context.Background()); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	updatedBody := strings.Replace(testBody, `"Test Model"`, `"Updated Model"`, 1)
	mu.Lock()
	body = updatedBody
	etag = `"v2"`
	mu.Unlock()
	client.current.FetchedAt = time.Now().Add(-2 * time.Hour)

	got, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("revised load: %v", err)
	}
	if got["test-provider"]["test-model"].Name != "Updated Model" {
		t.Fatalf("updated model was not applied: %q", got["test-provider"]["test-model"].Name)
	}
	if client.current.ETag != `"v2"` || requests.Load() != 2 {
		t.Fatalf("new revision not stored: etag=%q requests=%d", client.current.ETag, requests.Load())
	}
}

func TestLoadFallsBackToStaleCacheAndEmbeddedSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "offline", http.StatusBadGateway)
	}))
	defer server.Close()

	cachePath := filepath.Join(t.TempDir(), "catalog.json")
	client := NewWithOptions(Options{CachePath: cachePath, URL: server.URL})
	stale := &cacheRecord{
		Catalog:   Catalog{"stale-provider": {"stale-model": {ID: "stale-model", Name: "Stale"}}},
		Body:      []byte(`{"stale-provider":{"stale-model":{"id":"stale-model","name":"Stale"}}}`),
		FetchedAt: time.Now().Add(-time.Hour),
	}
	client.writeCache(stale)

	got, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("load with stale disk cache: %v", err)
	}
	if got["stale-provider"]["stale-model"].Name != "Stale" {
		t.Fatal("load did not use stale disk cache")
	}

	if err := os.Remove(cachePath); err != nil {
		t.Fatal(err)
	}
	client.current = nil
	got, err = client.Load(context.Background())
	if err != nil {
		t.Fatalf("load with embedded fallback: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("embedded snapshot has only %d providers", len(got))
	}
}

func TestCorruptCacheIsIgnoredAndUnknownFieldsSurvive(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(cachePath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(testBody))
	}))
	defer server.Close()

	client := NewWithOptions(Options{CachePath: cachePath, URL: server.URL})
	got, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("load after corrupt cache: %v", err)
	}
	model := got["test-provider"]["test-model"]
	if requests.Load() != 1 {
		t.Fatalf("corrupt cache made %d requests, want 1", requests.Load())
	}
	if !strings.Contains(string(model.AdditionalProperties["futureField"]), `"enabled":true`) {
		t.Fatalf("unknown field was not preserved: %s", model.AdditionalProperties["futureField"])
	}
	encoded, err := model.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal model with unknown fields: %v", err)
	}
	if !strings.Contains(string(encoded), `"futureField":{"enabled":true}`) {
		t.Fatalf("unknown field missing after marshal: %s", encoded)
	}
}

func TestDiskCachePreservesUnknownFieldsAcrossRestart(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "catalog.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testBody))
	}))
	defer server.Close()

	first := NewWithOptions(Options{CachePath: cachePath, URL: server.URL})
	if _, err := first.Load(context.Background()); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	second := NewWithOptions(Options{CachePath: cachePath, URL: "http://127.0.0.1:1"})
	got, err := second.Load(context.Background())
	if err != nil {
		t.Fatalf("load after restart: %v", err)
	}
	model := got["test-provider"]["test-model"]
	if !strings.Contains(string(model.AdditionalProperties["futureField"]), `"enabled":true`) {
		t.Fatalf("disk cache lost unknown metadata: %s", model.AdditionalProperties["futureField"])
	}
}

func TestMalformedAndEmptyUpdatesDoNotReplaceLastGoodCache(t *testing.T) {
	for _, badBody := range []string{"{not json", `{}`} {
		t.Run(badBody, func(t *testing.T) {
			var mu sync.Mutex
			body := testBody
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			cachePath := filepath.Join(t.TempDir(), "catalog.json")
			client := NewWithOptions(Options{CachePath: cachePath, URL: server.URL, RefreshInterval: time.Hour})
			if _, err := client.Load(context.Background()); err != nil {
				t.Fatalf("initial load: %v", err)
			}
			goodCache, err := os.ReadFile(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			body = badBody
			mu.Unlock()
			client.current.FetchedAt = time.Now().Add(-2 * time.Hour)
			got, err := client.Load(context.Background())
			if err != nil {
				t.Fatalf("load with invalid update: %v", err)
			}
			if got["test-provider"]["test-model"].Name != "Test Model" {
				t.Fatal("invalid update replaced in-memory good catalog")
			}
			goodAfter, err := os.ReadFile(cachePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(goodAfter) != string(goodCache) {
				t.Fatal("invalid update replaced disk cache")
			}
		})
	}
}

func TestLoadReturnsDeepCopy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testBody))
	}))
	defer server.Close()
	client := NewWithOptions(Options{URL: server.URL, RefreshInterval: time.Hour})

	got, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	model := got["test-provider"]["test-model"]
	model.Name = "mutated"
	model.Input[0] = "mutated"
	model.AdditionalProperties["futureField"][0] = 'x'
	got["test-provider"]["test-model"] = model

	again, err := client.Load(context.Background())
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	untouched := again["test-provider"]["test-model"]
	if untouched.Name != "Test Model" || untouched.Input[0] != "text" || strings.Contains(string(untouched.AdditionalProperties["futureField"]), "mutated") {
		t.Fatalf("caller mutation leaked into cached catalog: %+v", untouched)
	}
}

func TestFailedFetchRetryIsThrottledAndCancellationWinsOverCache(t *testing.T) {
	var requests atomic.Int32
	requestStarted := make(chan struct{}, 1)
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			select {
			case requestStarted <- struct{}{}:
			default:
			}
			<-releaseHandler
		}
		http.Error(w, "offline", http.StatusBadGateway)
	}))
	defer server.Close()
	defer close(releaseHandler)

	client := NewWithOptions(Options{URL: server.URL, RefreshInterval: time.Hour})
	client.current = &cacheRecord{
		Catalog:   Catalog{"stale-provider": {"stale-model": {ID: "stale-model", Name: "Stale"}}},
		Body:      []byte(`{"stale-provider":{"stale-model":{"id":"stale-model","name":"Stale"}}}`),
		FetchedAt: time.Now().Add(-2 * time.Hour),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Load(ctx)
		done <- err
	}()
	<-requestStarted
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("canceled load returned %v, want context.Canceled", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("canceled request count = %d, want 1", requests.Load())
	}
	// A non-canceled failure should keep the stale catalog usable without
	// waiting through another network timeout on each subsequent call.
	client.retryAt = time.Time{}
	client.current.FetchedAt = time.Now().Add(-2 * time.Hour)
	_, _ = client.Load(context.Background())
	before := requests.Load()
	if _, err := client.Load(context.Background()); err != nil {
		t.Fatalf("load during retry throttle: %v", err)
	}
	if requests.Load() != before {
		t.Fatalf("retry throttle allowed another network request: before=%d after=%d", before, requests.Load())
	}
}

func TestConcurrentLoadSharesOneRefresh(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(testBody))
	}))
	defer server.Close()
	client := NewWithOptions(Options{URL: server.URL, RefreshInterval: time.Hour})

	const callers = 12
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Load(context.Background())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent load: %v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("concurrent callers made %d requests, want 1", requests.Load())
	}
}

func TestLoadHonorsCanceledContext(t *testing.T) {
	client := NewWithOptions(Options{URL: "http://127.0.0.1:1"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Load(ctx); err == nil {
		t.Fatal("Load with canceled context succeeded")
	}
}
