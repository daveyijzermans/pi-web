package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"pi-web/internal/chat"
	"pi-web/internal/workers"
)

type nopWriteCloser struct{ w io.Writer }

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.w.Write(p) }
func (n nopWriteCloser) Close() error                { return nil }

func waitForPending(t *testing.T, w *piRPCWorker, id string) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		w.mu.Lock()
		_, ok := w.pending[id]
		w.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("pending request %q never registered", id)
}

func TestBangCommand(t *testing.T) {
	tests := []struct {
		name    string
		req     chat.Request
		wantCmd string
		wantExc bool
		wantOK  bool
	}{
		{"plain prompt", chat.Request{Message: "hello"}, "", false, false},
		{"bang", chat.Request{Message: "!src-status"}, "src-status", false, true},
		{"bang with args", chat.Request{Message: "! git log --oneline -3"}, "git log --oneline -3", false, true},
		{"double bang excluded", chat.Request{Message: "!!ls -la"}, "ls -la", true, true},
		{"bare bang", chat.Request{Message: "!"}, "", false, false},
		{"bang with image is a prompt", chat.Request{Message: "!ls", Images: []chat.Image{{}}}, "", false, false},
		{"bang with file is a prompt", chat.Request{Message: "!ls", Files: []chat.UploadedFile{{}}}, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, exc, ok := bangCommand(tt.req)
			if cmd != tt.wantCmd || exc != tt.wantExc || ok != tt.wantOK {
				t.Fatalf("bangCommand() = (%q, %v, %v), want (%q, %v, %v)", cmd, exc, ok, tt.wantCmd, tt.wantExc, tt.wantOK)
			}
		})
	}
}

func TestPromptSendsBashCommandForBangInput(t *testing.T) {
	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}
	done := make(chan error, 1)
	go func() { done <- w.Prompt(context.Background(), chat.Request{Message: "!!src-status"}) }()
	waitForPending(t, w, "req-1")
	w.handleRPCLine(`{"type":"response","id":"req-1","success":true}`)
	if err := <-done; err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sent); err != nil {
		t.Fatalf("unmarshal sent command: %v", err)
	}
	if sent["type"] != "bash" || sent["command"] != "src-status" || sent["excludeFromContext"] != true {
		t.Fatalf("sent = %#v", sent)
	}
	if got := w.Status(); got.State != workers.WorkerStateIdle {
		t.Fatalf("status after bash = %q, want idle (no agent turn)", got.State)
	}
}

func TestStatusReportsRunningDuringRecentStreamActivity(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`)

	if got := w.Status(); got.State != workers.WorkerStateRunning {
		t.Fatalf("status = %q, want running", got.State)
	}
}

func TestStatusReturnsIdleAfterAgentEnd(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`)
	w.handleRPCLine(`{"type":"agent_end"}`)

	if got := w.Status(); got.State != workers.WorkerStateIdle {
		t.Fatalf("status = %q, want idle", got.State)
	}
}

func TestStatusDoesNotStayRunningAfterStreamActivityExpires(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`)
	time.Sleep(2200 * time.Millisecond)

	if got := w.Status(); got.State != workers.WorkerStateIdle {
		t.Fatalf("status = %q, want idle after stream activity expires", got.State)
	}
}

func TestHandleRPCLineTracksTurnEndAsStreamActivity(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{"type":"turn_end"}`)

	if got := w.Status(); got.State != workers.WorkerStateRunning {
		t.Fatalf("status = %q, want running", got.State)
	}
}

func TestHandleRPCLineEmitsStreamPreviewCallbacks(t *testing.T) {
	var previews []StreamPreview
	w := &piRPCWorker{
		status:        workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending:       make(map[string]chan response),
		streamSink:    func(preview StreamPreview) { previews = append(previews, preview) },
		streamPreview: &streamPreviewAccumulator{},
	}

	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hel"}}`)
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"lo"}}`)

	if len(previews) != 2 {
		t.Fatalf("previews = %d, want 2", len(previews))
	}
	if previews[0].Content != "hel" || previews[0].Done {
		t.Fatalf("first preview = %+v", previews[0])
	}
	if previews[1].Content != "hello" || previews[1].Done {
		t.Fatalf("second preview = %+v", previews[1])
	}
}

func TestHandleRPCLineEmitsDonePreviewOnAgentEnd(t *testing.T) {
	var previews []StreamPreview
	w := &piRPCWorker{
		status:        workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending:       make(map[string]chan response),
		streamSink:    func(preview StreamPreview) { previews = append(previews, preview) },
		streamPreview: &streamPreviewAccumulator{},
	}

	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"hello"}}`)
	w.handleRPCLine(`{"type":"agent_end"}`)

	if len(previews) != 2 {
		t.Fatalf("previews = %d, want 2", len(previews))
	}
	if previews[1].Content != "hello" || !previews[1].Done {
		t.Fatalf("done preview = %+v", previews[1])
	}
}

