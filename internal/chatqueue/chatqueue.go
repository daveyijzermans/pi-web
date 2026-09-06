// Package chatqueue is the SQLite-backed store for per-session chat message
// queues. The queue lets a user line up follow-up prompts that the worker
// drains autonomously when it becomes idle, even across browser refreshes
// (and, with the drainer running, across browser closes).
//
// Two tables back the feature:
//
//   - chat_queue_items: append-only list of pending messages, ordered by
//     position within a session.
//   - chat_queue_state: per-session settings (currently just `paused`).
//
// Items may carry inline images (the same shape /api/chat hands to pi) and an
// optional not_before time: the drainer holds such an item until it is due,
// which is how "send later" turns are scheduled inside an existing session.
//
// Steers are intentionally not modelled here: they belong to a specific
// in-flight run on the browser and have no meaning after the page goes away.
package chatqueue

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"pi-web/internal/chat"
)

// ItemsTableDDL creates the queue items table. Registered in server.initDB.
const ItemsTableDDL = `CREATE TABLE IF NOT EXISTS chat_queue_items (
	session_id   TEXT NOT NULL,
	position     INTEGER NOT NULL,
	message      TEXT NOT NULL,
	display_text TEXT NOT NULL,
	created_at   DATETIME NOT NULL,
	images       TEXT NOT NULL DEFAULT '',
	attachments  TEXT NOT NULL DEFAULT '',
	not_before   DATETIME,
	PRIMARY KEY (session_id, position)
)`

// ItemsMigrationDDL adds the columns introduced after the table first shipped.
// Each statement fails harmlessly on databases that already have the column;
// MigrateItems runs them and ignores "duplicate column" errors.
var ItemsMigrationDDL = []string{
	`ALTER TABLE chat_queue_items ADD COLUMN images TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE chat_queue_items ADD COLUMN attachments TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE chat_queue_items ADD COLUMN not_before DATETIME`,
}

