// Package schemas contains the validation suite for every JSON Schema file
// (*.schema.json) published from this repository.
//
// The suite is split across three files:
//
//   - style.go      — syntax and style rules applied to the raw file bytes
//   - metaschema.go — validation of each schema against its meta-schema chain
//   - loader.go     — a caching HTTP loader shared by the meta-schema checks
package schemas

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// maxResponseBytes caps how much of a remote schema response is read,
// protecting the suite from a misbehaving server.
const maxResponseBytes = 10 << 20 // 10 MiB

// cachingLoader downloads JSON documents over HTTP(S) and caches the decoded
// result in memory, so each URL is fetched at most once per test run. It
// implements jsonschema.URLLoader.
type cachingLoader struct {
	client *http.Client

	mu    sync.Mutex
	cache map[string]any
}

// Interface guard: the compiler routes remote references through this type.
var _ jsonschema.URLLoader = (*cachingLoader)(nil)

func newCachingLoader() *cachingLoader {
	return &cachingLoader{
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  make(map[string]any),
	}
}

// Load returns the decoded JSON document at url, fetching it on first use.
// The mutex is held for the duration of the fetch; this serializes
// concurrent requests, a deliberate trade-off that guarantees a URL is never
// downloaded twice.
func (l *cachingLoader) Load(url string) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if doc, ok := l.cache[url]; ok {
		return doc, nil
	}
	doc, err := l.fetch(url)
	if err != nil {
		return nil, err
	}
	l.cache[url] = doc
	return doc, nil
}

// fetch performs the HTTP GET and decodes the response body as JSON.
func (l *cachingLoader) fetch(url string) (any, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/schema+json, application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status)
	}
	doc, err := jsonschema.UnmarshalJSON(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", url, err)
	}
	return doc, nil
}
