// Package asset bridges the pkg/asset reload mechanism to the ECS event bus.
// [WatchAssetType] makes Reload[A] publish an asset.AssetEvent[A] on a World's
// event bus, which hot-reload-aware systems (e.g. definition hot-reload, T-6A03)
// read via an EventReader. The dev-mode file watcher that triggers reloads is
// started separately via (*asset.AssetServer).NewReloadWatcher (dev builds only)
// — this bridge only connects the emit side, so it is build-tag-free.
package asset

import (
	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
	pkgasset "github.com/neuengine/neu/pkg/asset"
)

// WatchAssetType registers asset.AssetEvent[A] on w's event bus and wires
// pkg/asset's reload emitter to send onto it. After this, every Reload[A] for
// the server s — whether triggered manually or by the dev file watcher —
// publishes an AssetEvent[A] readable by any system the next frame (the event
// bus is rotated each frame by the ECS EventsPlugin). Call once per (A, World).
func WatchAssetType[A any](s *pkgasset.AssetServer, w *world.World) {
	event.RegisterEvent[pkgasset.AssetEvent[A]](w)
	pkgasset.WatchReloads[A](s, func(ev pkgasset.AssetEvent[A]) {
		if bus := event.Bus[pkgasset.AssetEvent[A]](w); bus != nil {
			bus.Send(ev)
		}
	})
}
