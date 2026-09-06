package planusage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func writeAuth(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadOAuthToken(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour).UnixMilli()
	past := now.Add(-time.Hour).UnixMilli()
	cases := []struct {
		name, body string
		wantErr    bool
	}{
		{"valid", `{"anthropic":{"type":"oauth","access":"tok","refresh":"r","expires":` + itoa(future) + `}}`, false},
		{"expired", `{"anthropic":{"type":"oauth","access":"tok","refresh":"r","expires":` + itoa(past) + `}}`, true},
		{"api key only", `{"anthropic":{"type":"api_key","key":"sk"}}`, true},
		{"missing provider", `{"openai":{"type":"oauth","access":"tok","expires":` + itoa(future) + `}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeAuth(t, tc.body)
			tok, err := ReadOAuthToken(path, "anthropic", now)
			if tc.wantErr {
				if !errors.Is(err, ErrNoCredential) {
					t.Fatalf("want ErrNoCredential, got %v (tok=%q)", err, tok)
				}
				return
			}
			if err != nil || tok != "tok" {
				t.Fatalf("got tok=%q err=%v", tok, err)
			}
		})
	}
	if _, err := ReadOAuthToken(filepath.Join(t.TempDir(), "missing.json"), "anthropic", now); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("missing file should be ErrNoCredential, got %v", err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestAnthropicFetchParsesWindows(t *testing.T) {
	var gotAuth, gotBeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"five_hour": {"utilization": 33.0, "resets_at": "2030-01-01T07:00:00Z"},
			"seven_day": {"utilization": 13.0, "resets_at": "2030-01-07T00:59:59Z"},
			"seven_day_opus": null,
			"extra_usage": {"is_enabled": false}
		}`))
	}))
	defer srv.Close()
	a := NewAnthropic()
	a.Endpoint = srv.URL
	usage, err := a.Fetch(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" || gotBeta == "" {
		t.Fatalf("headers: auth=%q beta=%q", gotAuth, gotBeta)
	}
	if len(usage.Windows) != 2 {
		t.Fatalf("want 2 windows, got %#v", usage.Windows)
	}
	if usage.Windows[0].Key != "five_hour" || usage.Windows[0].Utilization != 33 || usage.Windows[0].ResetsAt == nil {
		t.Fatalf("five_hour window: %#v", usage.Windows[0])
	}
	if usage.Windows[1].Key != "seven_day" {
		t.Fatalf("second window: %#v", usage.Windows[1])
	}
}

func TestAnthropicFetch429IsRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	a := NewAnthropic()
	a.Endpoint = srv.URL
	if _, err := a.Fetch(context.Background(), "tok"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
}

type fakeProvider struct {
	calls int
	err   error
}

func (f *fakeProvider) ID() string    { return "anthropic" }
func (f *fakeProvider) Label() string { return "Fake" }
func (f *fakeProvider) Fetch(ctx context.Context, token string) (Usage, error) {
	f.calls++
	if f.err != nil {
		return Usage{}, f.err
	}
	return Usage{Windows: []Window{{Key: "w", Label: "W", Utilization: 1}}}, nil
}

func TestServiceCachesAndBacksOff(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	path := writeAuth(t, `{"anthropic":{"type":"oauth","access":"tok","refresh":"r","expires":`+itoa(now.Add(24*time.Hour).UnixMilli())+`}}`)
	fp := &fakeProvider{}
	svc := NewService(path, fp)
	svc.Now = func() time.Time { return now }

	if got := svc.Snapshot(context.Background()); len(got) != 1 || got[0].Provider != "anthropic" || got[0].Label != "Fake" {
		t.Fatalf("first snapshot: %#v", got)
	}
	svc.Snapshot(context.Background())
	if fp.calls != 1 {
		t.Fatalf("second call within TTL must hit cache, calls=%d", fp.calls)
	}

	// TTL passes, provider now rate limits → keep last good data, back off.
	now = now.Add(2 * time.Minute)
	fp.err = ErrRateLimited
	if got := svc.Snapshot(context.Background()); len(got) != 1 {
		t.Fatalf("stale-but-good data should be served through a 429: %#v", got)
	}
	if fp.calls != 2 {
		t.Fatalf("calls=%d", fp.calls)
	}
	now = now.Add(2 * time.Minute) // inside the 5-minute backoff
	svc.Snapshot(context.Background())
	if fp.calls != 2 {
		t.Fatalf("must not retry inside backoff, calls=%d", fp.calls)
	}
}

func TestServiceOmitsProvidersWithoutCredential(t *testing.T) {
	path := writeAuth(t, `{}`)
	fp := &fakeProvider{}
	svc := NewService(path, fp)
	if got := svc.Snapshot(context.Background()); len(got) != 0 {
		t.Fatalf("want empty, got %#v", got)
	}
	if fp.calls != 0 {
		t.Fatalf("provider must not be called without a token")
	}
}
