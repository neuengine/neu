package component

import (
	"reflect"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/entity"
)

func TestHooksZeroValueIsNoOp(t *testing.T) {
	t.Parallel()

	var h Hooks
	if h.Any() {
		t.Fatal("zero-value Hooks must report Any() == false")
	}
}

func TestHooksAnyReportsPresence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		hook func() Hooks
	}{
		{"only_add", func() Hooks { return Hooks{OnAdd: func(HookContext, entity.Entity) {}} }},
		{"only_insert", func() Hooks { return Hooks{OnInsert: func(HookContext, entity.Entity) {}} }},
		{"only_replace", func() Hooks { return Hooks{OnReplace: func(HookContext, entity.Entity) {}} }},
		{"only_remove", func() Hooks { return Hooks{OnRemove: func(HookContext, entity.Entity) {}} }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !tc.hook().Any() {
				t.Fatal("Any() must return true when at least one hook is set")
			}
		})
	}
}

func TestHooksAttachToInfo(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	called := 0
	id := r.Register(Info{
		Type: reflect.TypeFor[hookProbe](),
		Hooks: Hooks{
			OnAdd: func(_ HookContext, _ entity.Entity) { called++ },
		},
	})
	info := r.Info(id)
	info.Hooks.OnAdd(nil, entity.NewEntity(1, 1))
	if called != 1 {
		t.Fatalf("OnAdd must fire when invoked; called=%d", called)
	}
}

type hookProbe struct{ X int }
