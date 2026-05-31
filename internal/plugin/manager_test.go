package plugin

import (
	"errors"
	"testing"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

func validManifest() pkgplugin.Manifest {
	return pkgplugin.Manifest{
		ID:            "com.example.test",
		Version:       "1.0.0",
		EngineVersion: "^1.0.0",
		Mode:          pkgplugin.ModeInProcess,
		Entry:         pkgplugin.EntrySpec{PackagePath: "github.com/example/test", Factory: "New"},
		RequiredCaps:  []pkgplugin.Capability{pkgplugin.CapNetworkOutbound},
	}
}

func engineV(s string) pkgplugin.Version { v, _ := pkgplugin.ParseVersion(s); return v }

func TestManagerRegister(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.4.0"))
	man := validManifest()
	granted := pkgplugin.NewCapabilitySet(pkgplugin.CapNetworkOutbound)

	if err := m.Register(man, granted); err != nil {
		t.Fatalf("Register valid: %v", err)
	}
	if st, ok := m.State(man.ID); !ok || st != pkgplugin.StateApproved {
		t.Errorf("state = %v,%v want Approved", st, ok)
	}
	if m.Count() != 1 {
		t.Errorf("count = %d, want 1", m.Count())
	}

	// INV-5: duplicate ID rejected.
	if err := m.Register(man, granted); !errors.Is(err, pkgplugin.ErrDuplicateID) {
		t.Errorf("duplicate err = %v, want ErrDuplicateID", err)
	}
}

func TestManagerEngineIncompatible(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("2.0.0")) // engine v2 vs manifest ^1.0.0
	if err := m.Register(validManifest(), nil); !errors.Is(err, pkgplugin.ErrEngineIncompatible) {
		t.Errorf("incompatible err = %v, want ErrEngineIncompatible (INV-2)", err)
	}
	if m.Count() != 0 {
		t.Error("incompatible plugin must not be registered")
	}
}

func TestManagerInvalidManifest(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.0.0"))
	bad := validManifest()
	bad.ID = "" // INV-1 violation
	if err := m.Register(bad, nil); !errors.Is(err, pkgplugin.ErrManifestInvalid) {
		t.Errorf("invalid manifest err = %v, want ErrManifestInvalid (INV-1)", err)
	}
}

func TestCapabilityProxyEnforcesINV3(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.0.0"))
	man := validManifest()
	granted := pkgplugin.NewCapabilitySet(pkgplugin.CapNetworkOutbound)
	if err := m.Register(man, granted); err != nil {
		t.Fatalf("Register: %v", err)
	}
	ctx, ok := m.Context(man.ID, nil)
	if !ok {
		t.Fatal("Context not found for registered plugin")
	}
	// Granted capability passes.
	if err := ctx.Check(pkgplugin.CapNetworkOutbound); err != nil {
		t.Errorf("granted cap Check = %v, want nil", err)
	}
	// Ungranted capability is denied with a CapabilityError (INV-3).
	err := ctx.Check(pkgplugin.CapFSWriteUser)
	if !errors.Is(err, pkgplugin.ErrCapabilityDenied) {
		t.Errorf("ungranted Check = %v, want ErrCapabilityDenied", err)
	}
	var ce pkgplugin.CapabilityError
	if !errors.As(err, &ce) || ce.Plugin != man.ID || ce.Capability != pkgplugin.CapFSWriteUser {
		t.Errorf("CapabilityError detail = %+v", ce)
	}
	// Commands are tagged with the plugin ID (INV-7).
	if ctx.Commands().PluginID() != man.ID {
		t.Error("commands must be tagged with the plugin ID")
	}
	if ctx.Config() != nil {
		t.Error("default config should be nil")
	}
	if !ctx.Capabilities().Has(pkgplugin.CapNetworkOutbound) {
		t.Error("Capabilities should reflect the granted set")
	}
}

func TestManagerSetStateAndUnknownContext(t *testing.T) {
	t.Parallel()
	m := NewManager(engineV("1.0.0"))
	man := validManifest()
	if err := m.Register(man, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	m.SetState(man.ID, pkgplugin.StateActive)
	if st, _ := m.State(man.ID); st != pkgplugin.StateActive {
		t.Errorf("state after SetState = %v, want Active", st)
	}
	if _, ok := m.State("does.not.exist"); ok {
		t.Error("unknown plugin State should be false")
	}
	if _, ok := m.Context("does.not.exist", nil); ok {
		t.Error("unknown plugin Context should be false")
	}
}
