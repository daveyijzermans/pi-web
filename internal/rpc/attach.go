package rpc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pi-web/internal/workers"
)

// SocketPathFor maps a session ID to its worker-holder socket path.
func SocketPathFor(socketDir, sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(socketDir, hex.EncodeToString(sum[:8])+".sock")
}

// StopHolder reaches a session's live worker-holder directly and tells it to
// shut down (which SIGKILLs its pi). This is the Stop path for a turn whose
// in-server worker was lost — e.g. a pi-web restart mid-turn leaves the holder
// + pi running detached, invisible to the manager's Abort. Returns (true,nil)
// when a live holder was found and told to die; (false,nil) when no holder is
// listening (nothing to stop); a non-nil error only on an unexpected I/O
// failure after connecting.
//
// It drains the holder's handshake/replay bytes so the holder's write side
// can't block its reader goroutine from seeing the shutdown control message;
// the drain returns when the holder exits (EOF), bounded by a deadline.
func StopHolder(socketDir, sessionID string) (bool, error) {
	conn, err := dialHolder(SocketPathFor(socketDir, sessionID))
	if err != nil {
		return false, nil // no live holder for this session
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(`{"type":"` + holderShutdownType + `"}` + "\n")); err != nil {
		return false, fmt.Errorf("stop holder %s: %w", sessionID, err)
	}
	_, _ = io.Copy(io.Discard, conn)
	return true, nil
}

// NewSocketWorkerWithStream returns a ChatWorker whose pi process lives in a
// detached worker-holder (see RunHolder) instead of as a child of this
// process. If the session's holder is already running — e.g. after a pi-web
// restart mid-turn — it reattaches and replays whatever the holder queued;
// otherwise it spawns a fresh holder.
func NewSocketWorkerWithStream(sessionID, sessionPath, socketDir string, streamSink StreamEventSink) (workers.ChatWorker, error) {
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return nil, err
	}
	socketPath := SocketPathFor(socketDir, sessionID)
	conn, err := dialHolder(socketPath)
	if err != nil {
		if err = spawnHolder(socketPath); err != nil {
			return nil, err
		}
		if conn, err = awaitHolder(socketPath, 15*time.Second); err != nil {
			return nil, err
		}
	}

	info, eventStream, err := readHolderInfo(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	worker := &piRPCWorker{
		sessionPath:   sessionPath,
		startedAt:     time.Unix(0, info.StartedAt),
		conn:          conn,
		piPid:         info.PiPid,
		stdin:         conn,
		status:        workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending:       make(map[string]chan response),
		stderrBuf:     &strings.Builder{},
		streamSink:    streamSink,
		streamPreview: &streamPreviewAccumulator{},
	}
	// The holder replays its detached backlog first; suppress preview
	// broadcasts until its replay-end marker so finished turns don't
	// re-stream into the browser.
	worker.replaying = true
	go worker.consume(eventStream)

	if !info.SessionSwitched {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := worker.switchSession(ctx); err != nil {
			_ = worker.Close()
			return nil, err
		}
	}
	stateCtx, stateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, _ = worker.GetState(stateCtx)
	stateCancel()
	worker.touch()
	return worker, nil
}

func dialHolder(socketPath string) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, time.Second)
}

// awaitHolder polls the socket until the freshly spawned holder listens.
func awaitHolder(socketPath string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := dialHolder(socketPath)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("worker-holder did not come up on %s: %w", socketPath, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func spawnHolder(socketPath string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi executable not found: %w", err)
	}
	cmd := exec.Command(self, HolderArg, socketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the immediate child when it exits; the holder itself lives in its
	// own session and survives us.
	go func() { _ = cmd.Wait() }()
	return nil
}

// readHolderInfo consumes the handshake line and returns the event stream to
// hand to consume(): any bytes bufio read past the info line belong to the
// stream (the holder's writes can coalesce) and are replayed ahead of the conn.
func readHolderInfo(conn net.Conn) (holderInfo, io.Reader, error) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})
	reader := bufio.NewReaderSize(conn, 64*1024)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return holderInfo{}, nil, fmt.Errorf("worker-holder handshake: %w", err)
	}
	var info holderInfo
	if err := json.Unmarshal(line, &info); err != nil || info.Type != holderInfoType {
		return holderInfo{}, nil, fmt.Errorf("worker-holder handshake: unexpected first line %q", line)
	}
	buffered := make([]byte, reader.Buffered())
	if len(buffered) > 0 {
		_, _ = io.ReadFull(reader, buffered)
	}
	return info, io.MultiReader(bytes.NewReader(buffered), conn), nil
}