func TestHandleRPCLineTracksMessageEndAsStreamActivity(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{"type":"message_end"}`)

	if got := w.Status(); got.State != workers.WorkerStateRunning {
		t.Fatalf("status = %q, want running", got.State)
	}
}

func TestGetCommandsReturnsCachedWithoutRPC(t *testing.T) {
	w := &piRPCWorker{
		pending:        make(map[string]chan response),
		commands:       []workers.SlashCommand{{Name: "skill:memory", Source: "skill"}},
		commandsCached: true,
	}
	// stdin is nil: the cache path must not attempt any RPC write.
	got, err := w.GetCommands(context.Background())
	if err != nil {
		t.Fatalf("GetCommands error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "skill:memory" {
		t.Fatalf("got = %#v", got)
	}
}

func TestGetCommandsParsesResponseAndCaches(t *testing.T) {
	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		pending: make(map[string]chan response),
	}

	type result struct {
		cmds []workers.SlashCommand
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		cmds, err := w.GetCommands(context.Background())
		resCh <- result{cmds, err}
	}()

	waitForPending(t, w, "req-1")
	w.handleRPCLine(`{"type":"response","id":"req-1","command":"get_commands","success":true,"data":{"commands":[{"name":"skill:memory","description":"mem","source":"skill"},{"name":"btw","description":"side chat","source":"extension"}]}}`)

	got := <-resCh
	if got.err != nil {
		t.Fatalf("GetCommands error: %v", got.err)
	}
	if len(got.cmds) != 2 {
		t.Fatalf("commands = %#v", got.cmds)
	}
	if got.cmds[0].Name != "skill:memory" || got.cmds[0].Source != "skill" || got.cmds[0].Description != "mem" {
		t.Fatalf("first command = %#v", got.cmds[0])
	}

	// Second call must hit the cache: no further RPC write to stdin.
	buf.Reset()
	cached, err := w.GetCommands(context.Background())
	if err != nil {
		t.Fatalf("cached GetCommands error: %v", err)
	}
	if len(cached) != 2 {
		t.Fatalf("cached commands = %#v", cached)
	}
	if buf.Len() != 0 {
		t.Fatalf("cache hit wrote to stdin: %q", buf.String())
	}
}

func TestHandleRPCLineIgnoresMalformedJSON(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	w.handleRPCLine(`{not-json}`)

	if got := w.Status(); got.State != workers.WorkerStateIdle {
		t.Fatalf("status = %q, want idle", got.State)
	}
}

func TestHandleRPCLineTracksThinkingAndTextStreamEvents(t *testing.T) {
	w := &piRPCWorker{
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}

	for _, line := range []string{
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_end"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_start"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_end","content":"done"}}`,
	} {
		w.handleRPCLine(line)
		if got := w.Status(); got.State != workers.WorkerStateRunning {
			t.Fatalf("line %s => status = %q, want running", strings.TrimSpace(line), got.State)
		}
	}
}

func TestPiCommandArgsIncludesApprove(t *testing.T) {
	if !slices.Contains(piCmdArgs, "--approve") {
		t.Fatal("pi command args missing --approve flag; project-local files (skills, extensions) will not be loaded")
	}
	if !slices.Contains(piCmdArgs, "--mode") || !slices.Contains(piCmdArgs, "rpc") {
		t.Fatal("pi command args must include --mode rpc")
	}
}

// A wedged worker (RPC loop dead, abort never acked) must not hang the cancel
// endpoint: Abort kills the worker and reports success — the jam is cleared.
func TestAbortKillsUnresponsiveWorker(t *testing.T) {
	oldTimeout := abortResponseTimeout
	abortResponseTimeout = 50 * time.Millisecond
	defer func() { abortResponseTimeout = oldTimeout }()

	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}

	if err := w.Abort(context.Background()); err != nil {
		t.Fatalf("Abort on wedged worker = %v, want nil (kill + success)", err)
	}
	if got := w.Status(); got.State != workers.WorkerStateError {
		t.Fatalf("status after killed abort = %q, want error", got.State)
	}
}

func TestAbortSetsIdleOnAck(t *testing.T) {
	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}
	done := make(chan error, 1)
	go func() { done <- w.Abort(context.Background()) }()
	waitForPending(t, w, "req-1")
	w.handleRPCLine(`{"type":"response","id":"req-1","success":true}`)
	if err := <-done; err != nil {
		t.Fatalf("Abort error: %v", err)
	}
	if got := w.Status(); got.State != workers.WorkerStateIdle {
		t.Fatalf("status after acked abort = %q, want idle", got.State)
	}
}

