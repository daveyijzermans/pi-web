package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pi-web/internal/git"
	"pi-web/internal/sessions"
)

var (
	errEmptyPath    = errors.New("path is required")
	errRelativePath = errors.New("path must be absolute")
)

// projectPrefsSchema creates the table that records which projects are shown on
// the index page. A project is "enabled" when it should appear; new projects
// discovered after the first run default to disabled (allowlist), while the
// very first run seeds every existing project as enabled so the homepage looks
// unchanged until the user starts curating.
const projectPrefsSchema = `CREATE TABLE IF NOT EXISTS project_prefs (
	project_path TEXT PRIMARY KEY,
	enabled INTEGER NOT NULL DEFAULT 1,
	source TEXT NOT NULL DEFAULT 'discovered',
	updated_at DATETIME
)`

// appSettingsSchema holds simple key/value app preferences. Currently only the
// project-filter master switch.
const appSettingsSchema = `CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT
)`

// projectsSchema stores project-level metadata: display name, GitHub repo slug,
// and a short description extracted from the README or GitHub API.
const projectsSchema = `CREATE TABLE IF NOT EXISTS projects (
	project_path TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	repo TEXT,
	readme_description TEXT,
	created_at DATETIME,
	updated_at DATETIME
)`

const settingProjectFilterEnabled = "project_filter_enabled"

// projectFilterEnabled reports whether the homepage should be filtered to only
// enabled projects. Off by default: with the filter off every project (and any
// new session) shows up normally.
func (s *Server) projectFilterEnabled() bool {
	if s.db == nil {
		return false
	}
	var v string
	if err := s.db.QueryRow("SELECT value FROM app_settings WHERE key = ?", settingProjectFilterEnabled).Scan(&v); err != nil {
		return false
	}
	return v == "1"
}

