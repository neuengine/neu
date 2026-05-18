package scene

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// SerializedScene is the portable interned representation of a DynamicScene.
//
// Interning: component type names and variant values are stored once in Names
// and Variants respectively. All per-entity references use integer indices,
// giving ~5-10× size reduction over naive per-entity string repetition.
type SerializedScene struct {
	Names    []string           `json:"names"`
	Variants []any              `json:"variants"`
	Entities []SerializedEntity `json:"entities"`
}

// SerializedEntity is a single entity in the portable format.
type SerializedEntity struct {
	NameIdx    int                   `json:"nameIdx"`
	Components []SerializedComponent `json:"components"`
}

// SerializedComponent holds the type index and its field (name, value) index pairs.
type SerializedComponent struct {
	TypeIdx int      `json:"typeIdx"`
	Props   [][2]int `json:"props"`
}

// EntityCount returns the number of entities in the scene.
func (s *SerializedScene) EntityCount() int { return len(s.Entities) }

// ComponentType returns the type-name string for entity i, component j.
func (s *SerializedScene) ComponentType(entityIdx, compIdx int) string {
	if entityIdx < 0 || entityIdx >= len(s.Entities) {
		return ""
	}
	e := s.Entities[entityIdx]
	if compIdx < 0 || compIdx >= len(e.Components) {
		return ""
	}
	idx := e.Components[compIdx].TypeIdx
	if idx < 0 || idx >= len(s.Names) {
		return ""
	}
	return s.Names[idx]
}

// PropertyValue returns the value at (entityIdx, compIdx, propIdx).
func (s *SerializedScene) PropertyValue(entityIdx, compIdx, propIdx int) any {
	if entityIdx < 0 || entityIdx >= len(s.Entities) {
		return nil
	}
	e := s.Entities[entityIdx]
	if compIdx < 0 || compIdx >= len(e.Components) {
		return nil
	}
	comp := e.Components[compIdx]
	if propIdx < 0 || propIdx >= len(comp.Props) {
		return nil
	}
	vidx := comp.Props[propIdx][1]
	if vidx < 0 || vidx >= len(s.Variants) {
		return nil
	}
	return s.Variants[vidx]
}

// ─── JSON codec ──────────────────────────────────────────────────────────────

// MarshalJSON encodes sc to canonical JSON.
func MarshalJSON(sc *SerializedScene) ([]byte, error) {
	return json.Marshal(sc)
}

// UnmarshalJSON decodes a JSON-encoded SerializedScene.
func UnmarshalJSON(data []byte) (SerializedScene, error) {
	var sc SerializedScene
	if err := json.Unmarshal(data, &sc); err != nil {
		return SerializedScene{}, fmt.Errorf("scene: unmarshal JSON: %w", err)
	}
	return sc, nil
}

// ─── Interned binary codec ───────────────────────────────────────────────────
//
// Wire format (little-endian):
//   magic   [4]byte  "NSCN"
//   version uint16   (current: 1)
//   nNames  uint32
//   names   []string (each: uint32 len + bytes)
//   nVar    uint32
//   vars    []string (each variant encoded as its JSON representation)
//   nEnt    uint32
//   entities []entity:
//     nameIdx uint32
//     nComp   uint32
//     comps   []comp:
//       typeIdx uint32
//       nProp   uint32
//       props   [][2]uint32

const binaryMagic = "NSCN"
const binaryVersion = uint16(1)

// ErrInvalidBinary is returned when the binary data is corrupt or malformed.
var ErrInvalidBinary = errors.New("scene: invalid binary data")

