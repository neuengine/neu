package errs

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSeverityStringTotal(t *testing.T) {
	t.Parallel()
	cases := map[Severity]string{
		SeverityDebug:       "Debug",
		SeverityWarning:     "Warning",
		SeverityRecoverable: "Recoverable",
		SeverityFatal:       "Fatal",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", sev, got, want)
		}
	}
	if got := Severity(200).String(); got != "Severity(?)" {
		t.Errorf("unknown severity = %q, want sentinel", got)
	}
}

func TestAudienceStringTotal(t *testing.T) {
	t.Parallel()
	cases := map[Audience]string{
		AudienceDeveloper: "Developer",
		AudienceUser:      "User",
		AudienceSystem:    "System",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("Audience(%d).String() = %q, want %q", a, got, want)
		}
	}
	if got := Audience(200).String(); got != "Audience(?)" {
		t.Errorf("unknown audience = %q, want sentinel", got)
	}
}

func TestRegisterAndLookup(t *testing.T) {
	// Not parallel: mutates the package-global registry.
	const code Code = "E0500"
	d := Descriptor{Code: code, Severity: SeverityRecoverable, Audience: AudienceDeveloper, Module: "test", Template: "boom %d"}
	if err := Register(d); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, code)
		registryMu.Unlock()
	})
	got, ok := Lookup(code)
	if !ok || got.Module != "test" {
		t.Fatalf("Lookup(%q) = %+v, %v", code, got, ok)
	}
	// Duplicate.
	if err := Register(d); !errors.As(err, new(ErrDuplicateCode)) {
		t.Errorf("duplicate Register err = %v, want ErrDuplicateCode", err)
	}
}

func TestRegisterMalformedCode(t *testing.T) {
	t.Parallel()
	for _, bad := range []Code{"X0001", "0001", "Eabcd", "E"} {
		if err := Register(Descriptor{Code: bad, Module: "test"}); !errors.Is(err, ErrMalformedCode) {
			t.Errorf("Register(%q) err = %v, want ErrMalformedCode", bad, err)
		}
	}
}

func TestRegisterOutOfRange(t *testing.T) {
	// Not parallel: mutates module-range + registry globals.
	RegisterModuleRange("ranged", CategorySchedMin, CategorySchedMax) // E1000–E1999
	t.Cleanup(func() {
		registryMu.Lock()
		delete(moduleRanges, "ranged")
		delete(registry, "E1500")
		registryMu.Unlock()
	})
	if err := Register(Descriptor{Code: "E0042", Module: "ranged"}); !errors.As(err, new(ErrCodeOutOfRange)) {
		t.Errorf("out-of-range Register err = %v, want ErrCodeOutOfRange", err)
	}
	if err := Register(Descriptor{Code: "E1500", Module: "ranged"}); err != nil {
		t.Errorf("in-range Register err = %v, want nil", err)
	}
}

func TestEngineErrorInterfaceAndChain(t *testing.T) {
	// Not parallel: registers a code.
	const code Code = "E0600"
	MustRegister(Descriptor{Code: code, Severity: SeverityRecoverable, Audience: AudienceUser, Module: "chain", Template: "wrapped thing"})
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, code)
		registryMu.Unlock()
	})

	cause := errors.New("root cause")
	err := Wrap(code, cause)

	// Implements EngineError + error.
	var ee EngineError
	if !errors.As(err, &ee) {
		t.Fatal("errors.As did not recover EngineError")
	}
	if ee.Code() != code || ee.Severity() != SeverityRecoverable || ee.Module() != "chain" {
		t.Errorf("accessors = %q/%v/%q", ee.Code(), ee.Severity(), ee.Module())
	}
	// Unwrap reaches the cause.
	if !errors.Is(err, cause) {
		t.Error("errors.Is did not find wrapped cause")
	}
	// Is matches by code through a fmt wrap.
	wrapped := fmt.Errorf("context: %w", err)
	if !errors.Is(wrapped, New(code)) {
		t.Error("errors.Is by code failed through fmt.Errorf chain")
	}
	if !strings.Contains(err.Error(), "root cause") {
		t.Errorf("Error() = %q, want it to include the cause", err.Error())
	}
}

func TestCatalogLocalizeAndFallback(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"errors.en.json": &fstest.MapFile{Data: []byte(`{"E1000":{"template":"system %q cyclic","solution":"break it"}}`)},
	}
	c := NewCatalog()
	if err := c.Load(fsys, "en"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Lang() != "en" {
		t.Errorf("Lang = %q", c.Lang())
	}
	if got := c.Localize("E1000", "Movement"); got != `system "Movement" cyclic` {
		t.Errorf("Localize = %q", got)
	}
	// Missing key, no descriptor → bare code.
	if got := c.Localize("E9999"); got != "E9999" {
		t.Errorf("fallback = %q, want bare code", got)
	}
}

func TestCatalogMissingFile(t *testing.T) {
	t.Parallel()
	c := NewCatalog()
	if err := c.Load(fstest.MapFS{}, "en"); err == nil {
		t.Error("Load of missing file should return an error (caller treats as Warning)")
	}
	// Still usable: falls back to bare code.
	if got := c.Localize("E0001"); got != "E0001" {
		t.Errorf("post-failure Localize = %q", got)
	}
}

func TestMustSucceedPanicPolicy(t *testing.T) {
	// Not parallel: registers codes.
	const fatalDev Code = "E0700"
	const fatalUser Code = "E0701"
	const recovDev Code = "E0702"
	MustRegister(Descriptor{Code: fatalDev, Severity: SeverityFatal, Audience: AudienceDeveloper, Module: "panic"})
	MustRegister(Descriptor{Code: fatalUser, Severity: SeverityFatal, Audience: AudienceUser, Module: "panic"})
	MustRegister(Descriptor{Code: recovDev, Severity: SeverityRecoverable, Audience: AudienceDeveloper, Module: "panic"})
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, fatalDev)
		delete(registry, fatalUser)
		delete(registry, recovDev)
		registryMu.Unlock()
	})

	assertPanics(t, "fatal+developer", func() { _ = MustSucceed(New(fatalDev)) })
	assertNoPanic(t, "fatal+user", func() { _ = MustSucceed(New(fatalUser)) })
	assertNoPanic(t, "recoverable+developer", func() { _ = MustSucceed(New(recovDev)) })
	assertNoPanic(t, "nil", func() { _ = MustSucceed(nil) })
	assertNoPanic(t, "plain error", func() { _ = MustSucceed(errors.New("plain")) })
}

func TestRedactor(t *testing.T) {
	t.Parallel()
	r := NewRedactor("")
	r.AddSecret("sk-12345")
	r.AddSecret("") // ignored — must not blanket-redact
	if got := r.Redact("key=sk-12345 done"); got != "key=*** done" {
		t.Errorf("Redact = %q", got)
	}
	var sb strings.Builder
	w := r.Writer(&sb)
	n, err := w.Write([]byte("token sk-12345!"))
	if err != nil || n != len("token sk-12345!") {
		t.Fatalf("Write n=%d err=%v", n, err)
	}
	if sb.String() != "token ***!" {
		t.Errorf("redacted writer = %q", sb.String())
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func assertNoPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: unexpected panic %v", name, r)
		}
	}()
	fn()
}
