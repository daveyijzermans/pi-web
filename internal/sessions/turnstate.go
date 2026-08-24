package sessions

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// turnStateTailBytes bounds how much of a session file's tail we read to find
// the last message entry. A single JSONL entry (even a large assistant turn
// with tool output) fits comfortably; reading only the tail keeps terminal-turn
// detection cheap on the status hot path regardless of total session size.
const turnStateTailBytes = 64 * 1024

// TurnState summarises the last message entry in a session JSONL, which is
// enough to tell whether a turn is mid-flight (a terminal `pi` process is
// actively working) or settled.
type TurnState struct {
	// HasMessage is false when the tail held no message entry (empty/new
	// session, or a metadata-only tail).
	HasMessage bool
	// Role of the last message entry: "user", "assistant", "tool", etc.
	Role string
	// StopReason of the last assistant entry (e.g. "toolUse", "stop"). Empty
	// for non-assistant roles or when the format omits it.
	StopReason string
}

// AssistantWorking reports whether the assistant is actively mid-response —
// it paused to call a tool (more turns to come) or a tool result is awaiting
// its continuation. This is the signal for "a pi process is working on this
// session right now".
//
// It deliberately excludes a bare trailing user message: that is ambiguous
// (a fresh or abandoned prompt whose reply hasn't started) and treating it as
// running risks false "done" notifications on new-session cold starts. A
// completed assistant message (stopReason "stop"/"end"/etc.) is settled — pi
// flushes whole entries, so a trailing assistant entry without a tool-use
// stop reason is final.
func (t TurnState) AssistantWorking() bool {
	if !t.HasMessage {
		return false
	}
	switch strings.ToLower(t.Role) {
	case "tool", "toolresult", "tool_result":
		return true
	case "assistant":
		switch t.StopReason {
		case "toolUse", "tool_use", "tooluse":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// ReadLastTurnState reads the tail of a session JSONL and returns the state of
// the last message entry. It never scans the whole file: it reads at most the
// final turnStateTailBytes and walks lines backwards to the last message.
func ReadLastTurnState(path string) (TurnState, error) {
	f, err := os.Open(path)
	if err != nil {
		return TurnState{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return TurnState{}, err
	}
	size := info.Size()
	start := int64(0)
	if size > turnStateTailBytes {
		start = size - turnStateTailBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return TurnState{}, err
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return TurnState{}, err
	}

	lines := strings.Split(string(buf), "\n")
	// If we started mid-file the first line is likely a partial entry; drop it.
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	var raw struct {
		Type    string `json:"type"`
		Message *struct {
			Role       string `json:"role"`
			StopReason string `json:"stopReason"`
		} `json:"message"`
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		raw.Type = ""
		raw.Message = nil
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if raw.Type != "message" || raw.Message == nil || raw.Message.Role == "" {
			continue
		}
		return TurnState{
			HasMessage: true,
			Role:       raw.Message.Role,
			StopReason: raw.Message.StopReason,
		}, nil
	}
	return TurnState{}, nil
}
