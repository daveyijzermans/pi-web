package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// HolderArg is the hidden subcommand that turns the pi-web binary into a
// detached worker-holder: a tiny process that owns a `pi --mode rpc` child and
// bridges its stdio to a unix socket. pi-web dials the socket instead of
// holding the pipes itself, so a server restart (dev hot-reload, deploy,
// crash) no longer kills in-flight agent turns: the holder and pi keep
// running, and the reborn server reattaches.
const HolderArg = "__worker-holder"

// holderShutdownType is the control message a client sends to terminate the
// held pi process and the holder itself (worker reaping).
const holderShutdownType = "__holder_shutdown"

// holderInfoType is the first line the holder sends on every attach.
const holderInfoType = "__holder_info"

// holderReplayEndType marks the end of the queued-while-detached backlog on
// attach. Everything before it is replay (stale for stream previews — a turn
// that finished while detached must not re-stream into the browser);
// everything after is live.
const holderReplayEndType = "__holder_replay_end"

type holderInfo struct {
	Type            string `json:"type"`
	PiPid           int    `json:"piPid"`
	StartedAt       int64  `json:"startedAt"` // unix nanos
	SessionSwitched bool   `json:"sessionSwitched"`
}

// maxQueuedLines bounds the undelivered-event queue while no client is
// attached; oldest lines are dropped first. Session content is never lost —
// pi appends it to the session file — this only bounds replayed stream noise.
const maxQueuedLines = 8192

// holderIdleTimeout self-reaps a holder that has sat detached with a silent pi
// for this long: after a server restart nobody may ever reattach, and without
// this each abandoned holder pins a pi process forever. Attach activity and pi
// output both reset the clock, so long unattended agent turns are safe.
const holderIdleTimeout = 30 * time.Minute

type holder struct {
	mu              sync.Mutex
	stdinMu         sync.Mutex
	client          net.Conn
	queue           [][]byte
	sessionSwitched bool
	pi              *exec.Cmd
	startedAt       time.Time
	socketPath      string
	lastActivity    time.Time
}

// RunHolder is the process body behind HolderArg. It never returns except on
// fatal setup errors; normal exit paths (pi exits, shutdown message, signal)
// call os.Exit after cleanup.
func RunHolder(socketPath string) error {
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("holder: listen %s: %w", socketPath, err)
	}

	pi := exec.Command("pi", piCmdArgs...)
	piStdin, err := pi.StdinPipe()
	if err != nil {
		return err
	}
	piStdout, err := pi.StdoutPipe()
	if err != nil {
		return err
	}
	pi.Stderr = nil
	if err := pi.Start(); err != nil {
		return fmt.Errorf("holder: start pi: %w", err)
	}

	h := &holder{
		pi:           pi,
		startedAt:    time.Now(),
		socketPath:   socketPath,
		lastActivity: time.Now(),
	}
	stdinWriter := piStdin

	// pi stdout -> attached client, or the bounded queue while detached.
	go func() {
		scanner := bufio.NewScanner(piStdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			h.deliver(line)
		}
		// pi closed stdout: it exited (or crashed). Nothing left to hold.
		_ = pi.Wait()
		h.cleanup()
		os.Exit(0)
	}()

	// Self-reap when detached and inactive: nobody may ever reattach.
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			h.mu.Lock()
			idle := h.client == nil && time.Since(h.lastActivity) > holderIdleTimeout
			h.mu.Unlock()
			if idle {
				h.terminate()
			}
		}
	}()

	// Signals: take pi down with us, then vanish.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		h.terminate()
	}()

	// Accept loop: one client at a time; a new attach replaces the old one
	// (a dead server's conn may linger until its next write fails). An Accept
	// error must not kill pi — it may be mid-turn writing the session file.
	// Back off and retry; if the listener is permanently dead nobody can ever
	// reattach and the idle self-reaper above ends the process once pi quiets.
	for {
		conn, err := listener.Accept()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		h.attach(conn, stdinWriter)
	}
}

func (h *holder) deliver(line []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastActivity = time.Now()
	if h.client != nil {
		if _, err := h.client.Write(append(line, '\n')); err == nil {
			return
		}
		_ = h.client.Close()
		h.client = nil
	}
	h.queue = append(h.queue, line)
	if len(h.queue) > maxQueuedLines {
		h.queue = h.queue[len(h.queue)-maxQueuedLines:]
	}
}

func (h *holder) attach(conn net.Conn, piStdin io.Writer) {
	h.mu.Lock()
	if h.client != nil {
		_ = h.client.Close()
	}
	h.client = conn
	h.lastActivity = time.Now()
	info, _ := json.Marshal(holderInfo{
		Type:            holderInfoType,
		PiPid:           h.pi.Process.Pid,
		StartedAt:       h.startedAt.UnixNano(),
		SessionSwitched: h.sessionSwitched,
	})
	ok := true
	if _, err := conn.Write(append(info, '\n')); err != nil {
		ok = false
	}
	for ok && len(h.queue) > 0 {
		line := h.queue[0]
		if _, err := conn.Write(append(line, '\n')); err != nil {
			ok = false
			break
		}
		h.queue = h.queue[1:]
	}
	if ok {
		if _, err := conn.Write([]byte(`{"type":"` + holderReplayEndType + `"}` + "\n")); err != nil {
			ok = false
		}
	}
	if !ok {
		_ = conn.Close()
		if h.client == conn {
			h.client = nil
		}
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	// client -> pi stdin, watching for control messages.
	go func() {
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			var probe struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(line, &probe)
			if probe.Type == holderShutdownType {
				h.terminate()
			}
			if probe.Type == "switch_session" {
				h.mu.Lock()
				h.sessionSwitched = true
				h.mu.Unlock()
			}
			h.stdinMu.Lock()
			_, err := piStdin.Write(append(append([]byte(nil), line...), '\n'))
			h.stdinMu.Unlock()
			if err != nil {
				h.terminate()
			}
		}
		// Client went away (server restart). Detach; keep pi running.
		h.mu.Lock()
		if h.client == conn {
			h.client = nil
		}
		h.mu.Unlock()
		_ = conn.Close()
	}()
}

func (h *holder) terminate() {
	if h.pi.Process != nil {
		_ = h.pi.Process.Kill()
	}
	h.cleanup()
	os.Exit(0)
}

func (h *holder) cleanup() {
	_ = os.Remove(h.socketPath)
}
