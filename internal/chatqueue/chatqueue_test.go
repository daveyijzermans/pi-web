package chatqueue

import (
	"database/sql"
	"testing"
	"time"

	"pi-web/internal/chat"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{ItemsTableDDL, ItemsSessionIndexDDL, StateTableDDL} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	}
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewStore(db)
	s.Now = func() time.Time { return fixed }
	return s
}

func TestAddListRemove(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Add("sess-1", "hello", "hello"); err != nil {
		t.Fatalf("Add hello: %v", err)
	}
	if _, err := s.Add("sess-1", "world", "world"); err != nil {
		t.Fatalf("Add world: %v", err)
	}
	snap, err := s.List("sess-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(snap.Items))
	}
	if snap.Items[0].Position != 1 || snap.Items[1].Position != 2 {
		t.Fatalf("positions not monotonic: %#v", snap.Items)
	}
	if snap.Paused {
		t.Fatalf("expected paused=false")
	}

	if err := s.Remove("sess-1", 1); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	snap, _ = s.List("sess-1")
	if len(snap.Items) != 1 || snap.Items[0].Message != "world" {
		t.Fatalf("after remove: %#v", snap.Items)
	}
}

func TestPositionsAreNotReused(t *testing.T) {
	s := newTestStore(t)
	a, _ := s.Add("sess-1", "a", "a")
	if err := s.Remove("sess-1", a.Position); err != nil {
		t.Fatalf("remove a: %v", err)
	}
	b, err := s.Add("sess-1", "b", "b")
	if err != nil {
		t.Fatalf("Add b: %v", err)
	}
	if b.Position <= a.Position {
		t.Fatalf("expected new position > %d, got %d", a.Position, b.Position)
	}
}

func TestPopHead(t *testing.T) {
	s := newTestStore(t)
	if _, ok, _ := s.PopHead("sess-1"); ok {
		t.Fatalf("expected empty pop")
	}
	s.Add("sess-1", "one", "one")
	s.Add("sess-1", "two", "two")
	item, ok, err := s.PopHead("sess-1")
	if err != nil || !ok || item.Message != "one" {
		t.Fatalf("PopHead 1: ok=%v err=%v item=%#v", ok, err, item)
	}
	item, ok, err = s.PopHead("sess-1")
	if err != nil || !ok || item.Message != "two" {
		t.Fatalf("PopHead 2: ok=%v err=%v item=%#v", ok, err, item)
	}
	if _, ok, _ := s.PopHead("sess-1"); ok {
		t.Fatalf("expected empty pop after drain")
	}
}

func TestPauseRoundtrip(t *testing.T) {
	s := newTestStore(t)
	snap, _ := s.List("sess-1")
	if snap.Paused {
		t.Fatalf("default paused must be false")
	}
	if err := s.SetPaused("sess-1", true); err != nil {
		t.Fatalf("SetPaused true: %v", err)
	}
	snap, _ = s.List("sess-1")
	if !snap.Paused {
		t.Fatalf("expected paused=true after SetPaused")
	}
	if err := s.SetPaused("sess-1", false); err != nil {
		t.Fatalf("SetPaused false: %v", err)
	}
	snap, _ = s.List("sess-1")
	if snap.Paused {
		t.Fatalf("expected paused=false after toggle")
	}
}

func TestSessionsWithItemsSkipsPaused(t *testing.T) {
	s := newTestStore(t)
	s.Add("active", "a", "a")
	s.Add("paused-with-items", "b", "b")
	s.SetPaused("paused-with-items", true)
	s.SetPaused("paused-empty", true) // paused but no items — should not appear

	ids, err := s.SessionsWithItems()
	if err != nil {
		t.Fatalf("SessionsWithItems: %v", err)
	}
	if len(ids) != 1 || ids[0] != "active" {
		t.Fatalf("expected [active], got %#v", ids)
	}
}

func TestSessionsIsolated(t *testing.T) {
	s := newTestStore(t)
	s.Add("A", "a-1", "a-1")
	s.Add("B", "b-1", "b-1")
	s.Add("A", "a-2", "a-2")

	a, _ := s.List("A")
	if len(a.Items) != 2 || a.Items[0].Message != "a-1" || a.Items[1].Message != "a-2" {
		t.Fatalf("session A snapshot wrong: %#v", a.Items)
	}
	b, _ := s.List("B")
	if len(b.Items) != 1 || b.Items[0].Message != "b-1" {
		t.Fatalf("session B snapshot wrong: %#v", b.Items)
	}
}

