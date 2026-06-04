//go:build profiling

package profiling

import (
	"encoding/json"
	"io"
	"strconv"
)

// tefEvent is one entry in the Chrome Trace Event Format. "X" = complete
// duration event; "i" = instant event (used for frame marks). Timestamps and
// durations are microseconds.
type tefEvent struct {
	Args  map[string]any `json:"args,omitempty"`
	Name  string         `json:"name"`
	Cat   string         `json:"cat,omitempty"`
	Ph    string         `json:"ph"`
	Scope string         `json:"s,omitempty"`
	Ts    int64          `json:"ts"`
	Dur   int64          `json:"dur,omitempty"`
	Pid   int            `json:"pid"`
	Tid   uint64         `json:"tid"`
}

// ChromeExporter buffers spans and frame marks and writes them as a Trace Event
// Format JSON array, viewable in chrome://tracing or Perfetto. It uses only
// encoding/json (no third-party dependency, C-003). The buffer is written on
// Flush/Shutdown; for long sessions a caller may Flush periodically.
type ChromeExporter struct {
	w      io.Writer
	events []tefEvent
}

// NewChromeExporter returns an exporter writing the TEF array to w. The plugin
// supplies an *os.File at ChromeOutputPath; tests may pass a bytes.Buffer.
func NewChromeExporter(w io.Writer) *ChromeExporter {
	return &ChromeExporter{w: w}
}

// Init is a no-op; the buffer is written on Flush.
func (c *ChromeExporter) Init() error { return nil }

// EmitSpan buffers a completed span as a TEF duration event. Span metadata is
// copied into the event's args (the pooled span is recycled after this call).
func (c *ChromeExporter) EmitSpan(s Span) {
	ev := tefEvent{
		Name: s.name,
		Cat:  s.category.String(),
		Ph:   "X",
		Ts:   s.startNs / 1_000,
		Dur:  (s.endNs - s.startNs) / 1_000,
		Pid:  0,
		Tid:  s.threadID,
	}
	if len(s.metadata) > 0 {
		args := make(map[string]any, len(s.metadata))
		for _, kv := range s.metadata {
			args[kv.Key] = kv.Value
		}
		ev.Args = args
	}
	c.events = append(c.events, ev)
}

// EmitFrameMark buffers a frame boundary as a global instant event.
func (c *ChromeExporter) EmitFrameMark(frame uint64) {
	c.events = append(c.events, tefEvent{
		Name:  "Frame " + strconv.FormatUint(frame, 10),
		Ph:    "i",
		Scope: "g",
		Ts:    nowNanos() / 1_000,
		Pid:   0,
	})
}

// Flush marshals the buffered events to the writer as a TEF JSON array.
func (c *ChromeExporter) Flush() error {
	data, err := json.Marshal(c.events)
	if err != nil {
		return err
	}
	_, err = c.w.Write(data)
	return err
}

// Shutdown flushes the buffer.
func (c *ChromeExporter) Shutdown() error { return c.Flush() }
