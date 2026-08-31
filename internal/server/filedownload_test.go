package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleFilesDownload(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a known file structure.
	os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sub", "nested.go"), []byte("package main"), 0644)

	s := newTestServer(t)

	t.Run("downloads a file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		cd := rec.Header().Get("Content-Disposition")
		if !strings.Contains(cd, `filename="hello.txt"`) {
			t.Errorf("unexpected Content-Disposition: %s", cd)
		}
		if rec.Body.String() != "hello world" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("downloads a nested file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=sub/nested.go", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "package main" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("escapes hostile and non-ASCII filenames in Content-Disposition", func(t *testing.T) {
		name := `a"bé.txt`
		os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0644)
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file="+url.QueryEscape(name), nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		cd := rec.Header().Get("Content-Disposition")
		if !strings.Contains(cd, `filename="a_b_.txt"`) {
			t.Errorf("quoted filename not sanitized: %s", cd)
		}
		if !strings.Contains(cd, `filename*=UTF-8''a%22b%C3%A9.txt`) {
			t.Errorf("filename* not percent-encoded: %s", cd)
		}
	})

	t.Run("rejects missing file param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir, nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rejects path traversal with ..", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=../hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("rejects absolute path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=/etc/passwd", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("rejects sneaky traversal sub/../../file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=sub/../../hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=nope.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("rejects directory", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=sub", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("rejects POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost,
			"/api/files/download?path="+tmpDir+"&file=hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("sets content type from extension", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		ct := rec.Header().Get("Content-Type")
		if ct != "text/plain; charset=utf-8" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
	})

	t.Run("sets Cache-Control no-cache so browsers revalidate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/api/files/download?path="+tmpDir+"&file=hello.txt", nil)
		rec := httptest.NewRecorder()
		s.handleFilesDownload(rec, req)

		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("expected Cache-Control no-cache, got %q", cc)
		}
	})
}

func TestWithinCwd(t *testing.T) {
	t.Run("same dir is within", func(t *testing.T) {
		if !withinCwd("/a/b", "/a/b") {
			t.Error("expected true")
		}
	})

	t.Run("nested path is within", func(t *testing.T) {
		if !withinCwd("/a/b", "/a/b/c/d") {
			t.Error("expected true")
		}
	})

	t.Run("sibling is not within", func(t *testing.T) {
		if withinCwd("/a/b", "/a/c") {
			t.Error("expected false")
		}
	})

	t.Run("parent is not within", func(t *testing.T) {
		if withinCwd("/a/b", "/a") {
			t.Error("expected false")
		}
	})
}