func TestClearDropsItemsAndPause(t *testing.T) {
	s := newTestStore(t)
	s.Add("X", "x", "x")
	s.SetPaused("X", true)
	if err := s.Clear("X"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	snap, _ := s.List("X")
	if len(snap.Items) != 0 || snap.Paused {
		t.Fatalf("after Clear: %#v", snap)
	}
}

func TestAddRejectsEmptyMessage(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Add("sess", "", ""); err == nil {
		t.Fatalf("expected error for empty message")
	}
}

func TestRemoveMissingIsNoop(t *testing.T) {
	s := newTestStore(t)
	if err := s.Remove("sess", 99); err != nil {
		t.Fatalf("remove missing should be no-op, got %v", err)
	}
}

func TestAddItemRoundtripsImagesAttachmentsAndNotBefore(t *testing.T) {
	s := newTestStore(t)
	due := s.Now().Add(time.Hour)
	added, err := s.AddItem("sess", NewItem{
		Message:     "look",
		Images:      []chat.Image{{Type: "image", Data: "AAAA", MimeType: "image/png"}},
		Attachments: []string{"shot.png"},
		NotBefore:   &due,
	})
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if added.ImageCount != 1 || added.NotBefore == nil || !added.NotBefore.Equal(due) {
		t.Fatalf("unexpected added item: %#v", added)
	}
	snap, err := s.List("sess")
	if err != nil {
		t.Fatal(err)
	}
	got := snap.Items[0]
	if len(got.Images) != 1 || got.Images[0].Data != "AAAA" || got.Images[0].MimeType != "image/png" {
		t.Fatalf("images not persisted: %#v", got.Images)
	}
	if len(got.Attachments) != 1 || got.Attachments[0] != "shot.png" {
		t.Fatalf("attachments not persisted: %#v", got.Attachments)
	}
	if got.NotBefore == nil || !got.NotBefore.Equal(due) {
		t.Fatalf("not_before not persisted: %v", got.NotBefore)
	}
	if got.ImageCount != 1 {
		t.Fatalf("ImageCount=%d", got.ImageCount)
	}
}

func TestAddItemAllowsImageOnly(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AddItem("sess", NewItem{Images: []chat.Image{{Type: "image", Data: "x", MimeType: "image/png"}}}); err != nil {
		t.Fatalf("image-only item should be accepted: %v", err)
	}
}

func TestNotBeforeHoldsItemUntilDue(t *testing.T) {
	s := newTestStore(t)
	base := s.Now()
	later := base.Add(30 * time.Minute)
	if _, err := s.AddItem("sess", NewItem{Message: "scheduled", NotBefore: &later}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("sess", "immediate", ""); err != nil {
		t.Fatal(err)
	}

	// Only the immediate item is due: it must skip ahead of the scheduled one.
	ids, _ := s.SessionsWithItems()
	if len(ids) != 1 {
		t.Fatalf("session with a due item should be listed, got %v", ids)
	}
	item, ok, err := s.PopHead("sess")
	if err != nil || !ok || item.Message != "immediate" {
		t.Fatalf("PopHead = %#v ok=%v err=%v; want immediate", item, ok, err)
	}
	if _, ok, _ := s.PopHead("sess"); ok {
		t.Fatalf("scheduled item must not pop before it is due")
	}
	ids, _ = s.SessionsWithItems()
	if len(ids) != 0 {
		t.Fatalf("no due items → no sessions, got %v", ids)
	}

	// Time passes → the scheduled item becomes due.
	s.Now = func() time.Time { return later.Add(time.Second) }
	ids, _ = s.SessionsWithItems()
	if len(ids) != 1 {
		t.Fatalf("due session should be listed, got %v", ids)
	}
	item, ok, _ = s.PopHead("sess")
	if !ok || item.Message != "scheduled" {
		t.Fatalf("PopHead after due = %#v ok=%v", item, ok)
	}
}

func TestSetNotBeforeClearsSchedule(t *testing.T) {
	s := newTestStore(t)
	later := s.Now().Add(time.Hour)
	added, err := s.AddItem("sess", NewItem{Message: "scheduled", NotBefore: &later})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetNotBefore("sess", added.Position, nil); err != nil {
		t.Fatal(err)
	}
	item, ok, _ := s.PopHead("sess")
	if !ok || item.NotBefore != nil {
		t.Fatalf("cleared schedule should be due immediately: %#v ok=%v", item, ok)
	}
}

func TestMigrateItemsIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	legacy := `CREATE TABLE chat_queue_items (
		session_id TEXT NOT NULL, position INTEGER NOT NULL, message TEXT NOT NULL,
		display_text TEXT NOT NULL, created_at DATETIME NOT NULL, PRIMARY KEY (session_id, position))`
	if _, err := db.Exec(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(StateTableDDL); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateItems(db); err != nil {
			t.Fatalf("migrate run %d: %v", i, err)
		}
	}
	s := NewStore(db)
	if _, err := s.Add("sess", "hello", ""); err != nil {
		t.Fatalf("Add after migration: %v", err)
	}
	if snap, err := s.List("sess"); err != nil || len(snap.Items) != 1 {
		t.Fatalf("List after migration: %v %#v", err, snap)
	}
}
