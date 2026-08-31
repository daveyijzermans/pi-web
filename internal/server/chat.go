package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pi-web/internal/agentdir"
	"pi-web/internal/chat"
	"pi-web/internal/sessions"
	"pi-web/internal/workers"
)

type ChatSender interface {
	Send(ctx context.Context, sessionID, sessionPath string, chat chat.Request) error
	SetModel(ctx context.Context, sessionID, sessionPath, provider, modelID string) error
	SetThinkingLevel(ctx context.Context, sessionID, sessionPath, level string) error
	Compact(ctx context.Context, sessionID, sessionPath string) error
	Abort(ctx context.Context, sessionID string) error
	GetState(ctx context.Context, sessionID string) (workers.WorkerStatus, error)
	GetCommands(ctx context.Context, sessionID string) ([]workers.SlashCommand, bool, error)
	Status(sessionID string) workers.WorkerStatus
	EnsureWorker(ctx context.Context, sessionID, sessionPath string) error
	HasWorker(sessionID string) bool
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	if !resolved.Session.ChatAvailable {
		writeJSONError(w, http.StatusConflict, resolved.Session.ChatDisabledReason)
		return
	}
	chatReq, err := chat.ParseRequest(r, chat.DefaultMaxImageBytes, chat.DefaultMaxRequestBytes)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrEmptyRequest):
			writeJSONError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, chat.ErrImageTooLarge):
			writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
		case errors.As(err, new(*http.MaxBytesError)):
			writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
		default:
			writeJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	// Block a new turn when the session is otherwise busy in a way the web
	// queue can't own: a web-initiated compaction is tying up the worker, or a
	// terminal pi process is mid-turn on the same session file. (A busy web
	// worker is intentionally excluded — that path queues/type-aheads instead.)
	if reason := s.chatBusyReason(resolved.Session.ID); reason != "" {
		writeJSONError(w, http.StatusConflict, reason)
		return
	}
	sessionID := resolved.Session.ID
	sessionPath := resolved.Path

	// Save uploaded files and append reference lines to the message
	if len(chatReq.Files) > 0 {
		uploadDir := filepath.Join(agentdir.WebDir(s.agentDir), "chat-uploads", sessionID)
		saved, err := chat.SaveUploads(uploadDir, chatReq.Files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "save uploads failed for %s: %v\n", sessionID, err)
			writeJSONError(w, http.StatusInternalServerError, "failed to save uploaded file")
			return
		}
		var lines []string
		for _, s := range saved {
			lines = append(lines, chat.AttachmentLine(s))
		}
		if chatReq.Message != "" {
			chatReq.Message += "\n\n" + strings.Join(lines, "\n")
		} else {
			chatReq.Message = strings.Join(lines, "\n")
		}
	}

	s.markWebTurnActive(sessionID)
	if !s.startTask(func(ctx context.Context) {
		if err := s.chatSender.Send(ctx, sessionID, sessionPath, chatReq); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "chat send failed for %s: %v\n", sessionID, err)
		}
	}) {
		writeJSONError(w, http.StatusServiceUnavailable, "server is shutting down")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": "queued"})
}

// recentSessionActivityWindow is the grace period after a JSONL write during
// which a session is still reported as "running" even when no in-process
// chat worker and no session-status file claims it. Kept short so the
// "running" status / Cancel button doesn't linger after the assistant
// finishes streaming its final message.
const recentSessionActivityWindow = 800 * time.Millisecond
const sessionStatusTTL = 10 * time.Second

