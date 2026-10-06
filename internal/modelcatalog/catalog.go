// Package modelcatalog fetches and caches Pi's public model catalog.
package modelcatalog

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultURL         = "https://pi.dev/api/models"
	defaultTTL         = 5 * time.Minute
	defaultHTTPTimeout = 8 * time.Second
	cacheVersion       = 1
	maxResponseBytes   = 16 << 20
)

//go:embed snapshot.json
var snapshotFS embed.FS

// Catalog maps provider IDs to model IDs to Pi model definitions.
type Catalog map[string]map[string]Model

// Model contains Pi's model fields used by clients. AdditionalProperties keeps
// metadata introduced by newer catalog versions available to callers.
type Model struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	API                  string                     `json:"api"`
	Provider             string                     `json:"provider"`
	BaseURL              string                     `json:"baseUrl"`
	Reasoning            bool                       `json:"reasoning"`
	Input                []string                   `json:"input"`
	Cost                 Cost                       `json:"cost"`
	ContextWindow        int64                      `json:"contextWindow"`
	MaxTokens            int64                      `json:"maxTokens"`
	Type                 string                     `json:"type,omitempty"`
	ThinkingLevelMap     map[string]json.RawMessage `json:"thinkingLevelMap,omitempty"`
	Compat               json.RawMessage            `json:"compat,omitempty"`
	Headers              map[string]string          `json:"headers,omitempty"`
	Lab                  string                     `json:"lab,omitempty"`
	Enabled              *bool                      `json:"enabled,omitempty"`
	Providers            json.RawMessage            `json:"providers,omitempty"`
	PromptCache          json.RawMessage            `json:"promptCache,omitempty"`
	InputLimits          json.RawMessage            `json:"inputLimits,omitempty"`
	AdditionalProperties map[string]json.RawMessage `json:"-"`
}

// Cost values are USD per one million tokens.
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// UnmarshalJSON captures fields not yet known to this package.
func (m *Model) UnmarshalJSON(data []byte) error {
	type modelAlias Model
	var known modelAlias
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	*m = Model(known)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range knownModelFields {
		delete(fields, name)
	}
	if len(fields) > 0 {
		m.AdditionalProperties = fields
	}
	return nil
}

// MarshalJSON writes known and newer catalog fields without losing either.
func (m Model) MarshalJSON() ([]byte, error) {
	type modelAlias Model
	b, err := json.Marshal(modelAlias(m))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	for name, value := range m.AdditionalProperties {
		if _, exists := fields[name]; !exists {
			fields[name] = value
		}
	}
	return json.Marshal(fields)
}

var knownModelFields = []string{
	"id", "name", "api", "provider", "baseUrl", "reasoning", "input", "cost",
	"contextWindow", "maxTokens", "type", "thinkingLevelMap", "compat", "headers",
	"lab", "enabled", "providers", "promptCache", "inputLimits",
}

// Options configures a Client. Zero values select Pi's public catalog URL,
// an eight-second HTTP timeout, and a five-minute refresh interval.
type Options struct {
	CachePath       string
	URL             string
	HTTPClient      *http.Client
	RefreshInterval time.Duration
	Logf            func(format string, args ...any)
}

// Client loads Pi model catalogs, revalidating stale snapshots with ETags.
type Client struct {
	url       string
	cachePath string
	http      *http.Client
	ttl       time.Duration
	logf      func(format string, args ...any)

	mu      sync.Mutex
	current *cacheRecord
	retryAt time.Time
}

// New creates a client using the Pi catalog and the supplied disk cache path.
func New(cachePath string) *Client {
	return NewWithOptions(Options{CachePath: cachePath})
}

// NewWithOptions creates a client with optional endpoint, transport, TTL, and logger.
func NewWithOptions(options Options) *Client {
	if options.URL == "" {
		options.URL = defaultURL
	}
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = defaultTTL
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		url:       options.URL,
		cachePath: options.CachePath,
		http:      options.HTTPClient,
		ttl:       options.RefreshInterval,
		logf:      options.Logf,
	}
}

// Load returns the newest available catalog. It uses a fresh memory or disk
// cache first, refreshes expired entries in the background-free request path,
// and falls back to stale cache or the embedded snapshot when the network fails.
func (c *Client) Load(ctx context.Context) (Catalog, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.current == nil {
		c.current = c.readCache()
	}
	if c.current != nil && time.Since(c.current.FetchedAt) < c.ttl {
		return cloneCatalog(c.current.Catalog), nil
	}
	if c.current != nil && time.Now().Before(c.retryAt) {
		return cloneCatalog(c.current.Catalog), nil
	}

	updated, err := c.fetch(ctx, c.current)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err == nil {
		c.current = updated
		c.retryAt = time.Time{}
		c.writeCache(updated)
		return cloneCatalog(updated.Catalog), nil
	}
	c.retryAt = time.Now().Add(c.ttl)
	if c.current != nil {
		c.log("Pi model catalog refresh failed; using cached catalog: %v", err)
		return cloneCatalog(c.current.Catalog), nil
	}

	snapshot, snapshotErr := embeddedCatalog()
	if snapshotErr == nil {
		c.current = &cacheRecord{Catalog: snapshot, Body: nil, FetchedAt: time.Now()}
		c.log("Pi model catalog refresh failed; using embedded snapshot: %v", err)
		return cloneCatalog(snapshot), nil
	}
	return nil, fmt.Errorf("refresh Pi model catalog: %w (embedded snapshot: %v)", err, snapshotErr)
}

