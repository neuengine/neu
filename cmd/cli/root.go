// Command neuengine is the engine CLI: a stdlib-flag command router for
// scaffolding, diagnostics, and plugin management. Command groups are gated on
// their backing subsystem existing (INV-3); no-arg invocation prints a
// structured help menu (INV-2); machine output is opt-in via --json (INV-4);
// scaffolding never silently overwrites (INV-1).
//
// Bootstrap: l2-cli-tooling-go Draft (Phase 6 Track F, C29 gate open).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Command is one CLI verb. Run receives the args after the command name and an
// Output sink that renders human text or stable JSON per the --json flag.
type Command interface {
	Name() string
	Help() string
	Run(ctx context.Context, args []string, out *Output) error
}

// Output renders command results as human text or stable JSON (INV-4).
type Output struct {
	JSON  bool
	w     io.Writer
	errW  io.Writer
	Force bool // --force: allow overwrite (INV-1)
	Dry   bool // --dry-run: preview, write nothing
}

// NewOutput builds an Output writing to w (stdout) and errW (stderr).
func NewOutput(w, errW io.Writer) *Output { return &Output{w: w, errW: errW} }

// Linef writes a human-readable line; suppressed in JSON mode.
func (o *Output) Linef(format string, args ...any) {
	if !o.JSON {
		_, _ = fmt.Fprintf(o.w, format+"\n", args...)
	}
}

// WriteJSON emits v as stable JSON; only in JSON mode.
func (o *Output) WriteJSON(v any) error {
	if !o.JSON {
		return nil
	}
	enc := json.NewEncoder(o.w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Errorf writes an actionable error line to stderr (JSON-wrapped in JSON mode).
func (o *Output) Errorf(format string, args ...any) {
	if o.JSON {
		_ = json.NewEncoder(o.errW).Encode(map[string]string{"error": fmt.Sprintf(format, args...)})
		return
	}
	_, _ = fmt.Fprintf(o.errW, "error: "+format+"\n", args...)
}

// Router dispatches the first argument to a registered Command (INV-3: only
// implemented commands are registered, so help never advertises a phantom).
type Router struct {
	commands map[string]Command
	order    []string
}

// NewRouter returns an empty router.
func NewRouter() *Router { return &Router{commands: make(map[string]Command)} }

// Register adds a command (in registration order for help listing).
func (r *Router) Register(c Command) {
	if _, dup := r.commands[c.Name()]; !dup {
		r.order = append(r.order, c.Name())
	}
	r.commands[c.Name()] = c
}

// Run dispatches argv (excluding the program name). It parses global flags,
// prints help for no-arg / "help" (INV-2), and returns a process exit code.
func (r *Router) Run(argv []string, stdout, stderr io.Writer) int {
	out := NewOutput(stdout, stderr)
	var positional []string
	for _, a := range argv {
		switch a {
		case "--json":
			out.JSON = true
		case "--force":
			out.Force = true
		case "--dry-run":
			out.Dry = true
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) == 0 || positional[0] == "help" {
		r.printHelp(out)
		return 0
	}
	name := positional[0]
	cmd, ok := r.commands[name]
	if !ok {
		out.Errorf("unknown command %q (run `neuengine help`)", name)
		return 2
	}
	if err := cmd.Run(context.Background(), positional[1:], out); err != nil {
		out.Errorf("%v", err)
		return 1
	}
	return 0
}

// printHelp lists the registered commands (INV-2). Only registered (existing)
// commands appear (INV-3).
func (r *Router) printHelp(out *Output) {
	names := append([]string(nil), r.order...)
	sort.Strings(names)
	if out.JSON {
		type cmdInfo struct{ Name, Help string }
		infos := make([]cmdInfo, 0, len(names))
		for _, n := range names {
			infos = append(infos, cmdInfo{n, r.commands[n].Help()})
		}
		_ = out.WriteJSON(map[string]any{"commands": infos})
		return
	}
	out.Linef("neuengine — engine CLI")
	out.Linef("")
	out.Linef("Commands:")
	for _, n := range names {
		out.Linef("  %-12s %s", n, r.commands[n].Help())
	}
}

// Commands returns the registered command names (test hook).
func (r *Router) Commands() []string { return append([]string(nil), r.order...) }
