package server

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPool reuses gzip.Writers across requests; a session payload is
// multi-MB and allocating a fresh compressor per request is wasteful.
var gzipWriterPool = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// GzipMiddleware compresses responses for clients that advertise gzip support.
// The big win is /api/session: it ships file/text JSON that compresses ~5-10x,
// turning a multi-MB body into a few hundred KB on the wire (the dominant cost
// on a slow link).
//
// Not compressed:
//   - Server-Sent Events — they stream incrementally and a compressor would
//     buffer/reframe them. Decided lazily on the first write from the
//     handler's Content-Type, so no path list to keep in sync.
//   - Range requests — http.ServeContent's 206 byte ranges address the raw
//     representation; encoding the slice would corrupt resumed downloads.
//   - Bodyless statuses (1xx/204/304) — net/http suppresses their bodies, so
//     a gzip header/trailer write would be dropped and Content-Encoding lie.
//   - Content types that are already compressed (images, archives, fonts,
//     audio/video) — re-gzipping wastes CPU for zero or negative gain.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	passthrough bool // SSE / already-encoded: write raw, no compression
}

// incompressible reports content types that gain nothing from gzip because
// their formats are already compressed.
func incompressible(ct string) bool {
	if strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "image/svg") {
		return true
	}
	if strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "audio/") {
		return true
	}
	for _, t := range []string{
		"application/zip", "application/gzip", "application/x-gzip",
		"application/x-7z-compressed", "application/x-rar-compressed",
		"application/zstd", "application/x-xz", "application/x-bzip2",
		"font/woff", "font/woff2", "application/font-woff",
	} {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	h := g.Header()
	ct := h.Get("Content-Type")
	bodyless := status < 200 || status == http.StatusNoContent || status == http.StatusNotModified
	if bodyless || strings.HasPrefix(ct, "text/event-stream") ||
		h.Get("Content-Encoding") != "" || incompressible(ct) {
		g.passthrough = true
		g.ResponseWriter.WriteHeader(status)
		return
	}
	h.Del("Content-Length") // compressed length is unknown up front
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(g.ResponseWriter)
	g.gz = gz
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.passthrough {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

// Flush passes through so streaming handlers (SSE, live install logs) keep
// working; when compressing it flushes a gzip sync point first.
func (g *gzipResponseWriter) Flush() {
	if g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack passes through for any handler that upgrades the connection.
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := g.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("gzip: underlying ResponseWriter is not a Hijacker")
}

func (g *gzipResponseWriter) close() {
	if g.gz != nil {
		_ = g.gz.Close()
		gzipWriterPool.Put(g.gz)
		g.gz = nil
	}
}
