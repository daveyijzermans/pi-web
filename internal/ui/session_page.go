package ui

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"os"
	"strconv"

	"pi-web/internal/sessions"
)

// share-session.html renders the static export/share snapshot only; the live
// session page is the Svelte SPA served via the app.html shell.
//
//go:embed embedded/share-session.html
var exportSessionHtml string

var exportSessionTmpl = template.Must(template.New("export_session").Parse(exportSessionHtml))

//go:embed embedded/styles/theme.css
var liveThemeCss string

//go:embed embedded/styles/session.css
var liveSessionCss string

//go:embed embedded/styles/menu.css
var liveMenuCss string

//go:embed embedded/styles/palette.css
var livePaletteCss string

// LargeSessionTailEntries controls how many trailing entries get embedded
// in the initial HTML render for huge sessions. The frontend exposes a
// "Load earlier" affordance that fetches preceding windows via
// /api/session?id=...&from=N&count=K.
//
// Defaults are production values; both are overridable via env vars so tests
// (and future config plumbing) can trigger truncation with a small session
// instead of rendering thousands of entries. Read once at startup.
var (
	LargeSessionThreshold   = envInt("PI_WEB_LARGE_SESSION_THRESHOLD", 1500)
	LargeSessionTailEntries = envInt("PI_WEB_LARGE_SESSION_TAIL_ENTRIES", 1000)
	// LargeSessionMaxBytes caps the serialized size of the embedded tail. Entry
	// count alone misses sessions that are few-but-huge — e.g. a transcript
	// with a handful of multi-MB file reads. Above this budget the tail is cut
	// by bytes too, so the initial payload (and the browser's parse/render) stay
	// bounded no matter how large individual entries are.
	LargeSessionMaxBytes = envInt("PI_WEB_LARGE_SESSION_MAX_BYTES", 4_000_000)
	// LargeSessionMinTailEntries is the floor the byte cap will not cut below,
	// so a giant last entry still leaves a usable window on screen.
	LargeSessionMinTailEntries = envInt("PI_WEB_LARGE_SESSION_MIN_TAIL_ENTRIES", 5)
)

// TailWindow decides which trailing slice of entries to embed on the initial
// load. It truncates when the session exceeds either the entry-count threshold
// or the byte budget, returning the start index and whether truncation
// happened. The byte walk runs newest-first and stops as soon as the budget is
// hit, so it marshals only ~one budget's worth even for a 30MB+ session.
func TailWindow(entries []map[string]any) (from int, truncated bool) {
	total := len(entries)
	if total == 0 {
		return 0, false
	}
	if total > LargeSessionThreshold {
		from = total - LargeSessionTailEntries
		if from < 0 {
			from = 0
		}
	}
	if LargeSessionMaxBytes > 0 {
		acc := 0
		byteFrom := 0
		for i := total - 1; i >= 0; i-- {
			b, _ := json.Marshal(entries[i])
			// Always keep at least LargeSessionMinTailEntries, budget or not, so a
			// giant last entry still leaves a usable window.
			mustKeep := (total - i) <= LargeSessionMinTailEntries
			if !mustKeep && acc+len(b) > LargeSessionMaxBytes {
				byteFrom = i + 1 // adding this entry tips over budget: start after it
				break
			}
			acc += len(b)
			byteFrom = i
		}
		if byteFrom > from {
			from = byteFrom
		}
	}
	if from < 0 {
		from = 0
	}
	return from, from > 0
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// prepareSessionPageData computes the payload (base64-encoded session data,
// themed CSS, and body attributes) for the static export/share snapshot.
//
// For sessions with more than LargeSessionThreshold entries we embed only the
// tail (LargeSessionTailEntries) and add { truncated, total, from } fields so
// the frontend can render a "Load earlier" banner and lazily fetch preceding
// windows. Small sessions get the full payload as before — zero behavior change.
func prepareSessionPageData(session sessions.Session, cssTemplate string) (dataBase64, css, bodyAttrs string) {
	leafID := ""
	for i := len(session.Entries) - 1; i >= 0; i-- {
		if typ, _ := session.Entries[i]["type"].(string); typ == "label" {
			continue
		}
		if id, ok := session.Entries[i]["id"].(string); ok && id != "" {
			leafID = id
			break
		}
	}

	total := len(session.Entries)
	entries := session.Entries
	from, truncated := TailWindow(session.Entries)
	if truncated {
		entries = session.Entries[from:]
	}

	sessionData := map[string]any{
		"header":        session.Header,
		"entries":       entries,
		"name":          session.Name,
		"leafId":        leafID,
		"systemPrompt":  nil,
		"tools":         nil,
		"renderedTools": nil,
		"total":         total,
		"from":          from,
		"truncated":     truncated,
	}

	dataJSON, _ := json.Marshal(sessionData)
	dataBase64 = base64.StdEncoding.EncodeToString(dataJSON)

	css = cssTemplate

	if session.SessionUUID != "" {
		bodyAttrs = ` data-session-uuid="` + session.SessionUUID + `"`
	}
	return
}
