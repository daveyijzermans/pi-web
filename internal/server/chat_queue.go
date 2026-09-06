package server

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"pi-web/internal/chat"
	"pi-web/internal/chatqueue"
	"pi-web/internal/sessions"
)

// /api/chat/queue: single endpoint that demuxes on method.
//
//   GET    /api/chat/queue?id=<sessionID>             — list items + paused
//   POST   /api/chat/queue?id=<sessionID>             — append an item
//   DELETE /api/chat/queue?id=<sessionID>&position=N  — remove an item
//   PATCH  /api/chat/queue?id=<sessionID>             — set paused flag, or
//                                                       reschedule one item
//   POST   /api/chat/queue/send?id=<sessionID>        — {position}: pop the
//                                                       item and send it now
//                                                       (steers if running)
//
// POST accepts either JSON {message, displayText, notBefore} or the same
// multipart form /api/chat takes (message + images files + displayText +
// notBefore). Uploads are saved to disk immediately — exactly as an immediate
// send would — so the queued row carries the attachment lines and inline
// images and survives the browser going away. notBefore (RFC 3339) holds the
// item until that time; the drainer dispatches it on its next tick after.
//
// On any state change, we broadcast a "queue" SSE event on the session topic
// so any other open tab refreshes its local view, and kick the drainer so it
// re-evaluates this session immediately rather than waiting for the periodic
// tick.

func (s *Server) handleChatQueue(w http.ResponseWriter, r *http.Request) {
	if s.chatQueue == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat queue unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleChatQueueGet(w, r)
	case http.MethodPost:
		s.handleChatQueuePost(w, r)
	case http.MethodDelete:
		s.handleChatQueueDelete(w, r)
	case http.MethodPatch:
		s.handleChatQueuePatch(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE, PATCH")
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) resolveQueueSession(r *http.Request, w http.ResponseWriter) (string, bool) {
	resolved, err := sessions.ResolveByID(s.sessionsDir, r.URL.Query().Get("id"))
	if resolveOrWriteError(w, err) {
		return "", false
	}
	return resolved.Session.ID, true
}

func (s *Server) handleChatQueueGet(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := s.resolveQueueSession(r, w)
	if !ok {
		return
	}
	snap, err := s.chatQueue.List(sessionID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list queue: "+err.Error())
		return
	}
	if snap.Items == nil {
		snap.Items = []chatqueue.Item{} // ensure JSON encodes [] not null
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleChatQueuePost(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := s.resolveQueueSession(r, w)
	if !ok {
		return
	}
	in, ok := s.parseQueuePost(w, r, sessionID)
	if !ok {
		return
	}
	item, err := s.chatQueue.AddItem(sessionID, in)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "add queue item: "+err.Error())
		return
	}
	s.notifyQueueChanged(sessionID)
	writeJSON(w, http.StatusCreated, item)
}

// parseQueuePost reads a JSON or multipart queue POST into a NewItem. Writes
// the error response itself and returns ok=false on failure.
func (s *Server) parseQueuePost(w http.ResponseWriter, r *http.Request, sessionID string) (chatqueue.NewItem, bool) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var in chatqueue.NewItem
	var notBeforeRaw string
	if mediaType == "multipart/form-data" {
		chatReq, err := chat.ParseRequest(r, chat.DefaultMaxImageBytes, chat.DefaultMaxRequestBytes)
		if err != nil {
			switch {
			case errors.Is(err, chat.ErrEmptyRequest):
				writeJSONError(w, http.StatusBadRequest, "message is required")
			case errors.Is(err, chat.ErrImageTooLarge), errors.As(err, new(*http.MaxBytesError)):
				writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
			default:
				writeJSONError(w, http.StatusBadRequest, err.Error())
			}
			return in, false
		}
		names, err := s.saveChatUploads(sessionID, &chatReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "save uploads failed for %s: %v\n", sessionID, err)
			writeJSONError(w, http.StatusInternalServerError, "failed to save uploaded file")
			return in, false
		}
		in.Message = chatReq.Message
		in.Images = chatReq.Images
		in.Attachments = names
		in.DisplayText = r.FormValue("displayText")
		notBeforeRaw = r.FormValue("notBefore")
	} else {
		var body struct {
			Message     string `json:"message"`
			DisplayText string `json:"displayText"`
			NotBefore   string `json:"notBefore"`
		}
		if !decodeJSONBody(w, r, &body) {
			return in, false
		}
		in.Message = strings.TrimSpace(body.Message)
		in.DisplayText = body.DisplayText
		notBeforeRaw = body.NotBefore
		if in.Message == "" {
			writeJSONError(w, http.StatusBadRequest, "message is required")
			return in, false
		}
	}
	if strings.TrimSpace(in.DisplayText) == "" {
		in.DisplayText = in.Message
	}
	notBefore, err := parseNotBefore(notBeforeRaw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return in, false
	}
	in.NotBefore = notBefore
	return in, true
}

