package server

import (
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// handleFilesDownload streams a file from the session's working directory as
// a browser download. Accepts ?id=<sessionId> (resolved to cwd) or ?path=<cwd>
// for the directory, and ?file=<relativePath> for the file to download.
func (s *Server) handleFilesDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cwd, err := resolveCwd(s, r)
	if resolveOrWriteError(w, err) {
		return
	}

	fileRel := r.URL.Query().Get("file")
	if fileRel == "" {
		writeJSONError(w, http.StatusBadRequest, "requires ?file= parameter")
		return
	}

	// Guard against path traversal before normalisation.
	if strings.HasPrefix(fileRel, "..") || strings.HasPrefix(fileRel, "/") {
		writeJSONError(w, http.StatusNotFound, "file not found")
		return
	}

	fileRel = filepath.Clean(fileRel)
	target := filepath.Join(cwd, fileRel)

	// Ensure the resolved target is inside cwd.
	if !withinCwd(cwd, target) {
		writeJSONError(w, http.StatusNotFound, "file not found")
		return
	}

	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "file not found")
		} else {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path is a directory")
		return
	}

	// Detect content type from extension; fallback to octet-stream.
	ext := filepath.Ext(target)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
	}

	name := filepath.Base(target)
	// Quoted filename: strip characters that would break or smuggle past the
	// quoted-string (RFC 6266); filename*: RFC 5987 percent-encoded UTF-8 for
	// non-ASCII names.
	quoted := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 0x20 || r > 0x7e {
			return '_'
		}
		return r
	}, name)
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+quoted+`"; filename*=UTF-8''`+url.PathEscape(name))
	w.Header().Set("Content-Type", ct)
	// no-cache forces the browser to revalidate against Last-Modified instead of
	// heuristically serving a stale copy, so an edited file always downloads fresh.
	w.Header().Set("Cache-Control", "no-cache")

	f, err := os.Open(target)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()

	http.ServeContent(w, r, name, info.ModTime(), f)
}

// withinCwd reports whether abs is cwd itself or nested inside it.
func withinCwd(cwd, abs string) bool {
	rel, err := filepath.Rel(cwd, abs)
	return err == nil && !strings.HasPrefix(rel, "..")
}
