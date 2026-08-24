package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSONL(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadLastTurnState(t *testing.T) {
	header := `{"type":"session","id":"s","cwd":"/x"}`
	cases := []struct {
		name     string
		lines    []string
		wantRole string
		wantStop string
		working  bool
	}{
		{
			name:     "bare user prompt is not (yet) assistant work",
			lines:    []string{header, `{"type":"message","message":{"role":"user","content":"hi"}}`},
			wantRole: "user",
			working:  false,
		},
		{
			name:     "assistant tool use is working",
			lines:    []string{header, `{"type":"message","message":{"role":"assistant","stopReason":"toolUse"}}`},
			wantRole: "assistant",
			wantStop: "toolUse",
			working:  true,
		},
		{
			name:     "tool result awaiting continuation is working",
			lines:    []string{header, `{"type":"message","message":{"role":"assistant","stopReason":"toolUse"}}`, `{"type":"message","message":{"role":"toolResult"}}`},
			wantRole: "toolResult",
			working:  true,
		},
		{
			name:     "assistant stop is settled",
			lines:    []string{header, `{"type":"message","message":{"role":"assistant","stopReason":"stop"}}`},
			wantRole: "assistant",
			wantStop: "stop",
			working:  false,
		},
		{
			name:     "metadata tail ignores trailing rename",
			lines:    []string{header, `{"type":"message","message":{"role":"assistant","stopReason":"toolUse"}}`, `{"type":"rename","name":"x"}`},
			wantRole: "assistant",
			wantStop: "toolUse",
			working:  true,
		},
		{
			name:    "header only is not a turn",
			lines:   []string{header},
			working: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeJSONL(t, tc.lines...)
			st, err := ReadLastTurnState(path)
			if err != nil {
				t.Fatal(err)
			}
			if st.Role != tc.wantRole || st.StopReason != tc.wantStop {
				t.Fatalf("role=%q stop=%q, want role=%q stop=%q", st.Role, st.StopReason, tc.wantRole, tc.wantStop)
			}
			if st.AssistantWorking() != tc.working {
				t.Fatalf("AssistantWorking=%v, want %v", st.AssistantWorking(), tc.working)
			}
		})
	}
}

func TestReadLastTurnStateMissingFile(t *testing.T) {
	if _, err := ReadLastTurnState(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
