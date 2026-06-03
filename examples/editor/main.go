// examples/editor validates the App-integration cohort end-to-end (T-6T07): a
// single headless App wires the window-sync, diagnostics, and visual-graph debug
// plugins together, loads a first-party plugin in-process, instantiates and
// hot-reloads a definition, lays out a UI tree, and round-trips graph-debug
// events over the pkg/protocol wire. run() returns a stable hash over the
// deterministic facts (C29-style); main() prints "PASS: editor hash=<N>".
//
// Determinism note: the loop runs on the real wall clock (gametime feeds the
// diagnostics; the live trace recorder stamps time.Now), so run() hashes only
// deterministic facts — registered diagnostic paths, entity counts, plugin
// lifecycle state, UI rects, and graph frames seeded with fixed timestamps. The
// live RunTraced → Recorder path (nondeterministic timestamp) is validated by
// structure in main_test.go, not by hash.
//
// Bootstrap: validates the App integration of internal/{window,diag,definition,
// plugin,grapheditor} + pkg/{visualgraph,editor,protocol} against their L2 specs.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"reflect"
	"sync/atomic"

	"github.com/neuengine/neu/internal/definition"
	"github.com/neuengine/neu/internal/diag"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/internal/grapheditor"
	iplugin "github.com/neuengine/neu/internal/plugin"
	"github.com/neuengine/neu/internal/ui"
	"github.com/neuengine/neu/internal/window"
	"github.com/neuengine/neu/pkg/app"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgdef "github.com/neuengine/neu/pkg/definition"
	pkgdiag "github.com/neuengine/neu/pkg/diag"
	"github.com/neuengine/neu/pkg/editor"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
	"github.com/neuengine/neu/pkg/protocol"
	pkgui "github.com/neuengine/neu/pkg/ui"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

const (
	demoGraphID = "graph.demo"
	exampleName = "editor"
)

// pluginSeq makes each run()'s in-process plugin id unique so concurrent
// invocations never collide on the process-global factory registry.
var pluginSeq atomic.Uint64

// --- demo component types (definition realization targets) -------------------

// Position and Label are the components the demo scene definition spawns.
// encoding/json matches "x"/"y"/"text" to these fields case-insensitively.
type Position struct{ X, Y float64 }

type Label struct{ Text string }

// --- demo in-process plugin --------------------------------------------------

// demoPlugin is a first-party plugin loaded in-process; it records the lifecycle
// phases the loader drives so the example can assert they ran.
type demoPlugin struct{ built, readied, finished, cleaned bool }

func (p *demoPlugin) Build(appface.Builder)   { p.built = true }
func (p *demoPlugin) Ready(appface.Builder)   { p.readied = true }
func (p *demoPlugin) Finish(appface.Builder)  { p.finished = true }
func (p *demoPlugin) Cleanup(appface.Builder) { p.cleaned = true }

// --- definition resolvers ----------------------------------------------------

// nameSet answers the definition's validation-time "does this type exist?".
type nameSet struct{ known map[string]bool }

func (s nameSet) ResolveType(name string) bool { return s.known[name] }

func realizeResolver(m map[string]reflect.Type) definition.TypeResolver {
	return func(name string) (reflect.Type, bool) { t, ok := m[name]; return t, ok }
}

const (
	sceneV1 = `{"definition":"scene","version":"1","content":{"entities":[
	  {"name":"player","components":{"Position":{"x":1,"y":2},"Label":{"text":"hero"}}}]}}`
	sceneV2 = `{"definition":"scene","version":"1","content":{"entities":[
	  {"name":"player","components":{"Position":{"x":9,"y":9}}},
	  {"name":"ally","components":{"Position":{"x":5,"y":5}}}]}}`
)

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %s: %v\n", exampleName, err)
		return
	}
	fmt.Printf("PASS: %s hash=%d\n", exampleName, h)
}

// run performs the deterministic end-to-end validation and returns a stable hash.
func run() (uint64, error) {
	h := fnv.New64a()

	if err := validateCohortApp(h); err != nil {
		return 0, err
	}
	if err := validateDefinitionHotReload(h); err != nil {
		return 0, err
	}
	validateUILayout(h)
	if err := validateGraphDebugRoundTrip(h); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}

