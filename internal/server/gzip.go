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
// Server-Sent Events must NOT be compressed: they stream incrementally and a
// compressor would buffer/reframe them. The decision is made lazily on the
// first write from the handler's Content-Type, so any handler that sets
// text/event-stream bypasses compression automatically — no path list to keep
// in sync.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
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

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	h := g.Header()
	ct := h.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") || h.Get("Content-Encoding") != "" {
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
