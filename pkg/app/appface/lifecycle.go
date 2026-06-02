package appface

// FullPlugin is an optional extension of Plugin that exposes the full
// Build → Ready → Finish → Cleanup lifecycle (L1-app-framework §4.2).
//
// Plugins that only implement Plugin continue to work with Build alone — the
// engine checks via type assertion and only calls the extra methods when
// present (non-breaking; T-6N03b).
//
// App calls:
//
//	Ready   — once, after all plugin Builds have completed (warm-up).
//	Finish  — once, when shutdown begins (stop accepting work).
//	Cleanup — once, after Finish (release resources; called in reverse order).
type FullPlugin interface {
	Plugin
	Ready(app Builder)
	Finish(app Builder)
	Cleanup(app Builder)
}