func (c *Client) fetch(ctx context.Context, previous *cacheRecord) (*cacheRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if previous != nil {
		if previous.ETag != "" {
			req.Header.Set("If-None-Match", previous.ETag)
		}
		if previous.LastModified != "" {
			req.Header.Set("If-Modified-Since", previous.LastModified)
		}
	}

	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request catalog: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		if previous == nil {
			return nil, errors.New("catalog returned 304 without a cached body")
		}
		updated := *previous
		updated.FetchedAt = time.Now()
		if value := response.Header.Get("ETag"); value != "" {
			updated.ETag = value
		}
		if value := response.Header.Get("Last-Modified"); value != "" {
			updated.LastModified = value
		}
		if value := response.Header.Get("x-pi-model-catalog-revision"); value != "" {
			updated.Revision = value
		}
		if value := response.Header.Get("x-pi-model-catalog-minimum-version"); value != "" {
			updated.MinimumVersion = value
		}
		return &updated, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("catalog returned HTTP %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read catalog response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("catalog response exceeds %d bytes", maxResponseBytes)
	}
	catalog, err := parseCatalog(body)
	if err != nil {
		return nil, fmt.Errorf("decode catalog response: %w", err)
	}
	return &cacheRecord{
		Catalog:        catalog,
		Body:           body,
		ETag:           response.Header.Get("ETag"),
		LastModified:   response.Header.Get("Last-Modified"),
		Revision:       response.Header.Get("x-pi-model-catalog-revision"),
		MinimumVersion: response.Header.Get("x-pi-model-catalog-minimum-version"),
		FetchedAt:      time.Now(),
	}, nil
}

func parseCatalog(body []byte) (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}
	if len(catalog) == 0 {
		return nil, errors.New("catalog has no providers")
	}
	modelCount := 0
	for providerID, models := range catalog {
		if strings.TrimSpace(providerID) == "" {
			return nil, errors.New("catalog contains an empty provider ID")
		}
		for modelID, model := range models {
			modelCount++
			if strings.TrimSpace(modelID) == "" || strings.TrimSpace(model.ID) == "" {
				return nil, fmt.Errorf("catalog provider %q has a model without an ID", providerID)
			}
			if model.ID != modelID {
				return nil, fmt.Errorf("catalog provider %q model key %q disagrees with model ID %q", providerID, modelID, model.ID)
			}
			if model.Provider != "" && model.Provider != providerID {
				return nil, fmt.Errorf("catalog provider %q model %q declares provider %q", providerID, modelID, model.Provider)
			}
		}
	}
	if modelCount == 0 {
		return nil, errors.New("catalog has no models")
	}
	return catalog, nil
}

func embeddedCatalog() (Catalog, error) {
	body, err := snapshotFS.ReadFile("snapshot.json")
	if err != nil {
		return nil, err
	}
	return parseCatalog(body)
}

type cacheRecord struct {
	Catalog        Catalog
	Body           json.RawMessage
	ETag           string
	LastModified   string
	Revision       string
	MinimumVersion string
	FetchedAt      time.Time
}

type diskCache struct {
	Version        int             `json:"version"`
	Body           json.RawMessage `json:"body"`
	ETag           string          `json:"etag,omitempty"`
	LastModified   string          `json:"lastModified,omitempty"`
	Revision       string          `json:"revision,omitempty"`
	MinimumVersion string          `json:"minimumVersion,omitempty"`
	FetchedAt      time.Time       `json:"fetchedAt"`
}

func (c *Client) readCache() *cacheRecord {
	if c.cachePath == "" {
		return nil
	}
	body, err := os.ReadFile(c.cachePath)
	if err != nil {
		return nil
	}
	var stored diskCache
	if err := json.Unmarshal(body, &stored); err != nil || stored.Version != cacheVersion || len(stored.Body) == 0 || stored.FetchedAt.IsZero() {
		return nil
	}
	catalog, err := parseCatalog(stored.Body)
	if err != nil {
		return nil
	}
	return &cacheRecord{
		Catalog:        catalog,
		Body:           stored.Body,
		ETag:           stored.ETag,
		LastModified:   stored.LastModified,
		Revision:       stored.Revision,
		MinimumVersion: stored.MinimumVersion,
		FetchedAt:      stored.FetchedAt,
	}
}

func (c *Client) writeCache(record *cacheRecord) {
	if c.cachePath == "" || len(record.Body) == 0 {
		return
	}
	data, err := json.Marshal(diskCache{
		Version:        cacheVersion,
		Body:           record.Body,
		ETag:           record.ETag,
		LastModified:   record.LastModified,
		Revision:       record.Revision,
		MinimumVersion: record.MinimumVersion,
		FetchedAt:      record.FetchedAt,
	})
	if err != nil {
		c.log("encode Pi model catalog cache: %v", err)
		return
	}
	directory := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		c.log("create Pi model catalog cache directory: %v", err)
		return
	}
	file, err := os.CreateTemp(directory, ".pi-model-catalog-*.tmp")
	if err != nil {
		c.log("create Pi model catalog cache temp file: %v", err)
		return
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		c.log("set Pi model catalog cache permissions: %v", err)
		return
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		c.log("write Pi model catalog cache: %v", err)
		return
	}
	if err := file.Sync(); err != nil {
		file.Close()
		c.log("sync Pi model catalog cache: %v", err)
		return
	}
	if err := file.Close(); err != nil {
		c.log("close Pi model catalog cache: %v", err)
		return
	}
	if err := os.Rename(tempPath, c.cachePath); err != nil {
		c.log("replace Pi model catalog cache: %v", err)
	}
}

func cloneCatalog(catalog Catalog) Catalog {
	if catalog == nil {
		return nil
	}
	body, err := json.Marshal(catalog)
	if err != nil {
		return catalog
	}
	clone, err := parseCatalog(body)
	if err != nil {
		return catalog
	}
	return clone
}

func (c *Client) log(format string, args ...any) {
	if c.logf != nil {
		c.logf(format, args...)
	}
}