type sessionStatusFile struct {
	State     string `json:"state"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Server) readSessionStatus(sessionID string) *workers.WorkerStatus {
	if sessionID == "" {
		return nil
	}
	path := filepath.Join(s.sessionStatusDir(), sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var status sessionStatusFile
	if err := json.Unmarshal(data, &status); err != nil {
		return nil
	}
	if status.State != "running" {
		return nil
	}
	updatedAt, err := time.Parse(time.RFC3339, status.UpdatedAt)
	if err != nil {
		return nil
	}
	if time.Since(updatedAt) > sessionStatusTTL {
		return nil
	}
	return &workers.WorkerStatus{State: workers.WorkerStateRunning}
}

// compactRequestTimeout bounds a manual compaction. Summary generation is a
// full LLM turn over the compacted history, so it can take well over a minute
// on long sessions.
const compactRequestTimeout = 5 * time.Minute

// handleCompact triggers a manual compaction of the session context via the
// pi RPC worker. Compaction is a full LLM turn that can outlive a proxy or
// browser connection timeout, so it runs fire-and-forget: the request returns
// 202 immediately and completion is signalled over SSE — a "reload" on success
// (the transcript re-renders with the compaction summary) or a "compact-error"
// event carrying the failure message.
func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	if !resolved.Session.ChatAvailable {
		writeJSONError(w, http.StatusConflict, resolved.Session.ChatDisabledReason)
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	sessionID := resolved.Session.ID
	sessionPath := resolved.Path
	s.setCompacting(sessionID, true)
	s.recomputeAndBroadcastStatus(sessionID)
	if !s.startTask(func(ctx context.Context) {
		defer func() {
			s.setCompacting(sessionID, false)
			s.recomputeAndBroadcastStatus(sessionID)
		}()
		ctx, cancel := context.WithTimeout(ctx, compactRequestTimeout)
		defer cancel()
		if err := s.chatSender.Compact(ctx, sessionID, sessionPath); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fmt.Fprintf(os.Stderr, "compact failed for %s: %v\n", sessionID, err)
			if msg, ferr := formatSSEJSONEvent("compact-error", map[string]any{"error": err.Error()}); ferr == nil {
				s.broadcast(sessionID, msg)
			}
			return
		}
		s.broadcast(sessionID, "reload")
	}) {
		s.setCompacting(sessionID, false)
		s.recomputeAndBroadcastStatus(sessionID)
		writeJSONError(w, http.StatusServiceUnavailable, "server is shutting down")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": "queued"})
}

func (s *Server) handleCancelChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	sessionID := resolved.Session.ID
	if s.chatSender.HasWorker(sessionID) {
		// Normal path: an attached web worker — graceful abort (which kills the
		// worker if it is wedged and unresponsive; see rpc worker Abort).
		if err := s.chatSender.Abort(r.Context(), sessionID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if s.stopOrphanedHolder != nil {
		// No attached worker: a turn orphaned by a pi-web restart keeps running
		// in a detached holder that Abort can't see. Reach the live holder and
		// kill it so the Stop button always ends a running turn. A false return
		// (no live holder) means there is nothing of ours to stop — e.g. a
		// genuine terminal pi session, which we must not touch.
		stopped, err := s.stopOrphanedHolder(sessionID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if stopped {
			s.markTurnStopped(sessionID)
			s.clearWebTurn(sessionID)
		}
	}
	_ = os.Remove(filepath.Join(s.sessionStatusDir(), sessionID))
	s.recomputeAndBroadcastStatus(sessionID)
	s.broadcast(sessionID, "reload")
	writeJSON(w, 0, map[string]any{"ok": true, "status": "cancelled"})
}

func (s *Server) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("id")

	status := workers.WorkerStatus{State: workers.WorkerStateIdle}
	if s.computeRunningStatus(sessionID) {
		status.State = workers.WorkerStateRunning
		status.Compacting = s.isCompacting(sessionID)
	} else if s.chatSender != nil {
		// Do not create/prewarm workers from status polling. A browser can poll
		// many visible sessions at once; if one pi RPC switch_session hangs, eager
		// prewarming accumulates stuck `pi --mode rpc` processes and starves real
		// chat requests. Only report state for an already-created worker here.
		stateCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if state, err := s.chatSender.GetState(stateCtx, sessionID); err == nil {
			status.Model = state.Model
			status.ModelName = state.ModelName
			status.ModelProvider = state.ModelProvider
			status.ThinkingLevel = state.ThinkingLevel
		}
	}
	// Tell the composer whether to block a new turn (compaction / terminal turn)
	// vs. allow it (idle, or type-ahead while its own web worker runs).
	status.BlockedReason = s.chatBusyReason(sessionID)
	writeJSON(w, 0, status)
}

// handleCommands serves the slash-command palette for a session's composer.
// By default it peeks at an existing worker and never spawns one; with
// ?load=1 it ensures a worker first (used when the user opens the palette and
// no worker exists yet). Any failure to query commands degrades to an empty
// list rather than an error — the palette is a non-critical affordance and
// must never break the composer.
func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	sessionID := resolved.Session.ID
	if r.URL.Query().Get("load") == "1" {
		if err := s.chatSender.EnsureWorker(r.Context(), sessionID, resolved.Path); err != nil {
			fmt.Fprintf(os.Stderr, "commands: ensure worker failed for %s: %v\n", sessionID, err)
		}
	}
	cmds, ready, err := s.chatSender.GetCommands(r.Context(), sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "commands: query failed for %s: %v\n", sessionID, err)
		cmds = nil
	}
	if cmds == nil {
		cmds = []workers.SlashCommand{}
	}
	writeJSON(w, 0, map[string]any{"commands": cmds, "workerReady": ready})
}

func (s *Server) hasRecentSessionActivity(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	now := s.now()
	s.fileModMu.RLock()
	mod, ok := s.fileActivity[sessionID]
	s.fileModMu.RUnlock()
	if !ok {
		return false
	}
	return !mod.IsZero() && now.Sub(mod) <= recentSessionActivityWindow
}

func (s *Server) handleSetModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		ModelID  string `json:"modelId"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.Provider == "" || body.ModelID == "" {
		writeJSONError(w, http.StatusBadRequest, "provider and modelId required")
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	if err := s.chatSender.SetModel(r.Context(), resolved.Session.ID, resolved.Path, body.Provider, body.ModelID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 0, map[string]any{"ok": true})
}

func (s *Server) handleSetThinkingLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return
	}
	var body struct {
		Level string `json:"level"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if body.Level == "" {
		writeJSONError(w, http.StatusBadRequest, "level required")
		return
	}
	if s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat unavailable")
		return
	}
	if err := s.chatSender.SetThinkingLevel(r.Context(), resolved.Session.ID, resolved.Path, body.Level); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := s.chatSender.Status(resolved.Session.ID)
	if state, err := s.chatSender.GetState(r.Context(), resolved.Session.ID); err == nil {
		status.ThinkingLevel = state.ThinkingLevel
	}
	writeJSON(w, 0, map[string]any{"ok": true, "thinkingLevel": status.ThinkingLevel})
}
