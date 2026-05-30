package diag

import (
	"testing"

	pkgdiag "github.com/neuengine/neu/pkg/diag"
	pkgmath "github.com/neuengine/neu/pkg/math"
)

func TestGizmoFeatureDrainsAndResets(t *testing.T) {
	t.Parallel()
	buf := pkgdiag.NewGizmoBuffer()
	white := pkgmath.LinearRgba{A: 1}
	buf.Line(pkgmath.Vec3{}, pkgmath.Vec3{X: 1}, white) // 2 verts
	buf.Text(pkgmath.Vec3{}, "hi", white)               // 1 text

	f := NewGizmoFeature(buf)

	// Headless Draw ignores its contexts (no GPU backend) — nil is safe.
	f.Draw(nil, nil)
	if f.LastLineVertexCount() != 2 {
		t.Errorf("LastLineVertexCount = %d, want 2", f.LastLineVertexCount())
	}
	if f.LastTextCount() != 1 {
		t.Errorf("LastTextCount = %d, want 1", f.LastTextCount())
	}

	// Flush clears the buffer (gizmos are one-frame, INV-2).
	f.Flush(nil)
	if len(buf.Lines()) != 0 || len(buf.Texts()) != 0 {
		t.Error("Flush should reset the gizmo buffer")
	}

	// No-op lifecycle hooks must not panic with nil contexts.
	f.Initialize(nil)
	f.Collect(nil)
	f.Extract(nil)
	f.PrepareEffectPermutations(nil)
	f.Prepare(nil)
}
