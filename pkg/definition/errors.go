package definition

import "fmt"

// ErrUnknownType is returned when a definition references a type name that is
// not registered (INV-4: every value maps to a registered type or primitive).
type ErrUnknownType struct{ Name string }

func (e ErrUnknownType) Error() string {
	return fmt.Sprintf("definition: unknown type %q (not registered)", e.Name)
}

// ErrDefinitionCycle is returned when include references form a cycle (INV-5).
type ErrDefinitionCycle struct{ Path string }

func (e ErrDefinitionCycle) Error() string {
	return fmt.Sprintf("definition: circular include detected at %q", e.Path)
}

// ErrSchemaInvalid is returned for a structurally malformed definition (bad
// JSON, missing/duplicate root kind). Err wraps the underlying cause if any.
type ErrSchemaInvalid struct {
	Err    error
	Reason string
}

func (e ErrSchemaInvalid) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("definition: invalid schema: %s: %v", e.Reason, e.Err)
	}
	return fmt.Sprintf("definition: invalid schema: %s", e.Reason)
}

func (e ErrSchemaInvalid) Unwrap() error { return e.Err }

// ErrUnknownAction is returned when an action type is not registered (INV: every
// action resolves to a handler).
type ErrUnknownAction struct{ Type string }

func (e ErrUnknownAction) Error() string {
	return fmt.Sprintf("definition: unknown action %q", e.Type)
}
