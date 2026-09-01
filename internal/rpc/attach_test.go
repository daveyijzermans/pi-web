package rpc

import (
	"bufio"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

// StopHolder must connect to a live holder socket and deliver the shutdown
// control message, draining the holder's handshake/replay so its reader can
// observe the shutdown. Uses a fake listener that mimics the holder wire shape.
func TestStopHolderSendsShutdownToLiveHolder(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess-stop"
	socketPath := SocketPathFor(dir, sessionID)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Handshake line + a queued replay line, like a real holder on attach.
		_, _ = conn.Write([]byte(`{"type":"` + holderInfoType + `","piPid":1234}` + "\n"))
		_, _ = conn.Write([]byte(`{"type":"message_update"}` + "\n"))
		// Read the client's control message.
		buf := bufio.NewScanner(conn)
		if buf.Scan() {
			got <- buf.Text()
		} else {
			got <- ""
		}
	}()

	stopped, err := StopHolder(dir, sessionID)
	if err != nil {
		t.Fatalf("StopHolder error: %v", err)
	}
	if !stopped {
		t.Fatal("StopHolder = false for a live holder, want true")
	}
	select {
	case line := <-got:
		if !strings.Contains(line, holderShutdownType) {
			t.Fatalf("holder received %q, want a %s control message", line, holderShutdownType)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("holder never received the shutdown message")
	}
}

// StopHolder must report (false, nil) — nothing to stop — when no holder is
// listening (no live orphaned turn; must not error or spawn anything).
func TestStopHolderNoLiveHolderIsNoop(t *testing.T) {
	dir := t.TempDir()
	stopped, err := StopHolder(dir, "absent-session")
	if err != nil {
		t.Fatalf("StopHolder error: %v", err)
	}
	if stopped {
		t.Fatal("StopHolder = true with no holder listening, want false")
	}
}

// On attach the holder must send, in order: the info line, the queued
// backlog, then the replay-end marker. The marker is the only thing that
// stops the worker suppressing stream previews — losing it would mute
// previews for the rest of the holder's life.
func TestHolderAttachSendsReplayEndAfterBacklog(t *testing.T) {
	proc, _ := os.FindProcess(os.Getpid())
	h := &holder{
		pi:              &exec.Cmd{Process: proc},
		startedAt:       time.Now(),
		sessionSwitched: true,
		queue:           [][]byte{[]byte(`{"type":"queued_one"}`), []byte(`{"type":"queued_two"}`)},
	}
	client, server := net.Pipe()
	defer client.Close()

	go h.attach(server, io.Discard)

	scanner := bufio.NewScanner(client)
	want := []string{holderInfoType, "queued_one", "queued_two", holderReplayEndType}
	for _, marker := range want {
		if !scanner.Scan() {
			t.Fatalf("stream ended before %q: %v", marker, scanner.Err())
		}
		if line := scanner.Text(); !strings.Contains(line, marker) {
			t.Fatalf("got %q, want line containing %q", line, marker)
		}
	}
	h.mu.Lock()
	remaining := len(h.queue)
	h.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("queue not drained: %d lines remain", remaining)
	}
}
