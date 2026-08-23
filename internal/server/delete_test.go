package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"pi-web/internal/sessions"
)

func TestHandleDeleteSessionRemovesFile(t *testing.T) {
	root := t.TempDir()
	path := writeSessionFile(t, root, "test-project", "session.jsonl")
	s := &Server{sessionsDir: root, cache: sessions.NewCache()}

	req := httptest.NewRequest(http.MethodPost, "/api/delete-session?id=session.jsonl", nil)
	w := httptest.NewRecorder()
	s.handleDeleteSession(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true {
		t.Fatalf("payload = %#v", payload)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err = %v", err)
	}
	if _, err := sessions.ResolveByID(root, "session.jsonl"); err == nil {
		t.Fatal("expected ResolveByID to fail after delete")
	}
}

func TestHandleDeleteSessionRejectsGet(t *testing.T) {
	s := &Server{sessionsDir: t.TempDir(), cache: sessions.NewCache()}
	req := httptest.NewRequest(http.MethodGet, "/api/delete-session?id=session.jsonl", nil)
	w := httptest.NewRecorder()
	s.handleDeleteSession(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}
