package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"pi-web/internal/chatqueue"

	_ "modernc.org/sqlite"
)

func newQueueTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		chatqueue.ItemsTableDDL,
		chatqueue.ItemsSessionIndexDDL,
		chatqueue.StateTableDDL,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	}
	return db
}

func writeQueueTestSession(t *testing.T, sessionsDir string) string {
	t.Helper()
	project := filepath.Join(sessionsDir, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "2026-06-22T01-00-00.000Z_q.jsonl"
	body := `{"type":"session","version":3,"id":"q","cwd":` + jsonString(project) + `}` + "\n"
	if err := os.WriteFile(filepath.Join(project, id), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return id
}

func newQueueServer(t *testing.T) (*Server, string) {
	t.Helper()
	db := newQueueTestDB(t)
	s := &Server{sessionsDir: t.TempDir(), agentDir: t.TempDir(), db: db, chatQueue: chatqueue.NewStore(db)}
	id := writeQueueTestSession(t, s.sessionsDir)
	return s, id
}

func decodeSnapshot(t *testing.T, w *httptest.ResponseRecorder) chatqueue.Snapshot {
	t.Helper()
	var snap chatqueue.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v\nbody=%s", err, w.Body.String())
	}
	return snap
}

func TestChatQueueAddThenList(t *testing.T) {
	s, id := newQueueServer(t)

	for _, msg := range []string{"hello", "world"} {
		body := `{"message":"` + msg + `"}`
		req := httptest.NewRequest(http.MethodPost,
			"/api/chat/queue?id="+id, strings.NewReader(body))
		w := httptest.NewRecorder()
		s.handleChatQueue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("POST %q: code=%d body=%s", msg, w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/queue?id="+id, nil)
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: code=%d", w.Code)
	}
	snap := decodeSnapshot(t, w)
	if len(snap.Items) != 2 {
		t.Fatalf("want 2 items, got %#v", snap.Items)
	}
	if snap.Items[0].Message != "hello" || snap.Items[1].Message != "world" {
		t.Fatalf("order wrong: %#v", snap.Items)
	}
	if snap.Items[0].Position >= snap.Items[1].Position {
		t.Fatalf("positions not monotonic: %#v", snap.Items)
	}
	if snap.Paused {
		t.Fatalf("default paused must be false")
	}
}

func TestChatQueueDelete(t *testing.T) {
	s, id := newQueueServer(t)
	for _, msg := range []string{"a", "b", "c"} {
		req := httptest.NewRequest(http.MethodPost,
			"/api/chat/queue?id="+id, strings.NewReader(`{"message":"`+msg+`"}`))
		s.handleChatQueue(httptest.NewRecorder(), req)
	}
	// Find b's position.
	getReq := httptest.NewRequest(http.MethodGet, "/api/chat/queue?id="+id, nil)
	getW := httptest.NewRecorder()
	s.handleChatQueue(getW, getReq)
	snap := decodeSnapshot(t, getW)
	var bPos int64
	for _, it := range snap.Items {
		if it.Message == "b" {
			bPos = it.Position
		}
	}
	if bPos == 0 {
		t.Fatalf("item b not found in %#v", snap.Items)
	}

	delURL := "/api/chat/queue?id=" + id + "&position=" + strconv.FormatInt(bPos, 10)
	delReq := httptest.NewRequest(http.MethodDelete, delURL, nil)
	delW := httptest.NewRecorder()
	s.handleChatQueue(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("DELETE: code=%d body=%s", delW.Code, delW.Body.String())
	}

	getW2 := httptest.NewRecorder()
	s.handleChatQueue(getW2, httptest.NewRequest(http.MethodGet, "/api/chat/queue?id="+id, nil))
	snap2 := decodeSnapshot(t, getW2)
	if len(snap2.Items) != 2 {
		t.Fatalf("want 2 items after delete, got %#v", snap2.Items)
	}
	for _, it := range snap2.Items {
		if it.Message == "b" {
			t.Fatalf("deleted item still present: %#v", snap2.Items)
		}
	}
}

func TestChatQueuePauseRoundtrip(t *testing.T) {
	s, id := newQueueServer(t)
	patchReq := httptest.NewRequest(http.MethodPatch,
		"/api/chat/queue?id="+id, strings.NewReader(`{"paused":true}`))
	patchW := httptest.NewRecorder()
	s.handleChatQueue(patchW, patchReq)
	if patchW.Code != http.StatusOK {
		t.Fatalf("PATCH: code=%d body=%s", patchW.Code, patchW.Body.String())
	}

	getW := httptest.NewRecorder()
	s.handleChatQueue(getW, httptest.NewRequest(http.MethodGet, "/api/chat/queue?id="+id, nil))
	snap := decodeSnapshot(t, getW)
	if !snap.Paused {
		t.Fatalf("expected paused=true after PATCH")
	}
}

func TestChatQueueRejectsEmptyMessage(t *testing.T) {
	s, id := newQueueServer(t)
	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/queue?id="+id, strings.NewReader(`{"message":"  "}`))
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChatQueueRejectsUnknownSession(t *testing.T) {
	s, _ := newQueueServer(t)
	// Well-formed id pattern that doesn't match any file on disk → 404.
	req := httptest.NewRequest(http.MethodGet,
		"/api/chat/queue?id=2099-01-01T00-00-00.000Z_unknown.jsonl", nil)
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChatQueueRejectsMissingPositionOnDelete(t *testing.T) {
	s, id := newQueueServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/chat/queue?id="+id, nil)
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// 1x1 transparent PNG — small but a real image so DetectContentType says image/png.
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func TestChatQueueMultipartPostStoresImagesAndSchedule(t *testing.T) {
	s, id := newQueueServer(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("message", "look at this")
	_ = mw.WriteField("displayText", "look at this")
	_ = mw.WriteField("notBefore", "2030-01-02T03:04:05Z")
	part, _ := mw.CreateFormFile("images", "shot.png")
	_, _ = part.Write(tinyPNG)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/chat/queue?id="+id, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST: code=%d body=%s", w.Code, w.Body.String())
	}
	var item chatqueue.Item
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ImageCount != 1 || len(item.Attachments) != 1 || !strings.HasSuffix(item.Attachments[0], ".png") {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.NotBefore == nil || !item.NotBefore.Equal(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("notBefore not parsed: %v", item.NotBefore)
	}
	if !strings.Contains(item.Message, "shot") {
		t.Fatalf("message should carry the attachment line, got %q", item.Message)
	}
	// The stored row keeps the inline image for the drainer.
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 || len(snap.Items[0].Images) != 1 || snap.Items[0].Images[0].MimeType != "image/png" {
		t.Fatalf("stored images: %#v", snap.Items)
	}
}

func TestChatQueueJSONPostRejectsBadNotBefore(t *testing.T) {
	s, id := newQueueServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/queue?id="+id,
		strings.NewReader(`{"message":"hi","notBefore":"tomorrow"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChatQueuePatchReschedulesItem(t *testing.T) {
	s, id := newQueueServer(t)
	later := time.Now().Add(time.Hour).UTC()
	item, err := s.chatQueue.AddItem(id, chatqueue.NewItem{Message: "later", NotBefore: &later})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"position":` + strconv.FormatInt(item.Position, 10) + `,"notBefore":""}`
	req := httptest.NewRequest(http.MethodPatch, "/api/chat/queue?id="+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleChatQueue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH: code=%d body=%s", w.Code, w.Body.String())
	}
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 || snap.Items[0].NotBefore != nil {
		t.Fatalf("schedule should be cleared: %#v", snap.Items)
	}
}
