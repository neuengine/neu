package asset

import "reflect"

// AssetEventKind classifies an [AssetEvent].
type AssetEventKind uint8

const (
	// AssetCreated marks an asset's first load. Reserved — not emitted yet.
	AssetCreated AssetEventKind = iota
	// AssetModified marks a hot-reload of an already-loaded asset.
	AssetModified
	// AssetRemoved marks an unloaded asset. Reserved — not emitted yet.
	AssetRemoved
)

// AssetEvent reports a lifecycle change for an asset of type A, addressed by
// both its stable source Path and its current AssetID. It is the typed payload
// a hot-reload-aware system reads via an EventReader[AssetEvent[A]] (the ECS
// bridge that publishes it lives in internal/asset). [Reload] emits
// AssetModified; AssetCreated / AssetRemoved are reserved for future wiring.
type AssetEvent[A any] struct {
	Kind AssetEventKind
	ID   AssetID
	Path string
}

// loadedRef records the type and current slot id of the asset most recently
// loaded from a given path, so a path-keyed reload (the dev file watcher) can
// resolve to the correct typed [Reload] and event.
type loadedRef struct {
	typ reflect.Type
	id  AssetID
}

// WatchReloads registers, for asset type A, the machinery that turns a reload
// into a typed [AssetEvent]: emit is invoked whenever [Reload] runs for type A,
// and a path-keyed dispatcher lets a runtime-typed trigger (the dev file
// watcher, via [AssetServer.dispatchReload]) re-enter the generic Reload[A].
//
// emit stays ECS-free — the ECS bridge (internal/asset) supplies an emit that
// sends the event on the world's event bus. Registering the same type twice
// replaces the prior emitter.
func WatchReloads[A any](s *AssetServer, emit func(AssetEvent[A])) {
	typ := reflect.TypeFor[A]()
	s.mu.Lock()
	s.reloadEmitters[typ] = func(path string, id AssetID) {
		emit(AssetEvent[A]{Kind: AssetModified, ID: id, Path: path})
	}
	s.reloadDispatch[typ] = func(path string) { Reload[A](s, path) }
	s.mu.Unlock()
}

// dispatchReload re-enters the typed [Reload] for whatever asset type was last
// loaded from path. It is the dev file watcher's onReload callback (see
// [AssetServer.NewReloadWatcher]); for an unknown or unwatched path it is a
// no-op.
func (s *AssetServer) dispatchReload(path string) {
	s.mu.Lock()
	ref, known := s.loaded[path]
	var dispatch func(string)
	if known {
		dispatch = s.reloadDispatch[ref.typ]
	}
	s.mu.Unlock()
	if dispatch != nil {
		dispatch(path)
	}
}
