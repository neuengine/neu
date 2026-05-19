package render

// RenderPhase is the fixed-order bucket a draw item belongs to within the main
// scene pass (l1-render-core §4.3). Phases execute in declaration order;
// PhaseNone marks utility passes that participate in no scene phase.
//
// Public because pure-data component specs reference it across packages
// (e.g. l2-materials-and-lighting-go: AlphaMode.Phase() render.RenderPhase).
type RenderPhase uint8

const (
	PhaseNone        RenderPhase = iota // utility / non-scene pass
	PhaseOpaque                         // front-to-back, minimises overdraw
	PhaseAlphaMask                      // front-to-back, fragment discard
	PhaseTransparent                    // back-to-front, blended
	PhaseUI                             // screen-space overlay, submission order
)

// String returns the phase name (diagnostics / golden fixtures).
func (p RenderPhase) String() string {
	switch p {
	case PhaseNone:
		return "None"
	case PhaseOpaque:
		return "Opaque"
	case PhaseAlphaMask:
		return "AlphaMask"
	case PhaseTransparent:
		return "Transparent"
	case PhaseUI:
		return "UI"
	default:
		return "RenderPhase(?)"
	}
}
