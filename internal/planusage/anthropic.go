package planusage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Anthropic reads Claude subscription (Pro/Max) usage windows via the OAuth
// usage endpoint. Provider id matches pi's "anthropic" credential entry.
type Anthropic struct {
	Client   *http.Client
	Endpoint string
}

func NewAnthropic() *Anthropic {
	return &Anthropic{
		Client:   &http.Client{Timeout: 10 * time.Second},
		Endpoint: "https://api.anthropic.com/api/oauth/usage",
	}
}

func (a *Anthropic) ID() string    { return "anthropic" }
func (a *Anthropic) Label() string { return "Anthropic" }

type anthropicWindow struct {
	Utilization float64    `json:"utilization"`
	ResetsAt    *time.Time `json:"resets_at"`
}

// anthropicWindows lists the windows we surface, in display order. Unknown or
// null windows are skipped so plan differences (e.g. no Opus bucket) don't
// break rendering.
var anthropicWindows = []struct{ key, label string }{
	{"five_hour", "5h"},
	{"seven_day", "7d"},
	{"seven_day_opus", "7d Opus"},
	{"seven_day_sonnet", "7d Sonnet"},
}

func (a *Anthropic) Fetch(ctx context.Context, token string) (Usage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Endpoint, nil)
	if err != nil {
		return Usage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pi-web")
	resp, err := a.Client.Do(req)
	if err != nil {
		return Usage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Usage{}, statusError(resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Usage{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Usage{}, err
	}
	usage := Usage{Windows: []Window{}}
	for _, w := range anthropicWindows {
		entry, ok := raw[w.key]
		if !ok || string(entry) == "null" {
			continue
		}
		var win anthropicWindow
		if err := json.Unmarshal(entry, &win); err != nil {
			continue
		}
		usage.Windows = append(usage.Windows, Window{
			Key:         w.key,
			Label:       w.label,
			Utilization: win.Utilization,
			ResetsAt:    win.ResetsAt,
		})
	}
	return usage, nil
}
