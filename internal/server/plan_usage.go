package server

import "net/http"

// GET /api/plan-usage — subscription rate-limit windows for every provider pi
// holds an OAuth login for. Cached server-side (see planusage.Service); the
// composer polls it lazily and the "send later" picker uses the reset times.
func (s *Server) handlePlanUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.planUsage == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.planUsage.Snapshot(r.Context())})
}
