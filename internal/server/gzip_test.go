package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddlewareCompressesJSON(t *testing.T) {
	body := strings.Repeat(`{"k":"vvvvvvvvvv"},`, 5000)
	h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", enc)
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("compressed size %d not smaller than raw %d", rec.Body.Len(), len(body))
	}
	gr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	got, _ := io.ReadAll(gr)
	if string(got) != body {
		t.Fatal("decompressed body does not match original")
	}
}

func TestGzipMiddlewareSkipsWithoutAcceptEncoding(t *testing.T) {
	h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil) // no Accept-Encoding
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("compressed despite no Accept-Encoding")
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body = %q, want hello", rec.Body.String())
	}
}

// Range requests must pass through untouched: ServeContent's 206 byte ranges
// address the raw representation and a gzipped slice would corrupt resumes.
func TestGzipMiddlewareBypassesRangeRequests(t *testing.T) {
	h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Repeat("x", 4096))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/files/download", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("range response was gzipped")
	}
}

// Bodyless statuses must not grow a gzip header/trailer body or claim an
// encoding they don't carry.
func TestGzipMiddlewareBypassesBodylessStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Header().Get("Content-Encoding") == "gzip" {
			t.Fatalf("status %d claimed gzip encoding", status)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("status %d wrote %d body bytes", status, rec.Body.Len())
		}
	}
}

// Already-compressed content types (a PNG download) must not be re-gzipped.
func TestGzipMiddlewareBypassesIncompressibleTypes(t *testing.T) {
	h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/files/download", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("png was gzipped")
	}
	if rec.Body.String() != "\x89PNG" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// SSE must stream uncompressed: a compressor would buffer/reframe the events.
func TestGzipMiddlewareBypassesEventStream(t *testing.T) {
	h := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: hi\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("event stream was gzipped")
	}
	if !strings.Contains(rec.Body.String(), "data: hi") {
		t.Fatalf("SSE body = %q", rec.Body.String())
	}
}
