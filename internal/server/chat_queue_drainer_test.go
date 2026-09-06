package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"pi-web/internal/chat"
	"pi-web/internal/chatqueue"
	"pi-web/internal/sessions"
	"pi-web/internal/workers"
)

func newDrainerServer(t *testing.T, sender ChatSender) (*Server, *queueDrainer, string) {
	t.Helper()
	db := newQueueTestDB(t)
	s := &Server{
		sessionsDir: t.TempDir(),
		db:          db,
		chatQueue:   chatqueue.NewStore(db),
		chatSender:  sender,
		now:         time.Now,
	}
	d := newQueueDrainer(s)
	s.queueDrainer = d
	id := writeQueueTestSession(t, s.sessionsDir)
	return s, d, id
}

func TestDrainerDispatchesNextItem(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s, d, id := newDrainerServer(t, fake)

	if _, err := s.chatQueue.Add(id, "first prompt", "first prompt"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	d.drainSession(id)

	select {
	case <-fake.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected Send within 2s")
	}

	sentID, _, req := fake.sentInfo()
	if sentID != id {
		t.Fatalf("Send sessionID=%q want %q", sentID, id)
	}
	if req.Message != "first prompt" {
		t.Fatalf("Send message=%q want %q", req.Message, "first prompt")
	}

	// PopHead removed the item, so the queue is now empty.
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 0 {
		t.Fatalf("queue should be empty after dispatch, got %#v", snap.Items)
	}
}

func TestDrainerSkipsWhenPaused(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s, d, id := newDrainerServer(t, fake)
	s.chatQueue.SetPaused(id, true)
	s.chatQueue.Add(id, "should not dispatch", "should not dispatch")

	d.drainSession(id)

	select {
	case <-fake.sendCh:
		t.Fatalf("Send should not have fired while paused")
	case <-time.After(150 * time.Millisecond):
	}

	// Item is still in the queue.
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 {
		t.Fatalf("expected item to remain, got %#v", snap.Items)
	}
}

func TestDrainerSkipsWhenWorkerBusy(t *testing.T) {
	fake := &fakeSender{
		status: workers.WorkerStatus{State: workers.WorkerStateRunning},
		sendCh: make(chan struct{}, 1),
	}
	s, d, id := newDrainerServer(t, fake)
	s.chatQueue.Add(id, "wait your turn", "wait your turn")

	d.drainSession(id)

	select {
	case <-fake.sendCh:
		t.Fatalf("Send should not fire while worker is running")
	case <-time.After(150 * time.Millisecond):
	}

	// Item must remain queued, waiting for the next idle transition.
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 {
		t.Fatalf("expected item to remain queued, got %#v", snap.Items)
	}
}

// A turn running without an in-process worker (detached holder, terminal pi)
// must still block the drainer: the jsonl tail shows the assistant mid-turn.
func TestDrainerSkipsWhenHolderOrTerminalTurnActive(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1)} // worker status: idle
	s, d, id := newDrainerServer(t, fake)
	s.chatQueue.Add(id, "wait for the holder", "wait for the holder")

	// Append a mid-turn tail: the assistant paused to call a tool.
	resolved, err := sessions.ResolveByID(s.sessionsDir, id)
	if err != nil {
		t.Fatal(err)
	}
	entry := `{"type":"message","id":"a1","message":{"role":"assistant","stopReason":"toolUse","content":[{"type":"toolCall","id":"t1","name":"bash"}]}}` + "\n"
	f, err := os.OpenFile(resolved.Path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(entry); err != nil {
		t.Fatal(err)
	}
	f.Close()

	d.drainSession(id)

	select {
	case <-fake.sendCh:
		t.Fatalf("Send should not fire while the jsonl tail shows an active turn")
	case <-time.After(150 * time.Millisecond):
	}
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 {
		t.Fatalf("expected item to remain queued, got %#v", snap.Items)
	}
}

func TestDrainerDrainAllScansEveryActiveSession(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 4)}
	s, d, id := newDrainerServer(t, fake)
	// Same session, two items. drainAll calls drainSession once, which pops
	// one item; the next idle kick handles the rest.
	s.chatQueue.Add(id, "alpha", "alpha")
	s.chatQueue.Add(id, "beta", "beta")

	d.drainAll()
	select {
	case <-fake.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected first Send")
	}

	// Second item still queued (waiting for next idle).
	snap, _ := s.chatQueue.List(id)
	if len(snap.Items) != 1 || snap.Items[0].Message != "beta" {
		t.Fatalf("after first drain, queue should hold beta: %#v", snap.Items)
	}
}

func TestDrainerKickIsNonBlocking(t *testing.T) {
	d := newQueueDrainer(&Server{})
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.kick("any")
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("kick should never block")
	}
}

func TestDrainerHoldsScheduledItemUntilDue(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s, d, id := newDrainerServer(t, fake)
	later := time.Now().Add(time.Hour)
	if _, err := s.chatQueue.AddItem(id, chatqueue.NewItem{Message: "later", NotBefore: &later}); err != nil {
		t.Fatal(err)
	}

	d.drainSession(id)
	select {
	case <-fake.sendCh:
		t.Fatalf("scheduled item dispatched before its time")
	case <-time.After(200 * time.Millisecond):
	}

	s.chatQueue.Now = func() time.Time { return later.Add(time.Second) }
	d.drainSession(id)
	select {
	case <-fake.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("due item should dispatch")
	}
}

func TestDrainerSendsStoredImages(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1)}
	s, d, id := newDrainerServer(t, fake)
	if _, err := s.chatQueue.AddItem(id, chatqueue.NewItem{
		Message: "see image",
		Images:  []chat.Image{{Type: "image", Data: "AAAA", MimeType: "image/png"}},
	}); err != nil {
		t.Fatal(err)
	}
	d.drainSession(id)
	select {
	case <-fake.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected Send")
	}
	_, _, req := fake.sentInfo()
	if len(req.Images) != 1 || req.Images[0].Data != "AAAA" {
		t.Fatalf("images not forwarded: %#v", req.Images)
	}
}

func TestChatQueueSendNowPopsAndDispatches(t *testing.T) {
	fake := &fakeSender{sendCh: make(chan struct{}, 1), status: workers.WorkerStatus{State: workers.WorkerStateRunning}}
	s, _, id := newDrainerServer(t, fake)
	later := time.Now().Add(time.Hour)
	item, err := s.chatQueue.AddItem(id, chatqueue.NewItem{Message: "now please", NotBefore: &later})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/chat/queue/send?id="+id,
		strings.NewReader(`{"position":`+strconv.FormatInt(item.Position, 10)+`}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleChatQueueSend(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	select {
	case <-fake.sendCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected immediate Send even while running")
	}
	if snap, _ := s.chatQueue.List(id); len(snap.Items) != 0 {
		t.Fatalf("item should be popped: %#v", snap.Items)
	}
}
