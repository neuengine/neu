// Package tween provides the internal tween execution system: per-frame advance,
// reflection-based write-back, and lifecycle management.
package tween

import (
	"errors"
	"fmt"
	"reflect"

	pkgtween "github.com/neuengine/neu/pkg/tween"
)

// ErrTweenTypeMismatch is surfaced when StartValue and EndValue types differ
// or are not supported by LerpAny.
var ErrTweenTypeMismatch = pkgtween.ErrTweenTypeMismatch

// writeAccessor is a cached reflection path for applying interpolated values.
// Resolved once on tween insertion; per-frame apply is allocation-free (C-027).
type writeAccessor struct {
	// path is the dot-separated field path, e.g. "Translation".
	path []string
}

// resolveField traverses path from root, returning the innermost settable Value.
// Returns the zero Value if the path cannot be resolved.
func resolveField(root reflect.Value, path []string) (reflect.Value, bool) {
	v := root
	for _, seg := range path {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		f := v.FieldByName(seg)
		if !f.IsValid() {
			return reflect.Value{}, false
		}
		v = f
	}
	return v, v.CanSet()
}

// Apply writes val to the target described by acc within target.
// target must be a pointer to the component struct.
// Returns false if the path cannot be resolved (INV-4 silent skip).
func (a *writeAccessor) Apply(target any, val any) bool {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return false
	}
	field, ok := resolveField(rv.Elem(), a.path)
	if !ok {
		return false
	}
	rval := reflect.ValueOf(val)
	if !rval.Type().AssignableTo(field.Type()) {
		return false
	}
	field.Set(rval)
	return true
}

// NewWriteAccessor parses a dot-separated path into a writeAccessor.
// Returns an error if the path is empty.
func NewWriteAccessor(dotPath string) (*writeAccessor, error) {
	if dotPath == "" {
		return nil, errors.New("tween: empty target field path")
	}
	parts := splitPath(dotPath)
	return &writeAccessor{path: parts}, nil
}

// splitPath splits a dot-separated path, e.g. "Transform.Translation" → ["Transform", "Translation"].
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := range len(path) {
		if path[i] == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// AdvanceTween advances a single tween by dt seconds and applies the interpolated
// value to target via reflection. Returns:
//   - done=true when the tween completes (LoopOnce only; Loop/PingPong never return true)
//   - err != nil on type mismatch (tween should be removed by caller)
func AdvanceTween(tw *pkgtween.Tween, acc *writeAccessor, target any, dt float64) (done bool, err error) {
	if tw.Duration <= 0 {
		// Degenerate: apply end value immediately.
		val, ok := pkgtween.LerpAny(tw.StartValue, tw.EndValue, 1)
		if !ok {
			return true, fmt.Errorf("%w: degenerate tween", pkgtween.ErrTweenTypeMismatch)
		}
		acc.Apply(target, val)
		return true, nil
	}

	tw.Elapsed += dt
	t := tw.Elapsed / tw.Duration

	switch tw.LoopMode {
	case pkgtween.Loop:
		// Wrap t to [0,1).
		if t >= 1 {
			tw.Elapsed = tw.Elapsed - float64(int(tw.Elapsed/tw.Duration))*tw.Duration
			t = tw.Elapsed / tw.Duration
		}
	case pkgtween.PingPong:
		// Bounce t off 0 and 1.
		tFloor := int(t)
		t = t - float64(tFloor)
		if tFloor%2 == 1 {
			t = 1 - t
		}
	default: // LoopOnce
		if t >= 1 {
			t = 1
		}
	}

	// Apply easing.
	eased := t
	if tw.Easing != nil {
		eased = tw.Easing(t)
	}

	val, ok := pkgtween.LerpAny(tw.StartValue, tw.EndValue, float32(eased))
	if !ok {
		return true, fmt.Errorf("%w: StartValue type mismatch EndValue", pkgtween.ErrTweenTypeMismatch)
	}
	acc.Apply(target, val)

	// Signal done only for LoopOnce when elapsed ≥ duration.
	if tw.LoopMode == pkgtween.LoopOnce && tw.Elapsed >= tw.Duration {
		return true, nil
	}
	return false, nil
}
