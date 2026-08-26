package server

import (
	"os"
	"path/filepath"
	"time"
)

// web-turn markers exist to survive a pi-web restart. Chat workers are
// in-memory only, so a server restart (crash, Air reload, upgrade) mid-turn
// wipes the worker while the session jsonl tail still shows the assistant
// working. Without a marker, chatBusyReason can't tell that orphaned web turn
// apart from a live terminal pi turn, and wedges the session for the whole
// terminalTurnStaleWindow. The marker records "pi-web owned an in-flight turn
// here"; on the next startup those markers become the orphaned set, so the
// block is skipped and a fresh Send can safely take the session back over.

// webTurnDir is the directory holding one marker file per session with a
// pi-web turn in flight.
func (s *Server) webTurnDir() string {
	return filepath.Join(s.agentDir, "web-turns")
}

// markWebTurnActive records that pi-web has dispatched a turn for this session.
// Best-effort: a missing marker only costs the pre-restart wedge this guards
// against, never correctness of a live turn.
func (s *Server) markWebTurnActive(sessionID string) {
	if sessionID == "" || s.agentDir == "" {
		return
	}
	dir := s.webTurnDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, sessionID), nil, 0o644)
}

// clearWebTurn removes the marker and drops the session from the orphaned set.
// Called when a turn finishes (running → idle): a completed turn is no longer
// in flight, so it must not be treated as orphaned after a later restart.
func (s *Server) clearWebTurn(sessionID string) {
	if sessionID == "" || s.agentDir == "" {
		return
	}
	s.orphanedMu.Lock()
	delete(s.orphanedWebTurns, sessionID)
	s.orphanedMu.Unlock()
	_ = os.Remove(filepath.Join(s.webTurnDir(), sessionID))
}

// loadOrphanedWebTurns seeds the orphaned set from marker files left by a
// previous instance. Each marker is a session whose web turn never reached a
// clean running → idle transition (the instance died first), so its mid-turn
// jsonl tail is a dead web turn, not a live terminal turn.
func (s *Server) loadOrphanedWebTurns() {
	if s.agentDir == "" {
		return
	}
	entries, err := os.ReadDir(s.webTurnDir())
	if err != nil {
		return
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	s.orphanedMu.Lock()
	s.orphanLoadedAt = now()
	for _, e := range entries {
		if !e.IsDir() {
			s.orphanedWebTurns[e.Name()] = struct{}{}
		}
	}
	s.orphanedMu.Unlock()
}

// isOrphanedWebTurn reports whether this session was left mid-web-turn by a
// previous instance. Such a session has no live worker and no terminal pi
// behind it, so it is neither running nor a reason to block a new web turn.
//
// The flag only holds for terminalTurnStaleWindow after startup: that is the
// exact span during which a dead turn's frozen jsonl tail would otherwise read
// as an active terminal turn. Past it the tail goes stale on its own, so the
// flag expires and normal terminal detection resumes — a genuine terminal turn
// started later is never masked.
func (s *Server) isOrphanedWebTurn(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	s.orphanedMu.Lock()
	defer s.orphanedMu.Unlock()
	if _, ok := s.orphanedWebTurns[sessionID]; !ok {
		return false
	}
	return now().Sub(s.orphanLoadedAt) <= terminalTurnStaleWindow
}
