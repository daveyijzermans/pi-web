package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pi-web/internal/sessions"
)

// writeToolUseSession writes a session whose tail is an assistant tool-use
// entry — i.e. a pi process actively mid-turn.
func writeToolUseSession(t *testing.T, root, project, name string) string {
	t.Helper()
	dir := filepath.Join(root, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	content := `{"type":"session","version":3,"id":"sid","cwd":"/x"}` + "\n" +
		`{"type":"message","message":{"role":"user","content":"go"}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","stopReason":"toolUse"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func chatMultipart(t *testing.T, msg string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("message", msg)
	_ = mw.Close()
	return &body, mw.FormDataContentType()
}

func TestWorkerStatusRunningForTerminalToolUse(t *testing.T) {
	root := t.TempDir()
	writeToolUseSession(t, root, "proj", "session.jsonl")
	s := &Server{sessionsDir: root, chatSender: &fakeSender{}, now: time.Now}

	req := httptest.NewRequest(http.MethodGet, "/api/worker-status?id=session.jsonl", nil)
	w := httptest.NewRecorder()
	s.handleWorkerStatus(w, req)

	if got := w.Body.String(); got == "" || !bytes.Contains([]byte(got), []byte(`"state":"running"`)) {
		t.Fatalf("body = %q, want state=running for an active terminal turn", got)
	}
}

func TestTerminalTurnRetiredWhenStale(t *testing.T) {
	root := t.TempDir()
	path := writeToolUseSession(t, root, "proj", "session.jsonl")
	old := time.Now().Add(-2 * terminalTurnStaleWindow)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	s := &Server{sessionsDir: root, chatSender: &fakeSender{}, now: time.Now}
	if s.hasActiveTerminalTurn("session.jsonl") {
		t.Fatal("stale tool-use tail must not count as an active terminal turn")
	}
}

func TestHandleChatBlocksTerminalTurn(t *testing.T) {
	root := t.TempDir()
	writeToolUseSession(t, root, "proj", "session.jsonl")
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s := &Server{sessionsDir: root, chatSender: fake, now: time.Now}

	body, ctype := chatMultipart(t, "hi")
	req := httptest.NewRequest(http.MethodPost, "/api/chat?id=session.jsonl", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	s.handleChat(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 while a terminal turn is active", w.Code)
	}
	select {
	case <-fake.sendCh:
		t.Fatal("Send must not run when the turn is blocked")
	case <-time.After(50 * time.Millisecond):
	}
}

// writeToolUseSessionCwd is writeToolUseSession with a real cwd so the session
// stays chat-available (an out-of-tree cwd disables chat before any turn gate).
func writeToolUseSessionCwd(t *testing.T, root, name, cwd string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session","version":3,"id":"sid","cwd":"` + cwd + `"}` + "\n" +
		`{"type":"message","message":{"role":"user","content":"go"}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","stopReason":"toolUse"}}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "proj", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOrphanedWebTurnUnblocksAndReportsIdle(t *testing.T) {
	root := t.TempDir()
	writeToolUseSessionCwd(t, root, "session.jsonl", t.TempDir())
	resolved, err := sessions.ResolveByID(root, "session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	id := resolved.Session.ID

	agentDir := t.TempDir()
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s := &Server{
		sessionsDir:      root,
		agentDir:         agentDir,
		chatSender:       fake,
		orphanedWebTurns: make(map[string]struct{}),
		now:              time.Now,
	}

	// Sanity: without the marker the active tool-use tail blocks a web turn.
	if reason := s.chatBusyReason(id); reason == "" {
		t.Fatal("pre-marker: active tail should block")
	}

	// Simulate a previous instance that died mid-web-turn: its marker survives.
	s.markWebTurnActive(id)
	s.loadOrphanedWebTurns()

	if reason := s.chatBusyReason(id); reason != "" {
		t.Fatalf("orphaned web turn should not block, got %q", reason)
	}
	if s.computeRunningStatus(id) {
		t.Fatal("orphaned web turn must report idle, not running")
	}

	// A fresh Send goes through (202) and actually dispatches.
	body, ctype := chatMultipart(t, "continue")
	req := httptest.NewRequest(http.MethodPost, "/api/chat?id=session.jsonl", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	s.handleChat(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 for an orphaned web turn", w.Code)
	}
	select {
	case <-fake.sendCh:
	case <-time.After(time.Second):
		t.Fatal("Send should run for an orphaned web turn")
	}
}

func TestOrphanedWebTurnFlagExpires(t *testing.T) {
	root := t.TempDir()
	writeToolUseSessionCwd(t, root, "session.jsonl", t.TempDir())
	resolved, err := sessions.ResolveByID(root, "session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	id := resolved.Session.ID

	now := time.Now()
	s := &Server{
		sessionsDir:      root,
		agentDir:         t.TempDir(),
		chatSender:       &fakeSender{},
		orphanedWebTurns: make(map[string]struct{}),
		now:              func() time.Time { return now },
	}
	s.markWebTurnActive(id)
	s.loadOrphanedWebTurns()

	if !s.isOrphanedWebTurn(id) {
		t.Fatal("flag should hold immediately after load")
	}
	// Past the stale window the flag expires so normal terminal detection
	// resumes and a genuine later terminal turn is never masked.
	now = now.Add(2 * terminalTurnStaleWindow)
	if s.isOrphanedWebTurn(id) {
		t.Fatal("flag must expire after terminalTurnStaleWindow")
	}
}

func TestHandleChatBlocksWhileCompacting(t *testing.T) {
	root := t.TempDir()
	// tail=user session: not a terminal turn, so only the compaction blocks it.
	writeSessionFile(t, root, "proj", "session.jsonl")
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s := &Server{sessionsDir: root, chatSender: fake, now: time.Now}
	s.setCompacting("session.jsonl", true)

	body, ctype := chatMultipart(t, "hi")
	req := httptest.NewRequest(http.MethodPost, "/api/chat?id=session.jsonl", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	s.handleChat(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 while compacting", w.Code)
	}

	// Clearing the compaction unblocks sending.
	s.setCompacting("session.jsonl", false)
	body2, ctype2 := chatMultipart(t, "hi")
	req2 := httptest.NewRequest(http.MethodPost, "/api/chat?id=session.jsonl", body2)
	req2.Header.Set("Content-Type", ctype2)
	w2 := httptest.NewRecorder()
	s.handleChat(w2, req2)
	if w2.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 after compaction cleared", w2.Code)
	}
}

func TestWorkerStatusCompactingFlag(t *testing.T) {
	root := t.TempDir()
	writeSessionFile(t, root, "proj", "session.jsonl")
	s := &Server{sessionsDir: root, chatSender: &fakeSender{}, now: time.Now}
	s.setCompacting("session.jsonl", true)

	req := httptest.NewRequest(http.MethodGet, "/api/worker-status?id=session.jsonl", nil)
	w := httptest.NewRecorder()
	s.handleWorkerStatus(w, req)

	body := w.Body.String()
	if !bytes.Contains([]byte(body), []byte(`"state":"running"`)) || !bytes.Contains([]byte(body), []byte(`"compacting":true`)) {
		t.Fatalf("body = %q, want running + compacting:true", body)
	}
}
