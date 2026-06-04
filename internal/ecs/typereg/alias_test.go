package typereg

import (
	"errors"
	"reflect"
	"testing"
)

type oldHealth struct{ HP int }
type newHealth struct{ HP int }

func TestRegisterAliasResolves(t *testing.T) {
	t.Parallel()
	r := NewTypeRegistry()
	reg := RegisterType[newHealth](r)
	canonical := reg.Name

	// Alias a legacy name onto the registered type (the rename migration case).
	if err := r.RegisterAlias("game.PlayerHealth", reflect.TypeFor[newHealth]()); err != nil {
		t.Fatalf("RegisterAlias: %v", err)
	}
	// Both the canonical name and the alias resolve to the same registration.
	if got := r.ResolveByName("game.PlayerHealth"); got == nil || got.Type != reg.Type {
		t.Errorf("alias did not resolve to the registered type: %v", got)
	}
	if got := r.ResolveByName(canonical); got == nil || got.Type != reg.Type {
		t.Errorf("canonical name no longer resolves after aliasing: %v", got)
	}
	// Re-aliasing the same name to the same type is idempotent.
	if err := r.RegisterAlias("game.PlayerHealth", reflect.TypeFor[newHealth]()); err != nil {
		t.Errorf("idempotent re-alias errored: %v", err)
	}
}

func TestRegisterAliasUnregisteredType(t *testing.T) {
	t.Parallel()
	r := NewTypeRegistry()
	err := r.RegisterAlias("anything", reflect.TypeFor[newHealth]())
	if !errors.Is(err, ErrTypeNotRegistered) {
		t.Errorf("RegisterAlias on unregistered type = %v, want ErrTypeNotRegistered", err)
	}
}

func TestRegisterAliasCollision(t *testing.T) {
	t.Parallel()
	r := NewTypeRegistry()
	RegisterType[oldHealth](r)
	newReg := RegisterType[newHealth](r)

	// Aliasing oldHealth's canonical name onto newHealth must be rejected.
	err := r.RegisterAlias(typeName(reflect.TypeFor[oldHealth]()), newReg.Type)
	if !errors.Is(err, ErrDuplicateTypeName) {
		t.Errorf("colliding alias = %v, want ErrDuplicateTypeName", err)
	}
}