// MarshalBinary encodes sc to the compact interned binary format.
func MarshalBinary(sc *SerializedScene) ([]byte, error) {
	var buf bytes.Buffer
	w := &buf

	writeBytes(w, []byte(binaryMagic))
	writeUint16(w, binaryVersion)
	writeUint32(w, uint32(len(sc.Names)))
	for _, name := range sc.Names {
		writeString(w, name)
	}
	writeUint32(w, uint32(len(sc.Variants)))
	for _, v := range sc.Variants {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("scene: marshal binary variant: %w", err)
		}
		writeString(w, string(encoded))
	}
	writeUint32(w, uint32(len(sc.Entities)))
	for _, ent := range sc.Entities {
		writeUint32(w, uint32(ent.NameIdx))
		writeUint32(w, uint32(len(ent.Components)))
		for _, comp := range ent.Components {
			writeUint32(w, uint32(comp.TypeIdx))
			writeUint32(w, uint32(len(comp.Props)))
			for _, prop := range comp.Props {
				writeUint32(w, uint32(prop[0]))
				writeUint32(w, uint32(prop[1]))
			}
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary decodes a binary-encoded SerializedScene.
// Returns ErrInvalidBinary on any structural error (malformed indices, truncated data).
func UnmarshalBinary(data []byte) (SerializedScene, error) {
	r := bytes.NewReader(data)

	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != binaryMagic {
		return SerializedScene{}, fmt.Errorf("%w: bad magic", ErrInvalidBinary)
	}
	ver, err := readUint16(r)
	if err != nil {
		return SerializedScene{}, fmt.Errorf("%w: version read: %v", ErrInvalidBinary, err)
	}
	if ver > binaryVersion {
		return SerializedScene{}, fmt.Errorf("%w: %w: version %d > %d",
			ErrSceneFromFuture, ErrInvalidBinary, ver, binaryVersion)
	}

	nNames, err := readUint32(r)
	if err != nil {
		return SerializedScene{}, wrapBinary("nNames", err)
	}
	names := make([]string, nNames)
	for i := range names {
		names[i], err = readString(r)
		if err != nil {
			return SerializedScene{}, wrapBinary("name", err)
		}
	}

	nVar, err := readUint32(r)
	if err != nil {
		return SerializedScene{}, wrapBinary("nVar", err)
	}
	variants := make([]any, nVar)
	for i := range variants {
		s, err := readString(r)
		if err != nil {
			return SerializedScene{}, wrapBinary("variant", err)
		}
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return SerializedScene{}, fmt.Errorf("%w: variant JSON: %v", ErrInvalidBinary, err)
		}
		variants[i] = v
	}

	nEnt, err := readUint32(r)
	if err != nil {
		return SerializedScene{}, wrapBinary("nEnt", err)
	}
	entities := make([]SerializedEntity, nEnt)
	for i := range entities {
		nameIdx, err := readUint32(r)
		if err != nil {
			return SerializedScene{}, wrapBinary("entity.nameIdx", err)
		}
		if int(nameIdx) >= len(names) {
			return SerializedScene{}, fmt.Errorf("%w: nameIdx %d out of range", ErrInvalidBinary, nameIdx)
		}
		nComp, err := readUint32(r)
		if err != nil {
			return SerializedScene{}, wrapBinary("entity.nComp", err)
		}
		comps := make([]SerializedComponent, nComp)
		for j := range comps {
			typeIdx, err := readUint32(r)
			if err != nil {
				return SerializedScene{}, wrapBinary("comp.typeIdx", err)
			}
			if int(typeIdx) >= len(names) {
				return SerializedScene{}, fmt.Errorf("%w: typeIdx %d out of range", ErrInvalidBinary, typeIdx)
			}
			nProp, err := readUint32(r)
			if err != nil {
				return SerializedScene{}, wrapBinary("comp.nProp", err)
			}
			props := make([][2]int, nProp)
			for k := range props {
				ni, err := readUint32(r)
				if err != nil {
					return SerializedScene{}, wrapBinary("prop.nameIdx", err)
				}
				vi, err := readUint32(r)
				if err != nil {
					return SerializedScene{}, wrapBinary("prop.valueIdx", err)
				}
				if int(ni) >= len(names) || int(vi) >= len(variants) {
					return SerializedScene{}, fmt.Errorf("%w: prop index out of range", ErrInvalidBinary)
				}
				props[k] = [2]int{int(ni), int(vi)}
			}
			comps[j] = SerializedComponent{TypeIdx: int(typeIdx), Props: props}
		}
		entities[i] = SerializedEntity{NameIdx: int(nameIdx), Components: comps}
	}

	return SerializedScene{Names: names, Variants: variants, Entities: entities}, nil
}

// ─── binary helpers ──────────────────────────────────────────────────────────

func writeBytes(w *bytes.Buffer, b []byte)  { w.Write(b) }
func writeUint16(w *bytes.Buffer, v uint16) { binary.Write(w, binary.LittleEndian, v) }
func writeUint32(w *bytes.Buffer, v uint32) { binary.Write(w, binary.LittleEndian, v) }
func writeString(w *bytes.Buffer, s string) {
	writeUint32(w, uint32(len(s)))
	w.WriteString(s)
}

func readUint16(r *bytes.Reader) (uint16, error) {
	var v uint16
	return v, binary.Read(r, binary.LittleEndian, &v)
}

func readUint32(r *bytes.Reader) (uint32, error) {
	var v uint32
	return v, binary.Read(r, binary.LittleEndian, &v)
}

func readString(r *bytes.Reader) (string, error) {
	n, err := readUint32(r)
	if err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}

func wrapBinary(field string, err error) error {
	return fmt.Errorf("%w: read %s: %v", ErrInvalidBinary, field, err)
}