// parseNotBefore accepts "" (no schedule) or an RFC 3339 timestamp.
func parseNotBefore(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New("notBefore must be an RFC 3339 timestamp")
	}
	t = t.UTC()
	return &t, nil
}

func (s *Server) handleChatQueueDelete(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := s.resolveQueueSession(r, w)
	if !ok {
		return
	}
	posStr := r.URL.Query().Get("position")
	if posStr == "" {
		writeJSONError(w, http.StatusBadRequest, "position is required")
		return
	}
	pos, err := strconv.ParseInt(posStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "position must be an integer")
		return
	}
	if err := s.chatQueue.Remove(sessionID, pos); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "remove queue item: "+err.Error())
		return
	}
	s.notifyQueueChanged(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatQueuePatch(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := s.resolveQueueSession(r, w)
	if !ok {
		return
	}
	var body struct {
		Paused    *bool   `json:"paused"`
		Position  *int64  `json:"position"`
		NotBefore *string `json:"notBefore"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	// Reschedule form: {position, notBefore} — notBefore "" or null clears the
	// schedule so the item becomes due immediately.
	if body.Position != nil {
		var raw string
		if body.NotBefore != nil {
			raw = *body.NotBefore
		}
		notBefore, err := parseNotBefore(raw)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.chatQueue.SetNotBefore(sessionID, *body.Position, notBefore); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "reschedule: "+err.Error())
			return
		}
		s.notifyQueueChanged(sessionID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "position": *body.Position, "notBefore": notBefore})
		return
	}
	if body.Paused == nil {
		writeJSONError(w, http.StatusBadRequest, "paused or position is required")
		return
	}
	if err := s.chatQueue.SetPaused(sessionID, *body.Paused); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "set paused: "+err.Error())
		return
	}
	s.notifyQueueChanged(sessionID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "paused": *body.Paused})
}

func (s *Server) handleChatQueueSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.chatQueue == nil || s.queueDrainer == nil || s.chatSender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "chat queue unavailable")
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
	var body struct {
		Position *int64 `json:"position"`
	}
	if !decodeJSONBody(w, r, &body) || body.Position == nil {
		if body.Position == nil {
			writeJSONError(w, http.StatusBadRequest, "position is required")
		}
		return
	}
	sessionID := resolved.Session.ID
	item, ok, err := s.chatQueue.Take(sessionID, *body.Position)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "take queue item: "+err.Error())
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "queue item not found")
		return
	}
	s.queueDrainer.dispatch(sessionID, resolved.Path, item)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "status": "queued"})
}

// notifyQueueChanged informs other tabs (via SSE) and the drainer that this
// session's queue state changed. The drainer kick is best-effort: a nil
// drainer (e.g. tests) is a no-op. The SSE payload is intentionally tiny —
// listeners just refetch /api/chat/queue to get the new state.
func (s *Server) notifyQueueChanged(sessionID string) {
	if msg, err := formatSSEJSONEvent("queue", map[string]any{"sessionId": sessionID}); err == nil {
		s.broadcast(sessionID, msg)
	}
	if s.queueDrainer != nil {
		s.queueDrainer.kick(sessionID)
	}
}