// validateCohortApp builds one headless App wiring window-sync + diagnostics +
// the graph-debug sync plugin, loads a plugin in-process, runs a single frame,
// and folds the lifecycle facts into h.
func validateCohortApp(h hash.Hash64) error {
	diagStore := pkgdiag.NewDiagnosticsStore()

	a := app.NewApp()
	a.AddPlugins(app.DefaultPlugins{})
	a.AddPlugin(window.SyncPlugin{Config: pkgwindow.WindowPlugin{
		PrimaryWindow: &pkgwindow.Window{Title: "Editor", Visible: true},
		ExitCondition: pkgwindow.DontExit,
	}})
	a.AddPlugin(diag.DiagnosticsPlugin{Store: diagStore})
	a.AddPlugin(grapheditor.SyncPlugin{Store: grapheditor.NewDebugStore()})

	// Register a reader so the diagnostics collector does real work (INV-1).
	diagStore.AddReader(diag.PathFPS)

	// Load a first-party plugin in-process via the compile-time factory registry.
	id := pkgplugin.PluginID(fmt.Sprintf("com.neuengine.example.editor.demo.%d", pluginSeq.Add(1)))
	plug := &demoPlugin{}
	iplugin.RegisterFactory(id, func() appface.Plugin { return plug })

	mgr := iplugin.NewManager(mustVersion("1.0.0"))
	man := pkgplugin.Manifest{
		ID:            id,
		Version:       "1.0.0",
		EngineVersion: "^1.0.0",
		Mode:          pkgplugin.ModeInProcess,
		Entry:         pkgplugin.EntrySpec{PackagePath: "com.neuengine.example.editor", Factory: "New"},
	}
	if err := mgr.Register(man, nil); err != nil {
		return fmt.Errorf("plugin register: %w", err)
	}
	if _, err := iplugin.LoadInProcess(mgr, id, a); err != nil {
		return fmt.Errorf("plugin load: %w", err)
	}

	a.SetRunMode(app.RunOnce)
	if err := a.Run(); err != nil {
		return fmt.Errorf("app run: %w", err)
	}

	// Drive the in-process shutdown phases to exercise the full lifecycle.
	iplugin.FinishInProcess(mgr, id, plug, a)

	if !plug.built || !plug.readied || !plug.finished || !plug.cleaned {
		return fmt.Errorf("plugin lifecycle incomplete: %+v", *plug)
	}
	if st, _ := mgr.State(id); st != pkgplugin.StateDisabled {
		return fmt.Errorf("plugin state = %v, want Disabled after finish", st)
	}
	if !diagStore.HasAnyReader() {
		return fmt.Errorf("diagnostics reader was not registered")
	}

	// Hash deterministic facts (NOT wall-clock metric values).
	fmt.Fprintf(h, "app:ran|diag:%s,%s,%s|plugin:lifecycle-ok",
		diag.PathFPS, diag.PathFrameTimeMS, diag.PathEntityCount)
	return nil
}

// validateDefinitionHotReload decodes a scene definition, instantiates it into a
// World, then hot-reloads it with a new version via the InstanceStore (the core
// operation the hot-reload system performs on an asset Modified event).
func validateDefinitionHotReload(h hash.Hash64) error {
	w := world.NewWorld()
	resolve := realizeResolver(map[string]reflect.Type{
		"Position": reflect.TypeFor[Position](),
		"Label":    reflect.TypeFor[Label](),
	})
	valid := nameSet{known: map[string]bool{"Position": true, "Label": true}}
	store := definition.NewInstanceStore()

	d1, err := pkgdef.Decode([]byte(sceneV1), valid, pkgdef.NewActionRegistry())
	if err != nil {
		return fmt.Errorf("decode v1: %w", err)
	}
	in1, errs := store.Instantiate(w, "scenes/demo", &d1, resolve)
	if len(errs) != 0 {
		return fmt.Errorf("instantiate v1: %v", errs)
	}
	pos, ok := world.Get[Position](w, in1.Entities[0])
	if !ok || *pos != (Position{X: 1, Y: 2}) {
		return fmt.Errorf("v1 player Position = %v (ok=%v), want {1 2}", pos, ok)
	}

	// Hot-reload: re-decode the modified definition and replace the instance.
	d2, err := pkgdef.Decode([]byte(sceneV2), valid, pkgdef.NewActionRegistry())
	if err != nil {
		return fmt.Errorf("decode v2: %w", err)
	}
	in2, errs := store.Reload(w, "scenes/demo", &d2, resolve)
	if len(errs) != 0 {
		return fmt.Errorf("reload v2: %v", errs)
	}
	if len(in2.Entities) != 2 {
		return fmt.Errorf("v2 spawned %d entities, want 2", len(in2.Entities))
	}
	// The old instance's entities must be gone (despawned by Reload).
	if _, alive := world.Get[Position](w, in1.Entities[0]); alive {
		return fmt.Errorf("hot-reload left a stale entity alive")
	}
	pos2, ok := world.Get[Position](w, in2.Entities[0])
	if !ok || *pos2 != (Position{X: 9, Y: 9}) {
		return fmt.Errorf("v2 player Position = %v (ok=%v), want {9 9}", pos2, ok)
	}

	fmt.Fprintf(h, "|def:v1=%d->v2=%d:%v", len(in1.Entities), len(in2.Entities), *pos2)
	return nil
}

