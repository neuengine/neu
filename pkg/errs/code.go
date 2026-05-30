package errs

import (
	"errors"
	"fmt"
	"sync"
)

// Code is an E-series error code in the form "E[Category][ID]" (L1 §3.1).
// It is a string newtype so codes are human-readable in logs and stable across
// versions; a future codegen pass may emit a constant block without changing
// the type.
type Code string

// Category ranges (L1 §3.1). A module's codes must fall inside its registered
// range; the numeric part is the four digits after the leading 'E'.
const (
	CategoryCoreMin  uint16 = 0    // E0000–E0999 Core ECS (Entity/Component/World)
	CategoryCoreMax  uint16 = 999  //
	CategorySchedMin uint16 = 1000 // E1000–E1999 Scheduling & Systems
	CategorySchedMax uint16 = 1999 //
	CategoryRenderMin uint16 = 2000 // E2000–E2999 Render & Assets
	CategoryRenderMax uint16 = 2999 //
	CategoryPhysMin  uint16 = 3000 // E3000–E3999 Physics & Collision
	CategoryPhysMax  uint16 = 3999 //
)

// Descriptor is the registered metadata for one error code.
type Descriptor struct {
	Code     Code
	Severity Severity
	Audience Audience
	Module   string // "ecs", "render", "physics"
	Template string // default-locale message template (fmt-style verbs)
	Solution string // actionable developer advice
}

// ErrDuplicateCode is returned when a code is registered twice.
type ErrDuplicateCode struct{ Code Code }

func (e ErrDuplicateCode) Error() string {
	return fmt.Sprintf("errs: duplicate registration of code %q", string(e.Code))
}

// ErrCodeOutOfRange is returned when a code falls outside its module's
// declared numeric range.
type ErrCodeOutOfRange struct {
	Code   Code
	Module string
}

func (e ErrCodeOutOfRange) Error() string {
	return fmt.Sprintf("errs: code %q is outside the range registered for module %q",
		string(e.Code), e.Module)
}

// ErrMalformedCode is returned when a code does not match the E#### grammar.
var ErrMalformedCode = errors.New("errs: code must match the form E followed by digits")

var (
	registry     = make(map[Code]Descriptor)
	moduleRanges = make(map[string][2]uint16)
	registryMu   sync.RWMutex
)

// RegisterModuleRange declares the inclusive numeric code range owned by a
// module. Subsequent Register calls for that module enforce the range (INV
// out-of-range). Calling it twice for the same module overwrites the range.
func RegisterModuleRange(module string, min, max uint16) {
	registryMu.Lock()
	defer registryMu.Unlock()
	moduleRanges[module] = [2]uint16{min, max}
}

// codeNum parses the numeric part of a code ("E0042" → 42).
func codeNum(c Code) (uint16, bool) {
	s := string(c)
	if len(s) < 2 || (s[0] != 'E' && s[0] != 'e') {
		return 0, false
	}
	var n uint32
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + uint32(r-'0')
		if n > 9999 {
			return 0, false
		}
	}
	return uint16(n), true
}

// Register records a descriptor. It fails fast on a duplicate code, a malformed
// code, or a code outside its module's declared range (when one is registered).
// Modules call this from init().
func Register(d Descriptor) error {
	num, ok := codeNum(d.Code)
	if !ok {
		return fmt.Errorf("%w: %q", ErrMalformedCode, string(d.Code))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[d.Code]; dup {
		return ErrDuplicateCode{Code: d.Code}
	}
	if r, has := moduleRanges[d.Module]; has {
		if num < r[0] || num > r[1] {
			return ErrCodeOutOfRange{Code: d.Code, Module: d.Module}
		}
	}
	registry[d.Code] = d
	return nil
}

// Lookup returns the descriptor for a code, if registered.
func Lookup(c Code) (Descriptor, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	d, ok := registry[c]
	return d, ok
}

// MustRegister registers a descriptor and panics on failure. Intended only for
// package init() where a registration error is a developer bug.
func MustRegister(d Descriptor) {
	if err := Register(d); err != nil {
		panic(err)
	}
}