// A worker stuck "running" whose get_state probe times out is wedged: kill it
// so the status flips to error (reads as not-running) instead of pinning the
// session "running" forever.
func TestProbeIfWedgedKillsUnresponsiveRunningWorker(t *testing.T) {
	oldTimeout := wedgeProbeTimeout
	wedgeProbeTimeout = 50 * time.Millisecond
	defer func() { wedgeProbeTimeout = oldTimeout }()

	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}

	if !w.ProbeIfWedged(time.Now()) {
		t.Fatal("ProbeIfWedged = false, want true (kill) for unresponsive running worker")
	}
	if got := w.Status(); got.State != workers.WorkerStateError {
		t.Fatalf("status after wedge kill = %q, want error", got.State)
	}
}

func TestProbeIfWedgedSparesResponsiveRunningWorker(t *testing.T) {
	var buf bytes.Buffer
	w := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}
	killed := make(chan bool, 1)
	go func() { killed <- w.ProbeIfWedged(time.Now()) }()
	waitForPending(t, w, "req-1")
	w.handleRPCLine(`{"type":"response","id":"req-1","success":true,"data":{}}`)
	if <-killed {
		t.Fatal("ProbeIfWedged killed a worker whose RPC loop answered")
	}
	if got := w.Status(); got.State != workers.WorkerStateRunning {
		t.Fatalf("status after healthy probe = %q, want running", got.State)
	}
}

func TestProbeIfWedgedSkipsIdleAndActiveWorkers(t *testing.T) {
	var buf bytes.Buffer
	idle := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending: make(map[string]chan response),
	}
	if idle.ProbeIfWedged(time.Now()) {
		t.Fatal("ProbeIfWedged killed an idle worker")
	}

	active := &piRPCWorker{
		stdin:   nopWriteCloser{&buf},
		status:  workers.WorkerStatus{State: workers.WorkerStateRunning},
		pending: make(map[string]chan response),
	}
	active.noteStreamActivity()
	if active.ProbeIfWedged(time.Now()) {
		t.Fatal("ProbeIfWedged killed a worker with recent stream activity")
	}
	if buf.Len() != 0 {
		t.Fatalf("skipped probes wrote RPC commands: %q", buf.String())
	}
}

// A holder replays its queued-while-detached backlog on reattach. A turn that
// finished while detached must not re-stream its reply into the browser; a
// message still mid-flight gets exactly one catch-up snapshot at replay end.
func TestReplaySuppressesFinishedTurnPreviews(t *testing.T) {
	var sunk []StreamPreview
	w := &piRPCWorker{
		status:        workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending:       make(map[string]chan response),
		streamSink:    func(p StreamPreview) { sunk = append(sunk, p) },
		streamPreview: &streamPreviewAccumulator{},
	}
	w.replaying = true

	// Replayed backlog: a complete turn.
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"old reply"}}`)
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_end","content":"old reply"}}`)
	w.handleRPCLine(`{"type":"agent_end"}`)
	if len(sunk) != 0 {
		t.Fatalf("replayed finished turn was broadcast: %+v", sunk)
	}

	w.handleRPCLine(`{"type":"` + holderReplayEndType + `"}`)
	if len(sunk) != 0 {
		t.Fatalf("replay end emitted a catch-up for a finished turn: %+v", sunk)
	}

	// Live events after the marker stream normally.
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"fresh"}}`)
	if len(sunk) != 1 || sunk[0].Content != "fresh" {
		t.Fatalf("live event after replay not broadcast, got %+v", sunk)
	}
}

func TestReplayEmitsOneCatchUpForMidFlightMessage(t *testing.T) {
	var sunk []StreamPreview
	w := &piRPCWorker{
		status:        workers.WorkerStatus{State: workers.WorkerStateIdle},
		pending:       make(map[string]chan response),
		streamSink:    func(p StreamPreview) { sunk = append(sunk, p) },
		streamPreview: &streamPreviewAccumulator{},
	}
	w.replaying = true

	// Replayed backlog ends mid-message: text is streaming, no text_end yet.
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"hmm "}}`)
	w.handleRPCLine(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"partial ans"}}`)
	if len(sunk) != 0 {
		t.Fatalf("replay was broadcast before the marker: %+v", sunk)
	}

	w.handleRPCLine(`{"type":"` + holderReplayEndType + `"}`)
	if len(sunk) != 1 || sunk[0].Content != "partial ans" || sunk[0].Thinking != "hmm " || sunk[0].Done {
		t.Fatalf("expected one catch-up snapshot, got %+v", sunk)
	}
}
