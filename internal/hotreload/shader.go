//go:build editor

package hotreload

// ShaderHandle identifies a compiled shader program in the render backend.
type ShaderHandle uint64

// ShaderCompiler is the render-backend seam the shader hot-swap drives. It is an
// interface so hot-reload stays decoupled from render internals; the app injects
// the real backend, the headless/software path a validating no-op, and tests a
// fake. RebindMaterials/ReleaseShader exist so a successful swap can repoint
// materials and free the old program out-of-band (double-buffered).
type ShaderCompiler interface {
	// CompileShader compiles source into a new program handle, or returns an
	// error (kept non-fatal by the reloader — the old program stays bound).
	CompileShader(path string, source []byte) (ShaderHandle, error)
	// RebindMaterials repoints every material from the old handle to the new.
	RebindMaterials(oldHandle, newHandle ShaderHandle)
	// ReleaseShader frees a program's GPU resources.
	ReleaseShader(handle ShaderHandle)
}

// EventSink receives shader reload notifications, which the app maps onto
// pkg/protocol ShaderError / ShaderReloaded messages for the editor overlay.
// Decoupled (injected) so hot-reload imports no transport types — the same
// seam pattern the visual-graph debug bridge uses.
type EventSink interface {
	ShaderError(path, message string)
	ShaderReloaded(path string)
}

// ShaderReloader performs in-process shader hot-swap (l1-hot-reload §4.5,
// INV-3). It compiles changed source into a NEW handle first; on a compile
// error the previously bound handle is kept (no visual glitch) and the error is
// surfaced. On success the swap is double-buffered: the new handle is bound and
// materials rebound immediately, while the old handle's GPU resources are
// released only after the current frame completes (via ReleasePending), avoiding
// a mid-frame GPU state change.
//
// Not safe for concurrent use; driven from the main thread.
type ShaderReloader struct {
	compiler ShaderCompiler
	sink     EventSink
	active   map[string]ShaderHandle
	pending  []ShaderHandle
}

// NewShaderReloader returns a reloader over the given compiler and event sink.
// A nil sink is tolerated (notifications are dropped).
func NewShaderReloader(compiler ShaderCompiler, sink EventSink) *ShaderReloader {
	return &ShaderReloader{
		compiler: compiler,
		sink:     sink,
		active:   make(map[string]ShaderHandle),
	}
}

// Swap recompiles the shader at path from source and hot-swaps it (INV-3). On a
// compile error the current handle stays bound and a ShaderError is emitted; on
// success materials are rebound to the new handle, the old handle is queued for
// post-frame release, and a ShaderReloaded is emitted.
func (r *ShaderReloader) Swap(path string, source []byte) {
	newHandle, err := r.compiler.CompileShader(path, source)
	if err != nil {
		// INV-3: keep the previously compiled shader active — no visual glitch.
		if r.sink != nil {
			r.sink.ShaderError(path, err.Error())
		}
		return
	}
	if old, ok := r.active[path]; ok {
		r.compiler.RebindMaterials(old, newHandle)
		r.pending = append(r.pending, old) // double-buffered: release after frame
	}
	r.active[path] = newHandle
	if r.sink != nil {
		r.sink.ShaderReloaded(path)
	}
}

// ReleasePending frees the handles deferred by the previous frame's swaps. It
// must be called once per frame after the frame's command buffer has completed,
// so the old program is never freed mid-frame.
func (r *ShaderReloader) ReleasePending() {
	for _, h := range r.pending {
		r.compiler.ReleaseShader(h)
	}
	r.pending = r.pending[:0]
}

// Active returns the current handle bound for path and whether one exists.
func (r *ShaderReloader) Active(path string) (ShaderHandle, bool) {
	h, ok := r.active[path]
	return h, ok
}
