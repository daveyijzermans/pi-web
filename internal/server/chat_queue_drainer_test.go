package server

import (
	"os"
	"sync"
	"testing"
	"time"

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

// A turn can be running with NO in-process worker: a detached worker-holder
// after a server restart, or a terminal pi. The jsonl tail shows the assistant
// mid-response; dispatching then would steer the queued item into that turn.
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
