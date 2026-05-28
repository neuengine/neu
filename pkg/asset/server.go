package asset

import (
	"reflect"
	"sync"

	"github.com/neuengine/neu/pkg/task"
)

// inFlight tracks an in-progress load so concurrent Load calls for the same
// path share a single IOPool submission and return handles to the same slot.
type inFlight struct {
	r  *rc // shared refcount; callers Clone on access
	id AssetID
}

// AssetServer coordinates async asset loading, the loader registry, and the
// VFS. It delegates IO work to the IOPool so asset decoding never blocks
// compute workers.
type AssetServer struct {
	vfs      *VFS
	io       *task.IOPool
	loaders  *loaderRegistry
	stores   map[reflect.Type]any
	inflight map[string]*inFlight
	mu       sync.Mutex
}

// NewAssetServer returns a server backed by vfs and the IO pool.
func NewAssetServer(vfs *VFS, io *task.IOPool) *AssetServer {
	return &AssetServer{
		vfs:      vfs,
		io:       io,
		loaders:  newLoaderRegistry(),
		stores:   make(map[reflect.Type]any),
		inflight: make(map[string]*inFlight),
	}
}

// storeFor returns the typed store for A, creating it on first use.
// mu must be held by the caller.
func storeFor[A any](s *AssetServer) *Assets[A] {
	t := reflect.TypeFor[A]()
	if raw, ok := s.stores[t]; ok {
		return raw.(*Assets[A])
	}
	a := NewAssets[A]()
	s.stores[t] = a
	return a
}

// RegisterLoader registers l for each extension it reports. Must be called
// before any Load call for asset type A.
func RegisterLoader[A any, S any](s *AssetServer, l AssetLoader[A, S]) {
	s.mu.Lock()
	s.loaders.register(&typedLoader[A, S]{l: l})
	s.mu.Unlock()
}

// Load returns a Handle[A] immediately; the actual decoding runs on the
// IOPool. Concurrent calls for the same path share a single loader invocation
// (in-flight dedup) — all callers receive independent strong handles to the
// same slot.
func Load[A any](s *AssetServer, path string) Handle[A] {
	return LoadWithSettings[A, struct{}](s, path, struct{}{})
}

// LoadWithSettings is like Load but passes settings to the loader.
func LoadWithSettings[A any, S any](s *AssetServer, path string, settings S) Handle[A] {
	s.mu.Lock()

	// In-flight dedup: reuse the existing rc if this path is already loading.
	if entry, ok := s.inflight[path]; ok {
		entry.r.n.Add(1) // Clone — must be atomic
		id := entry.id
		r := entry.r
		s.mu.Unlock()
		return Handle[A]{id: id, r: r}
	}

	store := storeFor[A](s)
	h, slot := store.addLoading()
	entry := &inFlight{r: h.r, id: h.id}
	s.inflight[path] = entry
	s.mu.Unlock()

	// Submit decode to the IOPool — never blocks the calling goroutine.
	loadPath := path
	_ = s.io.Go(func() {
		defer func() {
			s.mu.Lock()
			delete(s.inflight, loadPath)
			s.mu.Unlock()
		}()

		f, err := s.vfs.Open(loadPath)
		if err != nil {
			store.mu.Lock()
			slot.state = Failed
			slot.err = err
			store.mu.Unlock()
			return
		}
		defer func() { _ = f.Close() }()

		loader, err := s.loaders.find(loadPath)
		if err != nil {
			store.mu.Lock()
			slot.state = Failed
			slot.err = err
			store.mu.Unlock()
			return
		}

		// loadFrom sets slot.state to Loaded or Failed and writes slot.val.
		store.mu.Lock()
		_ = loader.loadFrom(f, slot)
		store.mu.Unlock()
	})

	return h
}

// GetLoadState returns the LoadState for the given handle's slot.
func GetLoadState[A any](s *AssetServer, h Handle[A]) LoadState {
	s.mu.Lock()
	store := storeFor[A](s)
	s.mu.Unlock()
	return store.GetLoadState(h.id)
}

// Reload marks the slot as Loading and re-submits the decode task, allowing
// hot-reload of both Loaded and Failed assets.
func Reload[A any](s *AssetServer, path string) Handle[A] {
	return Load[A](s, path)
}

// Unload explicitly removes the slot regardless of its refcount. Callers
// should ensure no live Handles remain for the slot before calling Unload.
func Unload[A any](s *AssetServer, h Handle[A]) {
	s.mu.Lock()
	store := storeFor[A](s)
	s.mu.Unlock()
	store.Remove(h.id)
}
