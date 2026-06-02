package diag

import (
	"context"
	"log/slog"
	"sync"
)

// LogLevel aliases slog.Level so the engine's level vocabulary lines up with
// the standard library. LevelTrace extends below slog.Debug (L1 §4.11).
type LogLevel = slog.Level

// LevelTrace is finer-grained than Debug for execution-flow tracing.
const LevelTrace LogLevel = slog.LevelDebug - 4

// levelConfig hol-ds the per-module thresholds shared across a handler and all
// of its WithAttrs/WithGroup derivations (single mutex — clones never copy it).
type levelConfig struct {
	levels map[string]LogLevel
	def    LogLevel
	mu     sync.RWMutex
}

func (c *levelConfig) levelFor(module string) LogLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if lvl, ok := c.levels[module]; ok {
		return lvl
	}
	return c.def
}

func (c *levelConfig) minLevel() LogLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	min := c.def
	for _, l := range c.levels {
		if l < min {
			min = l
		}
	}
	return min
}

// ModuleFilterHandler wraps a base slog.Handler and drops records whose level
// is below the threshold configured for their "module" attribute. A record
// without a module attribute uses the default level. This gives per-subsystem
// verbosity (e.g. render at Debug, everything else at Info) over stdlib slog.
type ModuleFilterHandler struct {
	base slog.Handler
	cfg  *levelConfig // shared with derived handlers
}

// NewModuleFilterHandler wraps base, applying def as the fallback level.
func NewModuleFilterHandler(base slog.Handler, def LogLevel) *ModuleFilterHandler {
	return &ModuleFilterHandler{
		base: base,
		cfg:  &levelConfig{levels: make(map[string]LogLevel), def: def},
	}
}

// SetModuleLevel sets the minimum level for records tagged module=name. It
// affects this handler and every handler derived from it.
func (h *ModuleFilterHandler) SetModuleLevel(name string, lvl LogLevel) {
	h.cfg.mu.Lock()
	defer h.cfg.mu.Unlock()
	h.cfg.levels[name] = lvl
}

// Enabled gates coarsely: a record can only be dropped precisely in Handle once
// its module attribute is known, so here we admit anything at or above the most
// permissive configured level.
func (h *ModuleFilterHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.cfg.minLevel()
}

// Handle applies the per-module threshold, then forwards to the base handler.
func (h *ModuleFilterHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < h.cfg.levelFor(moduleOf(r)) {
		return nil
	}
	return h.base.Handle(ctx, r)
}

// WithAttrs delegates to the base handler, sharing the level config.
func (h *ModuleFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ModuleFilterHandler{base: h.base.WithAttrs(attrs), cfg: h.cfg}
}

// WithGroup delegates to the base handler, sharing the level config.
func (h *ModuleFilterHandler) WithGroup(name string) slog.Handler {
	return &ModuleFilterHandler{base: h.base.WithGroup(name), cfg: h.cfg}
}

// moduleOf extracts the "module" string attribute from a record ("" if absent).
func moduleOf(r slog.Record) string {
	var module string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "module" {
			module = a.Value.String()
			return false
		}
		return true
	})
	return module
}

var _ slog.Handler = (*ModuleFilterHandler)(nil)