// validateUILayout solves a small UI tree against a fixed viewport and folds the
// resulting rects (deterministic) into h — the "ui" leg of the cohort.
func validateUILayout(h hash.Hash64) {
	root := &ui.LayoutNode{
		Style: pkgui.Style{FlexDirection: pkgui.Row, Width: pkgui.Px(200), Height: pkgui.Px(100)},
		Children: []*ui.LayoutNode{
			{Style: pkgui.Style{FlexGrow: 1}},
			{Style: pkgui.Style{FlexGrow: 1}},
		},
	}
	ui.Solve(root, ui.Viewport{Width: 200, Height: 100})

	fmt.Fprintf(h, "|ui:root=%v", root.Rect)
	for i, c := range root.Children {
		fmt.Fprintf(h, ",c%d=%v", i, c.Rect)
	}
}

// validateGraphDebugRoundTrip seeds a deterministic debug trace, drives the
// PostUpdate DebugSync system through one App frame so it emits pkg/protocol
// graph IPC messages, then asserts each message survives an Encode → Decode wire
// round-trip and folds the wire bytes into h.
func validateGraphDebugRoundTrip(h hash.Hash64) error {
	store := grapheditor.NewDebugStore()
	store.Open(demoGraphID)
	store.SetBreakpoint(demoGraphID, "n2")
	// Fixed timestamps keep the wire bytes (and thus the hash) deterministic.
	store.RecordFrame(demoGraphID, 7, editor.GraphExecutionFrame{
		NodeID: "n1", NodeType: "event.OnUpdate", StepIndex: 1, Timestamp: 1000,
		PinValues: map[string]any{"v": float64(1)},
	})
	store.RecordFrame(demoGraphID, 7, editor.GraphExecutionFrame{
		NodeID: "n2", NodeType: "action.Log", StepIndex: 2, Timestamp: 2000,
		PinValues: map[string]any{"v": float64(2)},
	})
	store.RecordError(demoGraphID, 7, "n3", "boom")

	var captured []any
	a := app.NewApp()
	a.AddPlugins(app.DefaultPlugins{})
	a.AddPlugin(grapheditor.SyncPlugin{Store: store, Sink: func(msg any) { captured = append(captured, msg) }})
	a.SetRunMode(app.RunOnce)
	if err := a.Run(); err != nil {
		return fmt.Errorf("graph sync app run: %w", err)
	}

	if len(captured) != 3 {
		return fmt.Errorf("captured %d graph messages, want 3", len(captured))
	}
	wantKinds := []protocol.Kind{
		protocol.KindGraphExecutionTrace,
		protocol.KindGraphBreakpointHit,
		protocol.KindGraphRuntimeError,
	}
	for i, msg := range captured {
		var buf bytes.Buffer
		if err := protocol.Encode(&buf, msg); err != nil {
			return fmt.Errorf("encode msg %d: %w", i, err)
		}
		decoded, kind, err := protocol.Decode(bufio.NewReader(bytes.NewReader(buf.Bytes())))
		if err != nil {
			return fmt.Errorf("decode msg %d: %w", i, err)
		}
		if kind != wantKinds[i] {
			return fmt.Errorf("msg %d kind = %q, want %q", i, kind, wantKinds[i])
		}
		if !reflect.DeepEqual(msg, decoded) {
			return fmt.Errorf("msg %d did not survive the wire round-trip:\n in=%#v\nout=%#v", i, msg, decoded)
		}
		h.Write(buf.Bytes())
	}
	return nil
}

// mustVersion parses a SemVer string, panicking on a malformed literal (the
// example controls its own inputs).
func mustVersion(s string) pkgplugin.Version {
	v, err := pkgplugin.ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}