func (s *Server) setProjectFilterEnabled(enabled bool) {
	if s.db == nil {
		return
	}
	v := "0"
	if enabled {
		v = "1"
	}
	_, _ = s.db.Exec(`INSERT INTO app_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, settingProjectFilterEnabled, v)
}

type projectEntry struct {
	Path              string   `json:"path"`
	Name              string   `json:"name"`
	Enabled           bool     `json:"enabled"`
	SessionCount      int      `json:"sessionCount"`
	Source            string   `json:"source"`
	RunningSessionIDs []string `json:"runningSessionIds,omitempty"`
}

// distinctProjects returns the unique, non-empty project paths in first-seen
// order.
func distinctProjects(summaries []sessions.SessionSummary) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, sum := range summaries {
		if sum.Project == "" || seen[sum.Project] {
			continue
		}
		seen[sum.Project] = true
		out = append(out, sum.Project)
	}
	return out
}

// migrateProjectPaths rewrites every project_path to its canonical form
// (forward slashes, lowercased Windows drive paths) and merges rows that
// collapse to the same canonical path, preferring an enabled row. Project keys
// must match sessions.CanonicalProject so prefs line up with the project keys
// derived from session cwds. Idempotent: canonical paths are left unchanged.
func (s *Server) migrateProjectPaths() {
	if s.db == nil {
		return
	}
	rows, err := s.db.Query("SELECT rowid, project_path, enabled FROM project_prefs")
	if err != nil {
		return
	}
	type prefRow struct {
		rowid   int64
		path    string
		enabled int
	}
	var all []prefRow
	for rows.Next() {
		var r prefRow
		if err := rows.Scan(&r.rowid, &r.path, &r.enabled); err != nil {
			continue
		}
		all = append(all, r)
	}
	rows.Close()

	// Pick one surviving row per canonical path, preferring an enabled row so a
	// user's enable choice is never lost when merging a backslash/case variant.
	type survivor struct {
		rowid   int64
		enabled int
	}
	winners := make(map[string]survivor)
	for _, r := range all {
		key := sessions.CanonicalProject(r.path)
		if w, ok := winners[key]; ok && !(r.enabled == 1 && w.enabled == 0) {
			continue
		}
		winners[key] = survivor{rowid: r.rowid, enabled: r.enabled}
	}
	winRowids := make(map[int64]bool, len(winners))
	for _, w := range winners {
		winRowids[w.rowid] = true
	}
	// Delete losers first so rewriting a survivor to its canonical path can't
	// collide with a duplicate that is about to be removed.
	for _, r := range all {
		if !winRowids[r.rowid] {
			_, _ = s.db.Exec("DELETE FROM project_prefs WHERE rowid = ?", r.rowid)
		}
	}
	for key, w := range winners {
		_, _ = s.db.Exec("UPDATE project_prefs SET project_path = ? WHERE rowid = ?", key, w.rowid)
	}
}

// syncProjectPrefs records any not-yet-tracked discovered projects. On the very
// first run (empty table) every discovered project is enabled; afterwards new
// projects are inserted disabled so they stay hidden until the user enables
// them. Existing rows are never modified.
func (s *Server) syncProjectPrefs(discovered []string) {
	if s.db == nil || len(discovered) == 0 {
		return
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM project_prefs").Scan(&count); err != nil {
		return
	}
	defaultEnabled := 0
	if count == 0 {
		defaultEnabled = 1
	}
	now := s.now()
	for _, p := range discovered {
		if p == "" {
			continue
		}
		_, _ = s.db.Exec(`INSERT INTO project_prefs (project_path, enabled, source, updated_at)
			VALUES (?, ?, 'discovered', ?)
			ON CONFLICT(project_path) DO NOTHING`, p, defaultEnabled, now)
	}
}

// detectGithubRepo runs `git remote get-url origin` and extracts the
// owner/repo slug. Returns "" on any error.
func detectGithubRepo(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	slug, ok := git.GithubSlug(strings.TrimSpace(string(out)))
	if !ok {
		return ""
	}
	return slug
}

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`[ \t]+`)
)

// extractReadmeDescription reads the README in dir and extracts a clean
// prose description. Priority: (1) any heading containing "short description"
// or exactly "description", (2) first clean prose paragraph in the document,
// (3) "" if nothing clean is found.
func extractReadmeDescription(dir string) string {
	var content string
	for _, name := range []string{"README.md", "README.rst"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			content = string(data)
			break
		}
	}
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")

	// Priority 1: heading containing "short description" or exactly "description"
	headingIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		headingText := strings.TrimSpace(strings.TrimLeft(trimmed, "# "))
		lower := strings.ToLower(headingText)
		if strings.Contains(lower, "short description") || lower == "description" {
			headingIdx = i
			break
		}
	}
	if headingIdx >= 0 {
		var buf strings.Builder
		for i := headingIdx + 1; i < len(lines); i++ {
			l := strings.TrimSpace(lines[i])
			if l == "" {
				continue
			}
			if strings.HasPrefix(l, "#") {
				break
			}
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(l)
		}
		if result := cleanText(buf.String()); result != "" {
			return result
		}
		return ""
	}

	// Priority 2: first clean prose paragraph
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || isHTMLLine(trimmed) || isImageLine(trimmed) {
			i++
			continue
		}
		// collect paragraph
		var buf strings.Builder
		for i < len(lines) {
			l := strings.TrimSpace(lines[i])
			if l == "" || strings.HasPrefix(l, "#") {
				break
			}
			if isHTMLLine(l) || isImageLine(l) {
				i++
				continue
			}
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(l)
			i++
		}
		if result := cleanText(buf.String()); result != "" {
			return result
		}
	}
	return ""
}

// isHTMLLine reports whether a trimmed line is an HTML tag.
func isHTMLLine(trimmed string) bool {
	return len(trimmed) > 0 && trimmed[0] == '<'
}

// isImageLine reports whether a trimmed line is a markdown image or a link
// wrapping only an image file.
func isImageLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "![") {
		return true
	}
	// check for [text](image.png) pattern
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != '[' {
			continue
		}
		closeBracket := -1
		for j := i + 1; j < len(trimmed); j++ {
			if trimmed[j] == ']' {
				closeBracket = j
				break
			}
		}
		if closeBracket < 0 || closeBracket+1 >= len(trimmed) || trimmed[closeBracket+1] != '(' {
			continue
		}
		parenStart := closeBracket + 2
		for j := parenStart; j < len(trimmed); j++ {
			if trimmed[j] == ')' {
				url := trimmed[parenStart:j]
				lower := strings.ToLower(url)
				if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") ||
					strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") ||
					strings.HasSuffix(lower, ".webp") || strings.HasSuffix(lower, ".svg") ||
					strings.HasSuffix(lower, ".ico") || strings.HasSuffix(lower, ".bmp") {
					return true
				}
				break
			}
		}
	}
	return false
}

// cleanText strips HTML tags, reduces markdown links to text, collapses
// whitespace, and trims.
func cleanText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	// replace [text](url) with text
	for i := 0; i < len(s); {
		if s[i] != '[' {
			i++
			continue
		}
		closeBracket := -1
		for j := i + 1; j < len(s); j++ {
			if s[j] == '[' {
				continue
			}
			if s[j] == ']' {
				closeBracket = j
				break
			}
		}
		if closeBracket < 0 {
			i++
			continue
		}
		if closeBracket+1 >= len(s) || s[closeBracket+1] != '(' {
			i = closeBracket + 1
			continue
		}
		parenStart := closeBracket + 2
		for j := parenStart; j < len(s); j++ {
			if s[j] == ')' && (j == parenStart || s[j-1] != '\\') {
				linkText := s[i+1 : closeBracket]
				s = s[:i] + linkText + s[j+1:]
				i = i + 1 + len(linkText)
				break
			}
		}
	}
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// enabledProjectSet returns the set of enabled project paths. The second return
// value is false when preferences are unavailable (no database), in which case
// callers should treat every project as enabled.
func (s *Server) enabledProjectSet() (map[string]bool, bool) {
	if s.db == nil {
		return nil, false
	}
	rows, err := s.db.Query("SELECT project_path FROM project_prefs WHERE enabled = 1")
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	set := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, false
		}
		set[p] = true
	}
	return set, true
}

// filterEnabledSummaries drops sessions whose project is disabled. Sessions with
// an empty project are always kept. With no database it is a no-op.
func (s *Server) filterEnabledSummaries(summaries []sessions.SessionSummary) []sessions.SessionSummary {
	if s.db == nil || !s.projectFilterEnabled() {
		return summaries
	}
	s.syncProjectPrefs(distinctProjects(summaries))
	enabled, ok := s.enabledProjectSet()
	if !ok {
		return summaries
	}
	out := make([]sessions.SessionSummary, 0, len(summaries))
	for _, sum := range summaries {
		if sum.Project == "" || enabled[sum.Project] {
			out = append(out, sum)
		}
	}
	return out
}

func (s *Server) handleApiProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	summaries, err := s.loadSummaries()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()
	currentProject := q.Get("current")
	currentSessionLimit, _ := strconv.Atoi(q.Get("sessionLimit"))
	currentSessions := make([]sessions.SessionSummary, 0)
	currentSessionsTotal := 0
	if currentProject != "" && currentSessionLimit > 0 && (q.Get("offset") == "" || q.Get("offset") == "0") {
		for _, sum := range s.filterBtwSummaries(summaries) {
			if sum.Project != currentProject {
				continue
			}
			currentSessionsTotal++
			if len(currentSessions) < currentSessionLimit {
				currentSessions = append(currentSessions, sum)
			}
		}
	}

	s.lastKnownMu.Lock()
	runningSessionIDs := make(map[string]bool, len(s.lastKnown))
	for id := range s.lastKnown {
		runningSessionIDs[id] = true
	}
	s.lastKnownMu.Unlock()

	counts := make(map[string]int)
	runningByProject := make(map[string][]string)
	for _, sum := range summaries {
		if sum.Project != "" {
			counts[sum.Project]++
			if runningSessionIDs[sum.ID] {
				runningByProject[sum.Project] = append(runningByProject[sum.Project], sum.ID)
			}
		}
	}
	s.syncProjectPrefs(distinctProjects(summaries))

	enabled := make(map[string]bool)
	source := make(map[string]string)
	names := make(map[string]string)
	if s.db != nil {
		rows, err := s.db.Query("SELECT project_path, enabled, source FROM project_prefs")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p, src string
				var en int
				if err := rows.Scan(&p, &en, &src); err != nil {
					continue
				}
				enabled[p] = en == 1
				source[p] = src
			}
		}
		// Load project display names
		rows2, err := s.db.Query("SELECT project_path, name FROM projects")
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				var p, n string
				if err := rows2.Scan(&p, &n); err != nil {
					continue
				}
				names[p] = n
			}
		}
	}

	// Union of projects that have sessions and projects recorded in prefs
	// (e.g. registered paths without sessions yet).
	paths := make(map[string]bool)
	for p := range counts {
		paths[p] = true
	}
	for p := range enabled {
		paths[p] = true
	}

	entries := make([]projectEntry, 0, len(paths))
	for p := range paths {
		src := source[p]
		if src == "" {
			src = "discovered"
		}
		en := enabled[p]
		// Without a database we cannot persist prefs; report everything enabled.
		if s.db == nil {
			en = true
		}
		name := names[p]
		if name == "" {
			name = filepath.Base(p)
		}
		entries = append(entries, projectEntry{
			Path:              p,
			Name:              name,
			Enabled:           en,
			SessionCount:      counts[p],
			Source:            src,
			RunningSessionIDs: runningByProject[p],
		})
	}
	// filtered=1 applies the Manage Projects allowlist (used by the session
	// sidebar). The current project is always kept so the project you are in
	// never disappears; the modal omits the param to keep seeing everything.
	if q.Get("filtered") == "1" && s.projectFilterEnabled() {
		kept := make([]projectEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.Enabled || entry.Path == currentProject {
				kept = append(kept, entry)
			}
		}
		entries = kept
	}

	sort.Slice(entries, func(i, j int) bool {
		if (entries[i].Path == currentProject) != (entries[j].Path == currentProject) {
			return entries[i].Path == currentProject
		}
		if entries[i].SessionCount != entries[j].SessionCount {
			return entries[i].SessionCount > entries[j].SessionCount
		}
		return entries[i].Path < entries[j].Path
	})

	total := len(entries)
	entries = paginateProjectEntries(entries, q.Get("offset"), q.Get("limit"))

	writeJSON(w, 0, map[string]any{
		"projects":             entries,
		"total":                total,
		"currentSessions":      currentSessions,
		"currentSessionsTotal": currentSessionsTotal,
		"filterEnabled":        s.projectFilterEnabled(),
	})
}

func paginateProjectEntries(entries []projectEntry, offsetStr, limitStr string) []projectEntry {
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return entries
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	if offset >= len(entries) {
		return []projectEntry{}
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return entries[offset:end]
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Path   string `json:"path"`
		Action string `json:"action"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}
	if s.db == nil {
		writeJSONError(w, http.StatusInternalServerError, "preferences are unavailable")
		return
	}

	if body.Action == "enable-filter" || body.Action == "disable-filter" {
		s.setProjectFilterEnabled(body.Action == "enable-filter")
		writeJSON(w, 0, map[string]any{"ok": true, "filterEnabled": s.projectFilterEnabled()})
		return
	}

	if body.Action == "enable-all" || body.Action == "disable-all" {
		s.setAllProjectsEnabled(body.Action == "enable-all")
		writeJSON(w, 0, map[string]any{"ok": true})
		return
	}

	path := body.Path
	if body.Action == "register" {
		normalized, err := normalizeProjectPath(path)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		path = normalized
	}
	if strings.TrimSpace(path) == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}

	if body.Action == "delete-sessions" {
		deleted, err := s.deleteProjectSessions(path)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to delete sessions: "+err.Error())
			return
		}
		writeJSON(w, 0, map[string]any{"ok": true, "path": path, "deleted": deleted})
		return
	}

	now := s.now()
	var err error
	switch body.Action {
	case "enable":
		_, err = s.db.Exec(`INSERT INTO project_prefs (project_path, enabled, source, updated_at)
			VALUES (?, 1, 'discovered', ?)
			ON CONFLICT(project_path) DO UPDATE SET enabled=1, updated_at=excluded.updated_at`, path, now)
	case "disable":
		_, err = s.db.Exec(`INSERT INTO project_prefs (project_path, enabled, source, updated_at)
			VALUES (?, 0, 'discovered', ?)
			ON CONFLICT(project_path) DO UPDATE SET enabled=0, updated_at=excluded.updated_at`, path, now)
	case "register":
		_, err = s.db.Exec(`INSERT INTO project_prefs (project_path, enabled, source, updated_at)
			VALUES (?, 1, 'registered', ?)
			ON CONFLICT(project_path) DO UPDATE SET enabled=1, updated_at=excluded.updated_at`, path, now)
	case "remove":
		_, err = s.db.Exec("DELETE FROM project_prefs WHERE project_path = ?", path)
		if err == nil {
			_, _ = s.db.Exec("DELETE FROM projects WHERE project_path = ?", path)
		}
	default:
		writeJSONError(w, http.StatusBadRequest, "unknown action")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to update project: "+err.Error())
		return
	}
	writeJSON(w, 0, map[string]any{"ok": true, "path": path})
}

