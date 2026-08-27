package rpc

import (
	"strings"
	"testing"
)

func TestSocketPathFor(t *testing.T) {
	a := SocketPathFor("/tmp/w", "session-a")
	b := SocketPathFor("/tmp/w", "session-b")
	if a == b {
		t.Fatalf("distinct sessions map to same socket: %s", a)
	}
	if !strings.HasPrefix(a, "/tmp/w/") || !strings.HasSuffix(a, ".sock") {
		t.Fatalf("unexpected socket path: %s", a)
	}
	if a != SocketPathFor("/tmp/w", "session-a") {
		t.Fatalf("socket path not stable for same session")
	}
}