// MigrateItems applies ItemsMigrationDDL, tolerating columns that already exist.
func MigrateItems(db *sql.DB) error {
	for _, stmt := range ItemsMigrationDDL {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumn(err) {
			return fmt.Errorf("migrate chat_queue_items: %w", err)
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column")
}

// ItemsSessionIndexDDL speeds up the List + drainer scans.
const ItemsSessionIndexDDL = `CREATE INDEX IF NOT EXISTS chat_queue_items_session_idx
	ON chat_queue_items(session_id, position)`

// StateTableDDL creates the per-session settings table. `next_position` holds
// the next monotonic position for this session: positions are never reused even
// after an item is removed, so any client-side identifier stays stable.
const StateTableDDL = `CREATE TABLE IF NOT EXISTS chat_queue_state (
	session_id    TEXT PRIMARY KEY,
	paused        INTEGER NOT NULL DEFAULT 0,
	next_position INTEGER NOT NULL DEFAULT 1,
	updated_at    DATETIME NOT NULL
)`

// Item is a single queued message in the order the user added it.
type Item struct {
	SessionID   string       `json:"sessionId"`
	Position    int64        `json:"position"`
	Message     string       `json:"message"`
	DisplayText string       `json:"displayText"`
	CreatedAt   time.Time    `json:"createdAt"`
	Images      []chat.Image `json:"-"`
	// Attachments lists the saved upload filenames so the UI can show what the
	// item carries without shipping the image bytes to every tab.
	Attachments []string   `json:"attachments"`
	ImageCount  int        `json:"imageCount"`
	NotBefore   *time.Time `json:"notBefore,omitempty"`
}

// NewItem is the input to AddItem.
type NewItem struct {
	Message     string
	DisplayText string
	Images      []chat.Image
	Attachments []string
	NotBefore   *time.Time
}

// Snapshot is the full state returned to the browser: the ordered items plus
// the paused flag.
type Snapshot struct {
	Items  []Item `json:"items"`
	Paused bool   `json:"paused"`
}

// Store is a thin SQLite-backed repository for queues and paused state. The
// schema must already be created (see ItemsTableDDL / StateTableDDL,
// registered in server.initDB).
type Store struct {
	db  *sql.DB
	Now func() time.Time
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, Now: time.Now}
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// List returns the snapshot for a session: items in queue order plus the
// paused flag. An unknown session yields an empty snapshot (paused=false), not
// an error.
func (s *Store) List(sessionID string) (Snapshot, error) {
	if sessionID == "" {
		return Snapshot{}, errors.New("sessionID is required")
	}
	rows, err := s.db.Query(
		`SELECT `+itemColumns+`
		 FROM chat_queue_items WHERE session_id = ? ORDER BY position ASC`,
		sessionID,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		item, err := scanItem(sessionID, rows)
		if err != nil {
			return Snapshot{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate items: %w", err)
	}
	paused, err := s.isPaused(sessionID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Items: items, Paused: paused}, nil
}

// IsPaused returns whether autonomous draining is paused for the session.
// Unknown sessions are reported as not paused.
func (s *Store) IsPaused(sessionID string) (bool, error) {
	return s.isPaused(sessionID)
}

func (s *Store) isPaused(sessionID string) (bool, error) {
	var paused int
	err := s.db.QueryRow(
		`SELECT paused FROM chat_queue_state WHERE session_id = ?`, sessionID,
	).Scan(&paused)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query paused: %w", err)
	}
	return paused != 0, nil
}

const itemColumns = `position, message, display_text, created_at, images, attachments, not_before`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(sessionID string, row rowScanner) (Item, error) {
	var item Item
	var images, attachments string
	var notBefore sql.NullTime
	item.SessionID = sessionID
	if err := row.Scan(&item.Position, &item.Message, &item.DisplayText, &item.CreatedAt, &images, &attachments, &notBefore); err != nil {
		return Item{}, fmt.Errorf("scan item: %w", err)
	}
	if images != "" {
		if err := json.Unmarshal([]byte(images), &item.Images); err != nil {
			return Item{}, fmt.Errorf("decode images: %w", err)
		}
	}
	if attachments != "" {
		if err := json.Unmarshal([]byte(attachments), &item.Attachments); err != nil {
			return Item{}, fmt.Errorf("decode attachments: %w", err)
		}
	}
	if item.Attachments == nil {
		item.Attachments = []string{}
	}
	item.ImageCount = len(item.Images)
	if notBefore.Valid {
		t := notBefore.Time.UTC()
		item.NotBefore = &t
	}
	return item, nil
}

// Add appends a text-only item; see AddItem.
func (s *Store) Add(sessionID, message, displayText string) (Item, error) {
	return s.AddItem(sessionID, NewItem{Message: message, DisplayText: displayText})
}

// AddItem appends an item to the session's queue and returns the new row. The
// position is the previous max+1 (or 1 if the queue was empty); positions are
// monotonic per session, never reused.
func (s *Store) AddItem(sessionID string, in NewItem) (Item, error) {
	if sessionID == "" {
		return Item{}, errors.New("sessionID is required")
	}
	message, displayText := in.Message, in.DisplayText
	if message == "" && len(in.Images) == 0 {
		return Item{}, errors.New("message is required")
	}
	if displayText == "" {
		displayText = message
	}
	imagesJSON, attachmentsJSON := "", ""
	if len(in.Images) > 0 {
		raw, err := json.Marshal(in.Images)
		if err != nil {
			return Item{}, fmt.Errorf("encode images: %w", err)
		}
		imagesJSON = string(raw)
	}
	if len(in.Attachments) > 0 {
		raw, err := json.Marshal(in.Attachments)
		if err != nil {
			return Item{}, fmt.Errorf("encode attachments: %w", err)
		}
		attachmentsJSON = string(raw)
	}
	var notBefore any
	var notBeforeOut *time.Time
	if in.NotBefore != nil {
		t := in.NotBefore.UTC()
		notBefore = t
		notBeforeOut = &t
	}
	now := s.now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return Item{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	// Take the next position from chat_queue_state.next_position and bump it,
	// so positions are monotonic per session even across removals — that keeps
	// position usable as a stable identifier on the client.
	if _, err := tx.Exec(
		`INSERT INTO chat_queue_state (session_id, paused, next_position, updated_at)
		 VALUES (?, 0, 1, ?)
		 ON CONFLICT(session_id) DO NOTHING`,
		sessionID, now,
	); err != nil {
		return Item{}, fmt.Errorf("ensure state: %w", err)
	}
	var next int64
	if err := tx.QueryRow(
		`SELECT next_position FROM chat_queue_state WHERE session_id = ?`, sessionID,
	).Scan(&next); err != nil {
		return Item{}, fmt.Errorf("read next position: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE chat_queue_state SET next_position = next_position + 1, updated_at = ? WHERE session_id = ?`,
		now, sessionID,
	); err != nil {
		return Item{}, fmt.Errorf("bump next position: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO chat_queue_items (session_id, position, message, display_text, created_at, images, attachments, not_before)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, next, message, displayText, now, imagesJSON, attachmentsJSON, notBefore,
	); err != nil {
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("commit: %w", err)
	}
	attachments := in.Attachments
	if attachments == nil {
		attachments = []string{}
	}
	return Item{
		SessionID:   sessionID,
		Position:    next,
		Message:     message,
		DisplayText: displayText,
		CreatedAt:   now,
		Images:      in.Images,
		Attachments: attachments,
		ImageCount:  len(in.Images),
		NotBefore:   notBeforeOut,
	}, nil
}

// Take removes and returns one specific item regardless of its schedule.
// Returns (Item{}, false, nil) when the row doesn't exist. Used by "send now".
func (s *Store) Take(sessionID string, position int64) (Item, bool, error) {
	if sessionID == "" {
		return Item{}, false, errors.New("sessionID is required")
	}
	row := s.db.QueryRow(
		`SELECT `+itemColumns+` FROM chat_queue_items WHERE session_id = ? AND position = ?`,
		sessionID, position,
	)
	item, err := scanItem(sessionID, row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, false, nil
		}
		return Item{}, false, fmt.Errorf("query item: %w", err)
	}
	if _, err := s.db.Exec(
		`DELETE FROM chat_queue_items WHERE session_id = ? AND position = ?`,
		sessionID, position,
	); err != nil {
		return Item{}, false, fmt.Errorf("delete item: %w", err)
	}
	return item, true, nil
}

// SetNotBefore updates (or clears, with nil) the earliest dispatch time of an
// item. Used by "send now" on a scheduled row and by rescheduling.
func (s *Store) SetNotBefore(sessionID string, position int64, notBefore *time.Time) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	var value any
	if notBefore != nil {
		value = notBefore.UTC()
	}
	_, err := s.db.Exec(
		`UPDATE chat_queue_items SET not_before = ? WHERE session_id = ? AND position = ?`,
		value, sessionID, position,
	)
	if err != nil {
		return fmt.Errorf("set not_before: %w", err)
	}
	return nil
}

// Remove deletes a single item by (sessionID, position). Returns nil if the
// row didn't exist — idempotent so the UI can fire-and-forget without racing.
func (s *Store) Remove(sessionID string, position int64) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	_, err := s.db.Exec(
		`DELETE FROM chat_queue_items WHERE session_id = ? AND position = ?`,
		sessionID, position,
	)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// PopHead removes and returns the lowest-position *due* item for a session
// (not_before unset or already passed). Returns (Item{}, false, nil) if nothing
// is due — used by the autonomous drainer when the worker is idle. Items that
// are not yet due never block later immediate ones.
func (s *Store) PopHead(sessionID string) (Item, bool, error) {
	if sessionID == "" {
		return Item{}, false, errors.New("sessionID is required")
	}
	row := s.db.QueryRow(
		`SELECT `+itemColumns+`
		 FROM chat_queue_items
		 WHERE session_id = ? AND (not_before IS NULL OR not_before <= ?)
		 ORDER BY position ASC LIMIT 1`,
		sessionID, s.now().UTC(),
	)
	item, err := scanItem(sessionID, row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, false, nil
		}
		return Item{}, false, fmt.Errorf("query head: %w", err)
	}
	if _, err := s.db.Exec(
		`DELETE FROM chat_queue_items WHERE session_id = ? AND position = ?`,
		sessionID, item.Position,
	); err != nil {
		return Item{}, false, fmt.Errorf("delete head: %w", err)
	}
	return item, true, nil
}

// SetPaused writes the per-session paused flag. paused=false with no items
// effectively clears any saved state, but we keep the row anyway for simplicity.
func (s *Store) SetPaused(sessionID string, paused bool) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	val := 0
	if paused {
		val = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO chat_queue_state (session_id, paused, next_position, updated_at)
		 VALUES (?, ?, 1, ?)
		 ON CONFLICT(session_id) DO UPDATE SET paused=excluded.paused, updated_at=excluded.updated_at`,
		sessionID, val, s.now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert paused: %w", err)
	}
	return nil
}

// SessionsWithItems lists session ids that have at least one *due* queued
// item AND are not paused — the set the autonomous drainer needs to wake up.
func (s *Store) SessionsWithItems() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT q.session_id
		 FROM chat_queue_items q
		 LEFT JOIN chat_queue_state st ON st.session_id = q.session_id
		 WHERE COALESCE(st.paused, 0) = 0
		   AND (q.not_before IS NULL OR q.not_before <= ?)`,
		s.now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Clear drops everything for a session. Currently unused; intended for
// session-fork / session-delete to keep the table tidy.
func (s *Store) Clear(sessionID string) error {
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM chat_queue_items WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("clear items: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM chat_queue_state WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("clear state: %w", err)
	}
	return tx.Commit()
}
