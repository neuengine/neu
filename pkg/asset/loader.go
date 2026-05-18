package asset

import (
	"errors"
	"io"
	"path"
	"strings"
)

// ErrNoLoader is returned when no loader is registered for a file extension.
var ErrNoLoader = errors.New("asset: no loader registered for extension")

// assetLoader is the type-erased form stored in the loader registry.
type assetLoader interface {
	// extensions returns the file extensions handled by this loader (e.g. ".png").
	extensions() []string
	// loadInto decodes r and stores the result in the provided *assetSlot.
	// The slot's val field must be settable to the loader's concrete type;
	// callers must ensure type alignment via reflection or generics.
	loadFrom(r io.Reader, slot any) error
}

// AssetLoader is the typed loader interface. A, S are the asset and settings
// types respectively. Implement this interface and register via RegisterLoader.
type AssetLoader[A any, S any] interface {
	// Load decodes r using settings and returns the decoded asset or an error.
	Load(r io.Reader, settings S) (A, error)
	// Extensions returns the file extensions this loader handles (e.g. ".txt").
	Extensions() []string
}

// typedLoader bridges AssetLoader[A,S] to the type-erased assetLoader.
type typedLoader[A any, S any] struct {
	l        AssetLoader[A, S]
	settings S // zero-value settings used when none provided
}

func (tl *typedLoader[A, S]) extensions() []string { return tl.l.Extensions() }

func (tl *typedLoader[A, S]) loadFrom(r io.Reader, slotAny any) error {
	slot := slotAny.(*assetSlot[A])
	val, err := tl.l.Load(r, tl.settings)
	if err != nil {
		slot.state = Failed
		slot.err = err
		return err
	}
	slot.val = val
	slot.state = Loaded
	return nil
}

// loaderRegistry maps file extensions to loaders.
type loaderRegistry struct {
	byExt map[string]assetLoader
}

func newLoaderRegistry() *loaderRegistry {
	return &loaderRegistry{byExt: make(map[string]assetLoader)}
}

// register maps each extension reported by l to l.
func (r *loaderRegistry) register(l assetLoader) {
	for _, ext := range l.extensions() {
		r.byExt[strings.ToLower(ext)] = l
	}
}

// find returns the loader for the given file path's extension, or ErrNoLoader.
func (r *loaderRegistry) find(filePath string) (assetLoader, error) {
	ext := strings.ToLower(path.Ext(filePath))
	l, ok := r.byExt[ext]
	if !ok {
		return nil, ErrNoLoader
	}
	return l, nil
}
