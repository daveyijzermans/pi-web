package server

import (
	"testing"

	"pi-web/internal/chatqueue"
)

// initDB creates the queue table with every column and then runs the
// add-column migration; on a fresh database those ALTERs collide with the
// CREATE and must be tolerated, and the store must work on the file-backed DB.
func TestInitDBQueueSchemaAndMigrationCoexist(t *testing.T) {
	db, err := initDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := chatqueue.NewStore(db)
	if _, err := store.Add("sess", "hello", ""); err != nil {
		t.Fatal(err)
	}
	ids, err := store.SessionsWithItems()
	if err != nil || len(ids) != 1 {
		t.Fatalf("SessionsWithItems = %v, %v", ids, err)
	}
	if _, ok, err := store.PopHead("sess"); err != nil || !ok {
		t.Fatalf("PopHead ok=%v err=%v", ok, err)
	}
}
