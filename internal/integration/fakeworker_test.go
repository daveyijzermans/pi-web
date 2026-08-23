package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"pi-web/internal/chat"
	"pi-web/internal/rpc"
	"pi-web/internal/workers"
)

// fakeWorker implements workers.ChatWorker, idleReportable, and inspector.
// It records all calls and can emit stream previews via its sink,
// simulate crashes, and control idle duration for reaping tests.
type fakeWorker struct {
	mu    sync.Mutex
	sink  rpc.StreamEventSink
	state workers.State
	err   string
	model string

	prompts          []chat.Request
	setModelCalls    []modelCall
	setThinkingCalls []string
	compactCalls     int
	abortCalls       int
	closeCalls       int

	// crashMode: when true, Prompt returns an error and Status reports error.
	crashMode bool

	// idleFor controls IdleSince return value (set by tests).
	idleFor time.Duration
}

type modelCall struct {
	provider string
	modelID  string
}

func newFakeWorker(sink rpc.StreamEventSink) *fakeWorker {
	return &fakeWorker{
		sink:  sink,
		state: workers.WorkerStateIdle,
	}
}

// emit sends a StreamPreview through the sink (drives the SSE broadcast path).
func (f *fakeWorker) emit(preview rpc.StreamPreview) {
	if f.sink != nil {
		f.sink(preview)
	}
}

// crash sets the worker into error state so the Manager evicts it.
func (f *fakeWorker) crash() {
	f.mu.Lock()
	f.crashMode = true
	f.state = workers.WorkerStateError
	f.err = "simulated crash"
	f.mu.Unlock()
}

func (f *fakeWorker) Prompt(ctx context.Context, r chat.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.crashMode {
		return fmt.Errorf("worker crashed: %s", f.err)
	}
	f.prompts = append(f.prompts, r)
	f.state = workers.WorkerStateRunning
	return nil
}

func (f *fakeWorker) SetModel(ctx context.Context, provider, modelID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setModelCalls = append(f.setModelCalls, modelCall{provider, modelID})
	f.model = modelID
	return nil
}

func (f *fakeWorker) SetThinkingLevel(ctx context.Context, level string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setThinkingCalls = append(f.setThinkingCalls, level)
	return nil
}

func (f *fakeWorker) Compact(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compactCalls++
	return nil
}

func (f *fakeWorker) Abort(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.abortCalls++
	return nil
}

func (f *fakeWorker) GetState(ctx context.Context) (workers.WorkerStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return workers.WorkerStatus{
		State: f.state,
		Error: f.err,
		Model: f.model,
	}, nil
}

func (f *fakeWorker) GetCommands(ctx context.Context) ([]workers.SlashCommand, error) {
	return nil, nil
}

func (f *fakeWorker) Status() workers.WorkerStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return workers.WorkerStatus{
		State: f.state,
		Error: f.err,
		Model: f.model,
	}
}

func (f *fakeWorker) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	return nil
}

// idleReportable (matches workers.idleReportable)
func (f *fakeWorker) IdleSince(now time.Time) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.idleFor
}

// inspector (matches workers.inspector)
func (f *fakeWorker) PID() int             { return 0 }
func (f *fakeWorker) StartedAt() time.Time { return time.Time{} }
