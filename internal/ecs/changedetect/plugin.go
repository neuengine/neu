package changedetect

// ChangeDetectionPlugin wires change-detection into the app schedule.
// It registers [world.World.ClearTrackers] in the Last system set so that
// tick baselines advance and stale removal entries are pruned once per frame.
//
// Consumed by DefaultPlugins in T-2F03 once pkg/app provides the Plugin
// interface. Register via App.AddPlugin(ChangeDetectionPlugin{}).
type ChangeDetectionPlugin struct{}
