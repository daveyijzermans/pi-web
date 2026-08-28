package server

import (
	"os"
	"path/filepath"
	"time"

	"pi-web/internal/sessions"
)

// workerSocketDir is where detached worker-holders listen (one socket per
// session). Mirrors the path the app wires into the worker factory.
func (s *Server) workerSocketDir() string {
	return filepath.Join(s.agentDir, "pi-web", "workers")
}

// markTurnStopped records that a session's turn was force-stopped (holder
// killed) via cancel. computeRunningStatus then reports idle until the jsonl
// is written again, so a killed turn's stale mid-turn tail can't keep the
// indicator lit for the full terminalTurnStaleWindow.
func (s *Server) markTurnStopped(sessionID string) {
	if sessionID == "" {
		return
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	s.stoppedMu.Lock()
	if s.stoppedAt == nil {
		s.stoppedAt = make(map[string]time.Time)
	}
	s.stoppedAt[sessionID] = now()
	s.stoppedMu.Unlock()
}

// wasStoppedAfterLastWrite reports whether the session was force-stopped and
// its jsonl has not been written since. A later write (a genuinely new turn)
// makes the mtime newer than the stop mark and clears the override.
func (s *Server) wasStoppedAfterLastWrite(sessionID string) bool {
	s.stoppedMu.Lock()
	ts, ok := s.stoppedAt[sessionID]
	s.stoppedMu.Unlock()
	if !ok {
		return false
	}
	path := s.sessionPath(sessionID)
	if path == "" {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return !info.ModTime().After(ts)
}

// terminalTurnStaleWindow caps how long a mid-turn tail state is trusted as
// "running" without a fresh write. It keeps the indicator lit through long
// tool executions (where pi writes nothing while a command runs) yet lets an
// abandoned turn (terminal killed mid-tool) fall back to idle instead of
// showing "running" forever.
const terminalTurnStaleWindow = 5 * time.Minute

// sessionPath returns the absolute jsonl path for a session id, preferring the
// watcher's cache (populated on every file event) and falling back to a
// directory resolve so status still works before the first watch event lands.
func (s *Server) sessionPath(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	s.fileModMu.RLock()
	path := s.filePath[sessionID]
	s.fileModMu.RUnlock()
	if path != "" {
		return path
	}
	if resolved, err := sessions.ResolveByID(s.sessionsDir, sessionID); err == nil {
		return resolved.Path
	}
	return ""
}

// terminalTurnState returns the last-message state of the session jsonl, but
// only when the file was written recently enough to still be an active turn
// (ok=false past terminalTurnStaleWindow, which retires abandoned turns).
func (s *Server) terminalTurnState(sessionID string) (sessions.TurnState, bool) {
	path := s.sessionPath(sessionID)
	if path == "" {
		return sessions.TurnState{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return sessions.TurnState{}, false
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	if now().Sub(info.ModTime()) > terminalTurnStaleWindow {
		return sessions.TurnState{}, false
	}
	state, err := sessions.ReadLastTurnState(path)
	if err != nil {
		return sessions.TurnState{}, false
	}
	return state, true
}

// hasActiveTerminalTurn reports whether an external (terminal) pi process is
// mid-turn on this session, inferred from the jsonl tail state. Unlike the
// 800ms recent-activity fallback it stays true across the quiet gaps of a
// running turn (e.g. while a tool command executes), which is what keeps the
// "working" indicator from flickering. Used for the (inclusive) indicator.
func (s *Server) hasActiveTerminalTurn(sessionID string) bool {
	state, ok := s.terminalTurnState(sessionID)
	return ok && state.AssistantWorking()
}

// chatBusyReason returns a human-readable reason to reject a new web turn, or
// "" when sending is allowed. It blocks only for states the web chat queue
// cannot safely own: an in-flight compaction, or a terminal pi process that is
// mid-turn on the same session. A busy in-process web worker returns "" so the
// existing type-ahead queue keeps working.
func (s *Server) chatBusyReason(sessionID string) string {
	if s.isCompacting(sessionID) {
		return "session is compacting—try again once it finishes"
	}
	if s.chatSender != nil && s.chatSender.HasWorker(sessionID) {
		return ""
	}
	// A turn this instance inherited as orphaned (its web worker died on a
	// previous restart) has no live writer behind it, so a new Send can safely
	// take the session back over instead of being wedged as a terminal turn.
	if s.isOrphanedWebTurn(sessionID) {
		return ""
	}
	// Only block when the assistant is clearly mid-response (a bare pending user
	// message is too ambiguous to reject a web turn over).
	if s.hasActiveTerminalTurn(sessionID) {
		return "terminal session is running"
	}
	return ""
}

// setCompacting marks (or clears) an in-flight manual compaction for a session.
func (s *Server) setCompacting(sessionID string, active bool) {
	if sessionID == "" {
		return
	}
	s.compactingMu.Lock()
	if s.compacting == nil {
		s.compacting = make(map[string]struct{})
	}
	if active {
		s.compacting[sessionID] = struct{}{}
	} else {
		delete(s.compacting, sessionID)
	}
	s.compactingMu.Unlock()
}

// isCompacting reports whether a manual compaction is currently running for
// this session (web-initiated). A compacting session is busy: its worker is
// tied up generating the summary, so new turns must be blocked.
func (s *Server) isCompacting(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	s.compactingMu.Lock()
	_, ok := s.compacting[sessionID]
	s.compactingMu.Unlock()
	return ok
}
