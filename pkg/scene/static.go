// Package scene implements scene capture and restore: StaticScene (fast gob
// blob with build-hash guard) and the interned-binary / JSON codecs for the
// DynamicScene portable format.
//
// C-003: stdlib only — encoding/gob, encoding/json, hash/fnv, runtime.
package scene

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"runtime"
)

// buildHash is computed once at package init from the Go runtime version.
// It guards StaticScene blobs against cross-version loads.
var buildHash uint64

func init() {
	h := fnv.New64a()
	_, _ = h.Write([]byte(runtime.Version()))
	buildHash = h.Sum64()
}

// Sentinel errors for scene loading.
var (
	ErrSceneVersionMismatch = errors.New("scene: StaticScene built by a different engine version")
	ErrSceneFromFuture      = errors.New("scene: format_version newer than this build supports")
	ErrUnremappedReference  = errors.New("scene: ref-bearing component has no remap visitor")
)

// staticBlob is the gob-encodable envelope stored inside a StaticScene.
// It carries the build hash alongside the actual payload so the decoder can
// reject stale blobs before attempting to interpret any data.
type staticBlob struct {
	Hash    uint64
	Payload []byte
}

// StaticScene is an opaque snapshot of arbitrary serializable data.
// It uses encoding/gob internally and is NOT portable across engine builds
// (the build-hash guard makes this limitation safe, not silent).
type StaticScene struct {
	blob []byte // gob-encoded staticBlob
}

// Capture serializes v into a StaticScene. v must be gob-encodable.
// The snapshot is tagged with the current build hash.
func Capture(v any) (StaticScene, error) {
	var payloadBuf bytes.Buffer
	if err := gob.NewEncoder(&payloadBuf).Encode(v); err != nil {
		return StaticScene{}, fmt.Errorf("scene: capture encode: %w", err)
	}
	env := staticBlob{Hash: buildHash, Payload: payloadBuf.Bytes()}
	var envBuf bytes.Buffer
	if err := gob.NewEncoder(&envBuf).Encode(env); err != nil {
		return StaticScene{}, fmt.Errorf("scene: capture envelope: %w", err)
	}
	return StaticScene{blob: envBuf.Bytes()}, nil
}

// Restore decodes the StaticScene back into v (must be a pointer).
// Returns ErrSceneVersionMismatch if the blob was created by a different build.
func (s StaticScene) Restore(v any) error {
	var env staticBlob
	if err := gob.NewDecoder(bytes.NewReader(s.blob)).Decode(&env); err != nil {
		return fmt.Errorf("scene: restore envelope: %w", err)
	}
	if env.Hash != buildHash {
		return fmt.Errorf("%w: saved=%016x current=%016x",
			ErrSceneVersionMismatch, env.Hash, buildHash)
	}
	if err := gob.NewDecoder(bytes.NewReader(env.Payload)).Decode(v); err != nil {
		return fmt.Errorf("scene: restore decode: %w", err)
	}
	return nil
}

// MarshalStatic serializes s to bytes suitable for persistent storage.
func MarshalStatic(s StaticScene) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s.blob); err != nil {
		return nil, fmt.Errorf("scene: marshal static: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalStatic deserializes a StaticScene from bytes produced by MarshalStatic.
func UnmarshalStatic(data []byte) (StaticScene, error) {
	var blob []byte
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&blob); err != nil {
		return StaticScene{}, fmt.Errorf("scene: unmarshal static: %w", err)
	}
	return StaticScene{blob: blob}, nil
}
