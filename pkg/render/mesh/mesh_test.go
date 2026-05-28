package mesh

import (
	"errors"
	"testing"
)

// helper: a valid 3-vertex position attribute (3×float32x3 = 36 bytes).
func posAttr(n int) VertexAttribute {
	return VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: make([]byte, n*12)}
}

func normalAttr(n int) VertexAttribute {
	return VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: make([]byte, n*12)}
}

// uint16 index buffer for n vertices, referencing 0 through n-1.
func sequentialIdx16(n int) IndexBuffer {
	data := make([]byte, n*2)
	for i := range n {
		data[i*2] = byte(i)
		data[i*2+1] = byte(i >> 8)
	}
	return IndexBuffer{Wide: false, Data: data}
}

// uint32 index buffer with one out-of-range entry.
func outOfRangeIdx32(vertexCount int) IndexBuffer {
	bad := uint32(vertexCount) // == vertexCount → out of range
	data := []byte{byte(bad), byte(bad >> 8), byte(bad >> 16), byte(bad >> 24)}
	return IndexBuffer{Wide: true, Data: data}
}

func TestMeshValidate(t *testing.T) {
	t.Parallel()
	const n = 3

	tests := []struct {
		name    string
		build   func() *Mesh
		wantErr error
	}{
		{
			name: "valid non-indexed",
			build: func() *Mesh {
				return NewMesh(TopologyTriangleList).SetAttribute(posAttr(n))
			},
		},
		{
			name: "valid indexed u16",
			build: func() *Mesh {
				m := NewMesh(TopologyTriangleList).SetAttribute(posAttr(n))
				m.SetIndices(sequentialIdx16(n))
				return m
			},
		},
		{
			name: "INV-1: missing position",
			build: func() *Mesh {
				return NewMesh(TopologyTriangleList).SetAttribute(normalAttr(n))
			},
			wantErr: ErrMeshNoPosition,
		},
		{
			name: "INV-2: index out of range (u32)",
			build: func() *Mesh {
				m := NewMesh(TopologyTriangleList).SetAttribute(posAttr(n))
				m.SetIndices(outOfRangeIdx32(n))
				return m
			},
			wantErr: ErrIndexOutOfRange,
		},
		{
			name: "INV-2: index out of range (u16)",
			build: func() *Mesh {
				m := NewMesh(TopologyTriangleList).SetAttribute(posAttr(1))
				// index 1 is out-of-range for a 1-vertex mesh
				m.SetIndices(IndexBuffer{Wide: false, Data: []byte{1, 0}})
				return m
			},
			wantErr: ErrIndexOutOfRange,
		},
		{
			name: "INV-3: joint weight/index length mismatch",
			build: func() *Mesh {
				// 3 vertices, joint weights for 3, joint indices for 2 → mismatch
				jw := VertexAttribute{Kind: AttrJointWeights, Format: FormatFloat32x4, Data: make([]byte, 3*16)}
				ji := VertexAttribute{Kind: AttrJointIndices, Format: FormatUint16x4, Data: make([]byte, 2*8)}
				return NewMesh(TopologyTriangleList).
					SetAttribute(posAttr(n)).
					SetAttribute(jw).
					SetAttribute(ji)
			},
			wantErr: ErrSkinAttributeMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.build().Validate()
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestLayoutHash(t *testing.T) {
	t.Parallel()

	build := func() *Mesh {
		return NewMesh(TopologyTriangleList).
			SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: make([]byte, 12)}).
			SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: make([]byte, 12)}).
			SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: make([]byte, 8)})
	}

	// Same attribute set → identical hash across multiple calls and builds.
	m1 := build()
	m2 := build()
	h1 := m1.Layout().Hash
	h2 := m2.Layout().Hash
	if h1 == 0 {
		t.Fatal("hash must be non-zero for a valid layout")
	}
	if h1 != h2 {
		t.Fatalf("layout hash is non-deterministic: %d != %d", h1, h2)
	}

	// Memoisation: second call returns same value without recomputing.
	if m1.Layout().Hash != h1 {
		t.Fatal("Layout() is not memoised")
	}

	// Different attribute set → different hash.
	mDiff := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: make([]byte, 12)})
	if mDiff.Layout().Hash == h1 {
		t.Fatal("different attribute sets produced identical hash")
	}
}

func TestLayoutHashStable(t *testing.T) {
	t.Parallel()
	// Run 20 times to confirm hash stability (map iteration is randomised in Go).
	const rounds = 20
	build := func() *Mesh {
		return NewMesh(TopologyTriangleList).
			SetAttribute(posAttr(1)).
			SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: make([]byte, 8)}).
			SetAttribute(VertexAttribute{Kind: AttrColor, Format: FormatFloat32x4, Data: make([]byte, 16)})
	}
	ref := build().Layout().Hash
	for range rounds {
		if h := build().Layout().Hash; h != ref {
			t.Fatalf("hash changed across builds: %d != %d", h, ref)
		}
	}
}
