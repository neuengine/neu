package animation

// RepeatMode controls what happens when a clip reaches its end.
type RepeatMode uint8

const (
	// RepeatOnce plays the clip once then transitions out.
	RepeatOnce RepeatMode = iota
	// RepeatLoop restarts the clip from the beginning.
	RepeatLoop
	// RepeatPingPong reverses direction at each end.
	RepeatPingPong
)

// AnimationTargetId identifies what an AnimationCurve animates:
// an entity (by path relative to the AnimationPlayer entity) and
// a component property (by reflection path).
//
// Both fields are resolved once at clip-load time into a cached write accessor;
// per-frame evaluation does not touch reflection (INV-1, C-027).
type AnimationTargetId struct {
	// EntityPath is a slash-separated path from the player entity, e.g. "Armature/Hips/Spine".
	// Empty means the player entity itself.
	EntityPath string
	// Property is a dot-separated reflection path, e.g. "Transform.Translation".
	Property string
}
