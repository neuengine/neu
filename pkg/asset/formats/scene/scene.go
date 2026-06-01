// Package scene provides the .scene.json asset loader: it decodes the engine's
// portable scene format ([enginescene.SerializedScene]) from JSON, reusing the
// Phase 3 scene codec (no new dependency, C-003).
//
// Scope: the loader does the *decode* half — bytes → a validated, portable
// SerializedScene. Hydrating that into live World entities (which needs a
// TypeRegistry) is the scene system's spawn path ([enginescene] Spawn), deferred
// to App integration.
//
// Bootstrap: l2-asset-formats-go (graduates the spec's `.scene.json` item).
package scene

import (
	"errors"
	"fmt"
	"io"

	"github.com/neuengine/neu/pkg/asset"
	enginescene "github.com/neuengine/neu/pkg/scene"
)

// LoadSettings carries optional scene decoding hints (none yet).
type LoadSettings struct{}

// SceneJSONLoader decodes a .scene.json document into a portable SerializedScene.
type SceneJSONLoader struct{}

var _ asset.AssetLoader[enginescene.SerializedScene, LoadSettings] = SceneJSONLoader{}

// Extensions reports the file extension this loader handles. NOTE: the
// AssetServer registry matches on the final path extension (`path.Ext` →
// ".json" for "*.scene.json"); routing compound extensions is a deferred
// asset-system enhancement, so for now invoke the loader via Load/Decode.
func (SceneJSONLoader) Extensions() []string { return []string{".scene.json"} }

// Load decodes r into a SerializedScene (INV-2: full asset or error).
func (SceneJSONLoader) Load(r io.Reader, _ LoadSettings) (enginescene.SerializedScene, error) {
	return Decode(r)
}

// Decode reads a .scene.json document and returns a validated SerializedScene.
// Malformed JSON or out-of-range interning indices yield an error — never a
// partial asset (INV-2). The base JSON codec does not range-check indices, so
// this loader does, mirroring the binary codec's guards.
func Decode(r io.Reader) (enginescene.SerializedScene, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return enginescene.SerializedScene{}, fmt.Errorf("scene: read input: %w", err)
	}
	sc, err := enginescene.UnmarshalJSON(data)
	if err != nil {
		return enginescene.SerializedScene{}, err
	}
	if err := validate(&sc); err != nil {
		return enginescene.SerializedScene{}, err
	}
	return sc, nil
}

// validate confirms every interning index resolves within Names/Variants.
func validate(sc *enginescene.SerializedScene) error {
	nNames, nVars := len(sc.Names), len(sc.Variants)
	for ei, e := range sc.Entities {
		if e.NameIdx < 0 || e.NameIdx >= nNames {
			return fmt.Errorf("scene: entity %d nameIdx %d out of range [0,%d)", ei, e.NameIdx, nNames)
		}
		for ci, c := range e.Components {
			if c.TypeIdx < 0 || c.TypeIdx >= nNames {
				return fmt.Errorf("scene: entity %d comp %d typeIdx %d out of range [0,%d)", ei, ci, c.TypeIdx, nNames)
			}
			for _, p := range c.Props {
				if p[0] < 0 || p[0] >= nNames || p[1] < 0 || p[1] >= nVars {
					return fmt.Errorf("scene: entity %d comp %d prop index %v out of range", ei, ci, p)
				}
			}
		}
	}
	return nil
}

// RegisterAll registers the .scene.json loader with the given AssetServer.
// See [SceneJSONLoader.Extensions] for the compound-extension routing caveat.
func RegisterAll(srv *asset.AssetServer) error {
	if srv == nil {
		return errors.New("asset/formats/scene: nil AssetServer")
	}
	asset.RegisterLoader(srv, SceneJSONLoader{})
	return nil
}
