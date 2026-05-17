// Package input provides device-agnostic input state resources for the ECS.
// ButtonInput[T] and AxisInput[T] track per-frame pressed/released state using
// map-based sets. All types live in the internal/ecs/input package.
package input

// ButtonInput tracks the pressed/released state of buttons.
// T must be comparable (e.g. KeyCode, MouseButton, GamepadButton).
// JustPressed and JustReleased are cleared each frame by Clear().
type ButtonInput[T comparable] struct {
	pressed      map[T]struct{}
	justPressed  map[T]struct{}
	justReleased map[T]struct{}
}

// NewButtonInput creates an empty ButtonInput with pre-allocated maps.
func NewButtonInput[T comparable]() *ButtonInput[T] {
	return &ButtonInput[T]{
		pressed:      make(map[T]struct{}, 8),
		justPressed:  make(map[T]struct{}, 8),
		justReleased: make(map[T]struct{}, 8),
	}
}

// Pressed reports whether button is currently held down.
func (b *ButtonInput[T]) Pressed(button T) bool {
	_, ok := b.pressed[button]
	return ok
}

// JustPressed reports whether button transitioned to pressed this frame.
// True for exactly one frame per press.
func (b *ButtonInput[T]) JustPressed(button T) bool {
	_, ok := b.justPressed[button]
	return ok
}

// JustReleased reports whether button transitioned to released this frame.
// True for exactly one frame per release.
func (b *ButtonInput[T]) JustReleased(button T) bool {
	_, ok := b.justReleased[button]
	return ok
}

// AnyPressed reports whether any button is currently held.
func (b *ButtonInput[T]) AnyPressed() bool { return len(b.pressed) > 0 }

// AnyJustPressed reports whether any button was just pressed this frame.
func (b *ButtonInput[T]) AnyJustPressed() bool { return len(b.justPressed) > 0 }

// GetPressed returns a slice of all currently pressed buttons.
func (b *ButtonInput[T]) GetPressed() []T {
	out := make([]T, 0, len(b.pressed))
	for k := range b.pressed {
		out = append(out, k)
	}
	return out
}

// GetJustPressed returns a slice of all buttons just pressed this frame.
func (b *ButtonInput[T]) GetJustPressed() []T {
	out := make([]T, 0, len(b.justPressed))
	for k := range b.justPressed {
		out = append(out, k)
	}
	return out
}

// GetJustReleased returns a slice of all buttons just released this frame.
func (b *ButtonInput[T]) GetJustReleased() []T {
	out := make([]T, 0, len(b.justReleased))
	for k := range b.justReleased {
		out = append(out, k)
	}
	return out
}

// Press records a button press. Adds to pressed and justPressed.
// If the button was already pressed, justPressed is not updated.
func (b *ButtonInput[T]) Press(button T) {
	if _, alreadyDown := b.pressed[button]; !alreadyDown {
		b.justPressed[button] = struct{}{}
	}
	b.pressed[button] = struct{}{}
}

// Release records a button release. Removes from pressed; adds to justReleased.
// If the button was not pressed, justReleased is not updated.
func (b *ButtonInput[T]) Release(button T) {
	if _, wasDown := b.pressed[button]; wasDown {
		b.justReleased[button] = struct{}{}
		delete(b.pressed, button)
	}
}

// Clear resets per-frame state (justPressed, justReleased) without touching
// the held-down set. Called by the engine at the start of each frame.
func (b *ButtonInput[T]) Clear() {
	clear(b.justPressed)
	clear(b.justReleased)
}

// Reset clears all state: pressed, justPressed, and justReleased.
func (b *ButtonInput[T]) Reset() {
	clear(b.pressed)
	clear(b.justPressed)
	clear(b.justReleased)
}
