// Package planusage reports subscription-plan rate-limit windows (e.g. a
// rolling 5-hour window and a weekly window) for providers pi is logged into
// via OAuth. Provider adapters read the token pi already stores and call the
// provider's usage endpoint; pi-web never refreshes or writes credentials —
// an expired token simply means "no data until pi refreshes it".
//
// The shape is deliberately provider-neutral: every provider exposes zero or
// more Windows, each with a utilization percentage and a reset time. The UI
// renders whatever comes back and offers "when the window resets" as a
// scheduling preset without knowing which provider or window it is.
package planusage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Window is one rate-limit bucket of a plan.
type Window struct {
	Key         string     `json:"key"`
	Label       string     `json:"label"`
	Utilization float64    `json:"utilization"` // percent, 0–100
	ResetsAt    *time.Time `json:"resetsAt,omitempty"`
}

// Usage is the snapshot for one provider.
type Usage struct {
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	Windows   []Window  `json:"windows"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// Provider fetches usage for one subscription provider.
type Provider interface {
	ID() string
	Label() string
	Fetch(ctx context.Context, token string) (Usage, error)
}

// ErrNoCredential means pi has no usable OAuth token for the provider.
var ErrNoCredential = errors.New("no oauth credential")

// oauthCredential mirrors the credential entry pi stores per provider id.
type oauthCredential struct {
	Type    string `json:"type"`
	Access  string `json:"access"`
	Expires int64  `json:"expires"` // unix ms
}

// ReadOAuthToken returns the current access token pi stores for providerID.
// Expired tokens are reported as ErrNoCredential; pi refreshes them itself on
// its next request, after which we pick up the new value.
func ReadOAuthToken(authPath, providerID string, now time.Time) (string, error) {
	raw, err := os.ReadFile(authPath)
	if err != nil {
		return "", ErrNoCredential
	}
	var creds map[string]oauthCredential
	if err := json.Unmarshal(raw, &creds); err != nil {
		return "", fmt.Errorf("parse auth file: %w", err)
	}
	cred, ok := creds[providerID]
	if !ok || cred.Type != "oauth" || cred.Access == "" {
		return "", ErrNoCredential
	}
	if cred.Expires > 0 && time.UnixMilli(cred.Expires).Before(now) {
		return "", ErrNoCredential
	}
	return cred.Access, nil
}

// Service polls every registered provider with caching and rate-limit backoff.
type Service struct {
	AuthPath  string
	Providers []Provider
	Now       func() time.Time
	// CacheTTL bounds how often a provider is re-queried; BackoffTTL is the
	// pause after a 429 or transport error.
	CacheTTL   time.Duration
	BackoffTTL time.Duration

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	usage    Usage
	ok       bool
	nextTry  time.Time
	lastSeen time.Time
}

// ErrRateLimited is returned by adapters on HTTP 429 so the service backs off.
var ErrRateLimited = errors.New("rate limited")

func NewService(authPath string, providers ...Provider) *Service {
	return &Service{
		AuthPath:   authPath,
		Providers:  providers,
		Now:        time.Now,
		CacheTTL:   60 * time.Second,
		BackoffTTL: 5 * time.Minute,
		cache:      map[string]cacheEntry{},
	}
}

// Snapshot returns the latest usage for every provider that has a usable
// credential and a successful fetch. Providers without data are omitted.
func (s *Service) Snapshot(ctx context.Context) []Usage {
	now := s.Now()
	var out []Usage
	for _, p := range s.Providers {
		if usage, ok := s.fetchCached(ctx, p, now); ok {
			out = append(out, usage)
		}
	}
	if out == nil {
		out = []Usage{}
	}
	return out
}

func (s *Service) fetchCached(ctx context.Context, p Provider, now time.Time) (Usage, bool) {
	s.mu.Lock()
	entry, cached := s.cache[p.ID()]
	s.mu.Unlock()
	if cached && now.Before(entry.nextTry) {
		return entry.usage, entry.ok
	}
	token, err := ReadOAuthToken(s.AuthPath, p.ID(), now)
	if err != nil {
		s.store(p.ID(), cacheEntry{nextTry: now.Add(s.CacheTTL)})
		return Usage{}, false
	}
	usage, err := p.Fetch(ctx, token)
	if err != nil {
		ttl := s.BackoffTTL
		if !errors.Is(err, ErrRateLimited) {
			ttl = s.CacheTTL
		}
		// Keep serving the last good snapshot through a transient failure.
		next := cacheEntry{usage: entry.usage, ok: entry.ok, nextTry: now.Add(ttl), lastSeen: entry.lastSeen}
		s.store(p.ID(), next)
		return next.usage, next.ok
	}
	usage.Provider = p.ID()
	usage.Label = p.Label()
	usage.FetchedAt = now
	if usage.Windows == nil {
		usage.Windows = []Window{}
	}
	s.store(p.ID(), cacheEntry{usage: usage, ok: true, nextTry: now.Add(s.CacheTTL), lastSeen: now})
	return usage, true
}

func (s *Service) store(id string, entry cacheEntry) {
	s.mu.Lock()
	s.cache[id] = entry
	s.mu.Unlock()
}

// statusError turns a non-2xx response into an error the service understands.
func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	return fmt.Errorf("usage endpoint returned %d", resp.StatusCode)
}