// deleteProjectSessions deletes every session file whose project matches path,
// evicting each from the cache and broadcasting a delete so open clients update.
// Its project_prefs/projects rows are left intact — the project simply drops to
// zero sessions; use the "remove" action to drop it from the list. Returns the
// number of sessions deleted.
func (s *Server) deleteProjectSessions(path string) (int, error) {
	summaries, err := s.loadSummaries()
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, sum := range summaries {
		if sum.Project != path {
			continue
		}
		var resolved sessions.ResolvedSession
		if s.cache != nil {
			resolved, err = s.cache.Resolve(s.sessionsDir, sum.ID)
		} else {
			resolved, err = sessions.ResolveByID(s.sessionsDir, sum.ID)
		}
		if err != nil {
			return deleted, err
		}
		if err := sessions.DeleteSession(resolved.Path); err != nil {
			return deleted, err
		}
		if s.cache != nil {
			s.cache.Remove(resolved.Session.ID)
		}
		s.broadcast(resolved.Session.ID, "deleted")
		deleted++
	}
	return deleted, nil
}

// setAllProjectsEnabled flips every known project (discovered ∪ registered) to
// enabled or disabled in one shot. Discovered projects are synced first so they
// exist as rows before the bulk update.
func (s *Server) setAllProjectsEnabled(enabled bool) {
	if s.db == nil {
		return
	}
	if summaries, err := s.loadSummaries(); err == nil {
		s.syncProjectPrefs(distinctProjects(summaries))
	}
	val := 0
	if enabled {
		val = 1
	}
	_, _ = s.db.Exec("UPDATE project_prefs SET enabled = ?, updated_at = ?", val, s.now())
}

