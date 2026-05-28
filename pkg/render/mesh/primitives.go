package mesh

import (
	"math"
)

// Cube generates a unit cube of the given half-size (side = 2*size).
// Each face has 4 unique vertices for flat normals; 24 vertices total, 36 indices.
// Attributes: Position (Float32x3), Normal (Float32x3), UV0 (Float32x2), Tangent (Float32x4).
func Cube(size float32) *Mesh {
	s := size

	type face struct {
		normal  [3]float32
		tangent [3]float32
		corners [4][3]float32 // 4 corners, ordered CCW from outside
		uvs     [4][2]float32
	}
	faces := [6]face{
		{ // +X right
			normal: [3]float32{1, 0, 0}, tangent: [3]float32{0, 0, -1},
			corners: [4][3]float32{{s, -s, -s}, {s, -s, s}, {s, s, s}, {s, s, -s}},
			uvs:     [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
		},
		{ // -X left
			normal: [3]float32{-1, 0, 0}, tangent: [3]float32{0, 0, 1},
			corners: [4][3]float32{{-s, -s, s}, {-s, -s, -s}, {-s, s, -s}, {-s, s, s}},
			uvs:     [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
		},
		{ // +Y top
			normal: [3]float32{0, 1, 0}, tangent: [3]float32{1, 0, 0},
			corners: [4][3]float32{{-s, s, -s}, {s, s, -s}, {s, s, s}, {-s, s, s}},
			uvs:     [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
		},
		{ // -Y bottom
			normal: [3]float32{0, -1, 0}, tangent: [3]float32{1, 0, 0},
			corners: [4][3]float32{{-s, -s, s}, {s, -s, s}, {s, -s, -s}, {-s, -s, -s}},
			uvs:     [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
		},
		{ // +Z front
			normal: [3]float32{0, 0, 1}, tangent: [3]float32{1, 0, 0},
			corners: [4][3]float32{{-s, -s, s}, {s, -s, s}, {s, s, s}, {-s, s, s}},
			uvs:     [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
		},
		{ // -Z back
			normal: [3]float32{0, 0, -1}, tangent: [3]float32{-1, 0, 0},
			corners: [4][3]float32{{s, -s, -s}, {-s, -s, -s}, {-s, s, -s}, {s, s, -s}},
			uvs:     [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
		},
	}

	const vertsPerFace = 4
	const trisPerFace = 2
	const idxPerFace = trisPerFace * 3
	nVerts := 6 * vertsPerFace
	nIdx := 6 * idxPerFace

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	tang := make([]byte, nVerts*16)
	idx := make([]byte, nIdx*4)

	vi := 0
	ii := 0
	for _, f := range faces {
		base := vi
		for c := range 4 {
			putF32x3(pos, vi, f.corners[c][0], f.corners[c][1], f.corners[c][2])
			putF32x3(norm, vi, f.normal[0], f.normal[1], f.normal[2])
			putF32x2(uv, vi, f.uvs[c][0], f.uvs[c][1])
			putF32x4(tang, vi, f.tangent[0], f.tangent[1], f.tangent[2], 1)
			vi++
		}
		// Two triangles: (0,1,2) and (0,2,3).
		b := uint32(base)
		putU32(idx, ii, b); ii++; putU32(idx, ii, b+1); ii++; putU32(idx, ii, b+2); ii++
		putU32(idx, ii, b); ii++; putU32(idx, ii, b+2); ii++; putU32(idx, ii, b+3); ii++
	}

	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv}).
		SetAttribute(VertexAttribute{Kind: AttrTangent, Format: FormatFloat32x4, Data: tang})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// Sphere generates a UV sphere with the given radius, sector (longitude) count,
// and stack (latitude) count. Minimum sectors=3, stacks=2.
// Attributes: Position, Normal, UV0, Tangent.
func Sphere(radius float32, sectors, stacks int) *Mesh {
	if sectors < 3 {
		sectors = 3
	}
	if stacks < 2 {
		stacks = 2
	}

	// (stacks+1) latitude rings × (sectors+1) unique U values = one seam vertex per ring.
	nVerts := (stacks + 1) * (sectors + 1)
	nIdx := stacks * sectors * 6

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	tang := make([]byte, nVerts*16)
	idx := make([]byte, nIdx*4)

	vi := 0
	for si := range stacks + 1 {
		phi := math.Pi/2 - float64(si)*math.Pi/float64(stacks) // +π/2 to -π/2
		y := radius * float32(math.Sin(phi))
		cosPhi := float32(math.Cos(phi))
		v := float32(si) / float32(stacks)
		for sec := range sectors + 1 {
			theta := float64(sec) * 2 * math.Pi / float64(sectors)
			x := radius * cosPhi * float32(math.Cos(theta))
			z := radius * cosPhi * float32(math.Sin(theta))
			nx := cosPhi * float32(math.Cos(theta))
			ny := float32(math.Sin(phi))
			nz := cosPhi * float32(math.Sin(theta))
			u := float32(sec) / float32(sectors)
			putF32x3(pos, vi, x, y, z)
			putF32x3(norm, vi, nx, ny, nz)
			putF32x2(uv, vi, u, v)
			// Tangent along +U: derivative of position w.r.t. theta.
			tx := -float32(math.Sin(theta))
			tz := float32(math.Cos(theta))
			putF32x4(tang, vi, tx, 0, tz, 1)
			vi++
		}
	}

	ii := 0
	row := sectors + 1
	for si := range stacks {
		for sec := range sectors {
			k1 := uint32(si*row + sec)
			k2 := k1 + uint32(row)
			if si != 0 {
				putU32(idx, ii, k1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k1+1); ii++
			}
			if si != stacks-1 {
				putU32(idx, ii, k1+1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k2+1); ii++
			}
		}
	}
	// Trim unused index slots (poles have fewer triangles).
	idx = idx[:ii*4]

	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv}).
		SetAttribute(VertexAttribute{Kind: AttrTangent, Format: FormatFloat32x4, Data: tang})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// Plane generates an axis-aligned XZ plane of the given world-space side size,
// subdivided into (subdivisions+1)² quads. Faces upward (+Y normal).
func Plane(size float32, subdivisions int) *Mesh {
	if subdivisions < 0 {
		subdivisions = 0
	}
	n := subdivisions + 1 // cells per axis
	nVerts := (n + 1) * (n + 1)
	nIdx := n * n * 6

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	idx := make([]byte, nIdx*4)

	half := size / 2
	vi := 0
	for row := range n + 1 {
		for col := range n + 1 {
			x := -half + float32(col)*size/float32(n)
			z := -half + float32(row)*size/float32(n)
			putF32x3(pos, vi, x, 0, z)
			putF32x3(norm, vi, 0, 1, 0)
			putF32x2(uv, vi, float32(col)/float32(n), float32(row)/float32(n))
			vi++
		}
	}
	ii := 0
	rowStride := n + 1
	for row := range n {
		for col := range n {
			k := uint32(row*rowStride + col)
			putU32(idx, ii, k); ii++; putU32(idx, ii, k+1); ii++; putU32(idx, ii, k+uint32(rowStride)+1); ii++
			putU32(idx, ii, k); ii++; putU32(idx, ii, k+uint32(rowStride)+1); ii++; putU32(idx, ii, k+uint32(rowStride)); ii++
		}
	}
	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// Cylinder generates a capped cylinder aligned along the Y axis.
// radius: cap radius. height: total height. sectors: longitudinal subdivisions.
func Cylinder(radius, height float32, sectors int) *Mesh {
	if sectors < 3 {
		sectors = 3
	}
	halfH := height / 2
	// Side vertices: (sectors+1) vertices per ring × 2 rings.
	// Cap centres + (sectors+1) perimeter vertices × 2 caps.
	sideVerts := 2 * (sectors + 1)
	capVerts := 2 * (sectors + 2) // centre + ring, times 2
	nVerts := sideVerts + capVerts
	sideIdx := sectors * 6
	capIdx := sectors * 3 * 2
	nIdx := sideIdx + capIdx

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	idx := make([]byte, nIdx*4)

	vi := 0
	ii := 0

	// Side rings (bottom y=-halfH, top y=+halfH).
	for _, y := range []float32{-halfH, halfH} {
		for sec := range sectors + 1 {
			theta := float64(sec) * 2 * math.Pi / float64(sectors)
			x := radius * float32(math.Cos(theta))
			z := radius * float32(math.Sin(theta))
			putF32x3(pos, vi, x, y, z)
			putF32x3(norm, vi, float32(math.Cos(theta)), 0, float32(math.Sin(theta)))
			putF32x2(uv, vi, float32(sec)/float32(sectors), (y+halfH)/height)
			vi++
		}
	}
	// Side indices.
	for sec := range sectors {
		b := uint32(sec)
		t := b + uint32(sectors+1)
		putU32(idx, ii, b); ii++; putU32(idx, ii, t); ii++; putU32(idx, ii, b+1); ii++
		putU32(idx, ii, b+1); ii++; putU32(idx, ii, t); ii++; putU32(idx, ii, t+1); ii++
	}

	// Caps (bottom, top).
	for _, sign := range []float32{-1, 1} {
		y := sign * halfH
		ny := sign
		centre := vi
		putF32x3(pos, vi, 0, y, 0)
		putF32x3(norm, vi, 0, ny, 0)
		putF32x2(uv, vi, 0.5, 0.5)
		vi++
		ring0 := vi
		for sec := range sectors + 1 {
			theta := float64(sec) * 2 * math.Pi / float64(sectors)
			x := radius * float32(math.Cos(theta))
			z := radius * float32(math.Sin(theta))
			putF32x3(pos, vi, x, y, z)
			putF32x3(norm, vi, 0, ny, 0)
			putF32x2(uv, vi, 0.5+0.5*float32(math.Cos(theta)), 0.5+0.5*float32(math.Sin(theta)))
			vi++
		}
		for sec := range sectors {
			a := uint32(ring0 + sec)
			b := uint32(ring0 + sec + 1)
			c := uint32(centre)
			if sign > 0 {
				putU32(idx, ii, c); ii++; putU32(idx, ii, b); ii++; putU32(idx, ii, a); ii++
			} else {
				putU32(idx, ii, c); ii++; putU32(idx, ii, a); ii++; putU32(idx, ii, b); ii++
			}
		}
	}
	idx = idx[:ii*4]
	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// Capsule generates a capsule (cylinder with hemispherical caps) aligned on Y.
// radius: sphere radius. height: total height (must be ≥ 2*radius).
// sectors: longitudinal subdivisions. stacks: hemisphere ring count.
func Capsule(radius, height float32, sectors, stacks int) *Mesh {
	if sectors < 3 {
		sectors = 3
	}
	if stacks < 1 {
		stacks = 1
	}
	if height < 2*radius {
		height = 2 * radius
	}
	midH := (height - 2*radius) / 2

	// Top hemisphere (stacks+1 rings) + bottom hemisphere (stacks+1 rings)
	// + cylinder middle (2 rings).
	totalRings := 2*(stacks+1) + 2
	nVerts := totalRings * (sectors + 1)
	nIdx := (totalRings - 1) * sectors * 6

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	idx := make([]byte, nIdx*4)

	vi := 0
	addRing := func(y float32, nx, ny, nz float32, uvV float32) {
		for sec := range sectors + 1 {
			theta := float64(sec) * 2 * math.Pi / float64(sectors)
			ct := float32(math.Cos(theta))
			st := float32(math.Sin(theta))
			putF32x3(pos, vi, ct*nx*radius, y, st*nz*radius)
			putF32x3(norm, vi, ct*nx, ny, st*nz)
			putF32x2(uv, vi, float32(sec)/float32(sectors), uvV)
			vi++
		}
	}
	_ = addRing
	_ = midH

	// Use simpler per-vertex computation.
	type ring struct{ y, rx, ry, rz, uvV float32 }
	rings := make([]ring, 0, totalRings)
	uvStep := 1.0 / float32(totalRings-1)

	// Bottom hemisphere: phi from -π/2 to 0.
	for s := range stacks + 1 {
		phi := -math.Pi/2 + float64(s)*math.Pi/2/float64(stacks)
		cosPhi := float32(math.Cos(phi))
		sinPhi := float32(math.Sin(phi))
		rings = append(rings, ring{
			y:   -midH + radius*sinPhi,
			rx:  cosPhi, ry: sinPhi, rz: cosPhi,
			uvV: float32(s) * uvStep,
		})
	}
	// Middle cylinder (two rings: bottom edge, top edge of cylinder).
	rings = append(rings, ring{y: -midH, rx: 1, ry: 0, rz: 1, uvV: float32(stacks+1) * uvStep})
	rings = append(rings, ring{y: midH, rx: 1, ry: 0, rz: 1, uvV: float32(stacks+2) * uvStep})
	// Top hemisphere: phi from 0 to π/2.
	for s := range stacks + 1 {
		phi := float64(s) * math.Pi / 2 / float64(stacks)
		cosPhi := float32(math.Cos(phi))
		sinPhi := float32(math.Sin(phi))
		rings = append(rings, ring{
			y:   midH + radius*sinPhi,
			rx:  cosPhi, ry: sinPhi, rz: cosPhi,
			uvV: float32(stacks+3+s) * uvStep,
		})
	}

	for _, r := range rings {
		for sec := range sectors + 1 {
			theta := float64(sec) * 2 * math.Pi / float64(sectors)
			ct := float32(math.Cos(theta))
			st := float32(math.Sin(theta))
			x := r.rx * ct * radius
			z := r.rz * st * radius
			putF32x3(pos, vi, x, r.y, z)
			putF32x3(norm, vi, r.rx*ct, r.ry, r.rz*st)
			putF32x2(uv, vi, float32(sec)/float32(sectors), r.uvV)
			vi++
		}
	}

	ii := 0
	nR := len(rings)
	stride := sectors + 1
	for r := range nR - 1 {
		for sec := range sectors {
			k1 := uint32(r*stride + sec)
			k2 := k1 + uint32(stride)
			putU32(idx, ii, k1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k1+1); ii++
			putU32(idx, ii, k1+1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k2+1); ii++
		}
	}
	idx = idx[:ii*4]
	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// Torus generates a torus with the given major radius (R, from centre to tube
// centre) and minor radius (r, tube radius). majorSectors and minorSectors
// control tessellation of the ring and tube respectively.
func Torus(majorR, minorR float32, majorSectors, minorSectors int) *Mesh {
	if majorSectors < 3 {
		majorSectors = 3
	}
	if minorSectors < 3 {
		minorSectors = 3
	}
	nVerts := (majorSectors + 1) * (minorSectors + 1)
	nIdx := majorSectors * minorSectors * 6

	pos := make([]byte, nVerts*12)
	norm := make([]byte, nVerts*12)
	uv := make([]byte, nVerts*8)
	idx := make([]byte, nIdx*4)

	vi := 0
	for maj := range majorSectors + 1 {
		phi := float64(maj) * 2 * math.Pi / float64(majorSectors)
		cosPhi := float32(math.Cos(phi))
		sinPhi := float32(math.Sin(phi))
		cx := majorR * cosPhi // tube centre
		cy := majorR * sinPhi
		for min := range minorSectors + 1 {
			theta := float64(min) * 2 * math.Pi / float64(minorSectors)
			cosTheta := float32(math.Cos(theta))
			sinTheta := float32(math.Sin(theta))
			x := (majorR + minorR*cosTheta) * cosPhi
			y := (majorR + minorR*cosTheta) * sinPhi
			z := minorR * sinTheta
			nx := cosTheta * cosPhi
			ny := cosTheta * sinPhi
			nz := sinTheta
			// Tube-centre direction for normal reference.
			_ = cx
			_ = cy
			putF32x3(pos, vi, x, z, y) // Y-up: swap y↔z
			putF32x3(norm, vi, nx, nz, ny)
			putF32x2(uv, vi, float32(maj)/float32(majorSectors), float32(min)/float32(minorSectors))
			vi++
		}
	}
	ii := 0
	stride := minorSectors + 1
	for maj := range majorSectors {
		for min := range minorSectors {
			k1 := uint32(maj*stride + min)
			k2 := k1 + uint32(stride)
			putU32(idx, ii, k1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k1+1); ii++
			putU32(idx, ii, k1+1); ii++; putU32(idx, ii, k2); ii++; putU32(idx, ii, k2+1); ii++
		}
	}
	m := NewMesh(TopologyTriangleList).
		SetAttribute(VertexAttribute{Kind: AttrPosition, Format: FormatFloat32x3, Data: pos}).
		SetAttribute(VertexAttribute{Kind: AttrNormal, Format: FormatFloat32x3, Data: norm}).
		SetAttribute(VertexAttribute{Kind: AttrUV0, Format: FormatFloat32x2, Data: uv})
	m.SetIndices(IndexBuffer{Wide: true, Data: idx})
	return m
}

// ─── Byte-level helpers ───────────────────────────────────────────────────────

func putF32(buf []byte, byteOffset int, v float32) {
	bits := math.Float32bits(v)
	buf[byteOffset+0] = byte(bits)
	buf[byteOffset+1] = byte(bits >> 8)
	buf[byteOffset+2] = byte(bits >> 16)
	buf[byteOffset+3] = byte(bits >> 24)
}

func putF32x2(buf []byte, vi int, x, y float32) {
	o := vi * 8
	putF32(buf, o, x)
	putF32(buf, o+4, y)
}

func putF32x3(buf []byte, vi int, x, y, z float32) {
	o := vi * 12
	putF32(buf, o, x)
	putF32(buf, o+4, y)
	putF32(buf, o+8, z)
}

func putF32x4(buf []byte, vi int, x, y, z, w float32) {
	o := vi * 16
	putF32(buf, o, x)
	putF32(buf, o+4, y)
	putF32(buf, o+8, z)
	putF32(buf, o+12, w)
}

func putU32(buf []byte, ii int, v uint32) {
	o := ii * 4
	buf[o+0] = byte(v)
	buf[o+1] = byte(v >> 8)
	buf[o+2] = byte(v >> 16)
	buf[o+3] = byte(v >> 24)
}
