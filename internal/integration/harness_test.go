package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"pi-web/internal/auth"
	"pi-web/internal/rpc"
	"pi-web/internal/server"
	"pi-web/internal/sessions"
	"pi-web/internal/workers"
)

// harness wires a real *server.Server and *workers.Manager together with
// fake workers so integration scenarios run in milliseconds.
type harness struct {
	t           *testing.T
	server      *server.Server
	manager     *workers.Manager
	sessionsDir string
	agentDir    string
	mux         *http.ServeMux

	mu      sync.Mutex
	workers map[string]*fakeWorker // sessionID -> fakeWorker
}

// newHarness builds a test harness. Pass ttl=0 to disable the reaper, or a
// small duration for idle-reaping tests.
func newHarness(t *testing.T, ttl time.Duration) *harness {
	t.Helper()
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	agentDir := filepath.Join(root, "agent")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &harness{
		t:           t,
		sessionsDir: sessionsDir,
		agentDir:    agentDir,
		workers:     make(map[string]*fakeWorker),
		mux:         http.NewServeMux(),
	}

	var srv *server.Server
	manager := workers.NewManagerWithTTL(func(sessionID, sessionPath string) (workers.ChatWorker, error) {
		fw := newFakeWorker(func(preview rpc.StreamPreview) {
			if srv != nil {
				srv.BroadcastChatPreview(sessionID, preview)
			}
		})
		h.mu.Lock()
		h.workers[sessionID] = fw
		h.mu.Unlock()
		return fw, nil
	}, ttl)

	// Allow any host: the integration client dials the httptest server with a
	// non-loopback Host header, which the merged upstream host-check would
	// otherwise reject with 403 "unrecognized host".
	testAuth := auth.New("")
	testAuth.AllowAnyHost()
	var err error
	srv, err = server.New(server.Deps{
		AgentDir:            agentDir,
		SessionsDir:         sessionsDir,
		Auth:                testAuth,
		ChatSender:          manager,
		Cache:               sessions.NewCache(),
		RenderExportSession: func(s sessions.Session, theme string) string { return "" },
		Models:              func(ctx context.Context) (json.RawMessage, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	srv.Register(h.mux)
	h.server = srv
	h.manager = manager

	t.Cleanup(func() {
		srv.Shutdown()
		manager.Close()
	})

	return h
}

// getWorker returns the fakeWorker for a sessionID. Panics if not found.
func (h *harness) getWorker(sessionID string) *fakeWorker {
	h.mu.Lock()
	defer h.mu.Unlock()
	w, ok := h.workers[sessionID]
	if !ok {
		h.t.Fatalf("no fakeWorker for session %q", sessionID)
	}
	return w
}

// writeSessionFile creates a minimal session JSONL file and returns its path.
func (h *harness) writeSessionFile(name, project string) string {
	h.t.Helper()
	dir := filepath.Join(h.sessionsDir, project)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatal(err)
	}
	cwd := filepath.Join(dir, "cwd")
	if err := os.MkdirAll(cwd, 0755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	content := `{"type":"session","version":3,"id":"sid","timestamp":"2026-05-06T00:00:00.000Z","cwd":"` +
		filepath.ToSlash(cwd) + `"}` + "\n" +
		`{"type":"message","id":"aaaaaaaa","parentId":null,"timestamp":"2026-05-06T00:00:01.000Z","message":{"role":"user","content":"hello","timestamp":1778025601000}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		h.t.Fatal(err)
	}
	return path
}

// syncRecorder wraps httptest.ResponseRecorder so the handler goroutine's
// writes and the test goroutine's reads of the body do not race under -race.
type syncRecorder struct {
	*httptest.ResponseRecorder
	mu  sync.Mutex
	buf bytes.Buffer
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (s *syncRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitForBody polls until the recorder's body contains want or times out.
func waitForBody(t *testing.T, rec *syncRecorder, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.body(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q in body:\n%s", want, rec.body())
}

// sseSubscriber drives an SSE connection via HTTP and collects events.
type sseSubscriber struct {
	rec    *syncRecorder
	cancel context.CancelFunc
	done   chan struct{}
}

// subscribeSSE starts an SSE subscriber for the given session topic.
// Call sub.close() to disconnect.
func (h *harness) subscribeSSE(sessID string) *sseSubscriber {
	h.t.Helper()
	rec := newSyncRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events?id="+sessID, nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		h.mux.ServeHTTP(rec, req)
		close(done)
	}()

	return &sseSubscriber{rec: rec, cancel: cancel, done: done}
}

// close disconnects the SSE subscriber.
func (s *sseSubscriber) close() {
	s.cancel()
	<-s.done
}

// contains checks if the captured body contains the given string.
func (s *sseSubscriber) contains(want string) bool {
	return strings.Contains(s.rec.body(), want)
}

// waitFor blocks until the body contains want or times out.
func (s *sseSubscriber) waitFor(t *testing.T, want string) {
	t.Helper()
	waitForBody(t, s.rec, want)
}