// normalizeProjectPath expands a leading ~ and cleans the path so a registered
// project matches the cwd recorded in future session headers.
func normalizeProjectPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errEmptyPath
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", errRelativePath
	}
	return sessions.CanonicalProject(path), nil
}

// handleGetProject returns project metadata, live git info, open issues/PRs
// for a single project. Route: GET /api/project/<path>
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/project/")
	path = pathValue(path)
	if strings.TrimSpace(path) == "" {
		writeJSONError(w, http.StatusBadRequest, "project path required")
		return
	}
	if s.db == nil {
		writeJSONError(w, http.StatusNotFound, "project not found")
		return
	}

	// COALESCE: repo/readme_description are nullable (a rename inserts only the
	// name), and scanning NULL into a plain string errors — which would silently
	// discard the row and with it the user's saved name.
	var name, repo, readmeDesc string
	err := s.db.QueryRow(
		"SELECT name, COALESCE(repo, ''), COALESCE(readme_description, '') FROM projects WHERE project_path = ?",
		path).Scan(&name, &repo, &readmeDesc)
	if err != nil || name == "" {
		name = filepath.Base(path)
	}
	// Fill in whatever the row didn't provide (or the row was absent entirely).
	if repo == "" {
		if _, err := os.Stat(filepath.Join(path, ".git", "config")); err == nil {
			repo = detectGithubRepo(path)
		}
	}
	if readmeDesc == "" {
		readmeDesc = extractReadmeDescription(path)
	}
	if readmeDesc == "" {
		readmeDesc = git.RepoDescription(path)
	}

	summaries, err := s.loadSummaries()
	var sessionCount int
	if err == nil {
		for _, sum := range summaries {
			if sum.Project == path {
				sessionCount++
			}
		}
	}

	// Each of these shells out (git / gh, the latter with a 4s network
	// timeout); run them concurrently so the panel opens in one round-trip's
	// worth of latency instead of the sum.
	var (
		gitInfo git.Info
		issues  []git.IssueInfo
		prs     []git.IssueInfo
		wg      sync.WaitGroup
	)
	wg.Add(3)
	go func() { defer wg.Done(); gitInfo, _ = git.Describe(path) }()
	go func() { defer wg.Done(); issues = git.OpenIssues(path) }()
	go func() { defer wg.Done(); prs = git.OpenPRs(path) }()
	wg.Wait()

	writeJSON(w, 0, map[string]any{
		"project": map[string]any{
			"path":              path,
			"name":              name,
			"repo":              repo,
			"readmeDescription": readmeDesc,
		},
		"gitInfo":      gitInfo,
		"openIssues":   issues,
		"openPRs":      prs,
		"sessionCount": sessionCount,
	})
}

// handleUpdateProjectName updates the display name for a project.
// Route: POST /api/project/update with body {"path": "...", "name": "..."}
func (s *Server) handleUpdateProjectName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if s.db == nil {
		writeJSONError(w, http.StatusInternalServerError, "preferences are unavailable")
		return
	}
	path := strings.TrimSpace(body.Path)
	name := strings.TrimSpace(body.Name)
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "path is required")
		return
	}
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := s.now()
	_, err := s.db.Exec(`INSERT INTO projects (project_path, name, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(project_path) DO UPDATE SET name=excluded.name, updated_at=excluded.updated_at`,
		path, name, now)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to update project name: "+err.Error())
		return
	}
	writeJSON(w, 0, map[string]any{"ok": true, "name": name, "path": path})
}

// pathValue URL-decodes a path segment.
func pathValue(p string) string {
	v, err := url.PathUnescape(p)
	if err != nil {
		return p
	}
	return v
}
