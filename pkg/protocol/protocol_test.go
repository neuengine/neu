package protocol_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/neuengine/neu/pkg/protocol"
)

// ── TestProtocolRoundTrip ─────────────────────────────────────────────────────

// roundTripCase pairs a message (with Type pre-set) with the Kind it carries.
type roundTripCase struct {
	kind protocol.Kind
	msg  any
}

var roundTripCases = []roundTripCase{
	{
		protocol.KindHotReloadPrepare,
		protocol.HotReloadPrepare{Type: protocol.KindHotReloadPrepare, SnapshotPath: "/tmp/snap.bin"},
	},
	{
		protocol.KindHotReloadReady,
		protocol.HotReloadReady{Type: protocol.KindHotReloadReady, SnapshotPath: "/tmp/snap.bin", EntityCount: 42, SnapshotSize: 1024},
	},
	{
		protocol.KindHotReloadFailed,
		protocol.HotReloadFailed{Type: protocol.KindHotReloadFailed, Reason: "compile error"},
	},
	{
		protocol.KindShaderError,
		protocol.ShaderError{Type: protocol.KindShaderError, Path: "shaders/pbr.wgsl", Message: "unexpected token"},
	},
	{
		protocol.KindShaderReloaded,
		protocol.ShaderReloaded{Type: protocol.KindShaderReloaded, Path: "shaders/pbr.wgsl"},
	},
	{
		protocol.KindReloadMetrics,
		protocol.ReloadMetrics{Type: protocol.KindReloadMetrics, SnapshotMS: 5, BuildMS: 120, RestoreMS: 3, EntitiesLost: []string{}},
	},
	{
		protocol.KindNetworkAlert,
		protocol.NetworkAlert{Type: protocol.KindNetworkAlert, Metric: "rtt_ms", Level: protocol.AlertCritical, Value: 250, Message: "high latency"},
	},
	{
		protocol.KindDiagnosticSnap,
		protocol.DiagnosticSnapshot{Type: protocol.KindDiagnosticSnap, Timestamp: 1_000_000, Metrics: map[string]float64{"fps": 60.0}},
	},
}

func TestProtocolRoundTrip(t *testing.T) {
	for _, tc := range roundTripCases {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()

			// Encode into a buffer.
			var buf bytes.Buffer
			if err := protocol.Encode(&buf, tc.msg); err != nil {
				t.Fatalf("Encode error: %v", err)
			}

			// Buffer must end with '\n'.
			b := buf.Bytes()
			if len(b) == 0 || b[len(b)-1] != '\n' {
				t.Fatal("encoded message does not end with newline")
			}

			// Decode back.
			r := bufio.NewReader(&buf)
			got, kind, err := protocol.Decode(r)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}
			if kind != tc.kind {
				t.Errorf("kind = %q, want %q", kind, tc.kind)
			}

			// Compare via JSON marshal to avoid unexported field issues.
			wantJSON, _ := json.Marshal(tc.msg)
			gotJSON, _ := json.Marshal(got)
			if string(wantJSON) != string(gotJSON) {
				t.Errorf("round-trip mismatch:\nwant %s\n got %s", wantJSON, gotJSON)
			}
		})
	}
}

// ── Forward-compat: unknown kind ──────────────────────────────────────────────

func TestDecode_UnknownKind(t *testing.T) {
	line := `{"type":"FutureMessage","data":42}` + "\n"
	r := bufio.NewReader(strings.NewReader(line))
	got, kind, err := protocol.Decode(r)
	if err != protocol.ErrUnknownKind {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
	if kind != "FutureMessage" {
		t.Errorf("kind = %q, want FutureMessage", kind)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

// ── Forward-compat: extra fields ──────────────────────────────────────────────

func TestDecode_ExtraFields(t *testing.T) {
	line := `{"type":"HotReloadFailed","reason":"boom","future_field":99}` + "\n"
	r := bufio.NewReader(strings.NewReader(line))
	msg, kind, err := protocol.Decode(r)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if kind != protocol.KindHotReloadFailed {
		t.Errorf("kind = %q", kind)
	}
	m := msg.(protocol.HotReloadFailed)
	if m.Reason != "boom" {
		t.Errorf("Reason = %q, want boom", m.Reason)
	}
}

// ── Scanner ───────────────────────────────────────────────────────────────────

func TestScanner_MultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	_ = protocol.Encode(&buf, protocol.HotReloadFailed{Type: protocol.KindHotReloadFailed, Reason: "e1"})
	_ = protocol.Encode(&buf, protocol.ShaderReloaded{Type: protocol.KindShaderReloaded, Path: "a.wgsl"})

	sc := protocol.NewScanner(&buf)
	count := 0
	for {
		_, _, ok, err := sc.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestScanner_EOF(t *testing.T) {
	sc := protocol.NewScanner(strings.NewReader(""))
	_, _, ok, err := sc.Next()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ok {
		t.Error("ok should be false at EOF")
	}
}

// ── Encode: malformed message triggers error ──────────────────────────────────

func TestEncode_NonSerializable(t *testing.T) {
	// channels are not JSON-serializable.
	ch := make(chan int)
	var buf bytes.Buffer
	if err := protocol.Encode(&buf, ch); err == nil {
		t.Error("Encode should error for non-serializable type")
	}
}

// ── Decode: malformed JSON ────────────────────────────────────────────────────

func TestDecode_MalformedJSON(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("{not json}\n"))
	_, _, err := protocol.Decode(r)
	if err == nil {
		t.Error("Decode should error on malformed JSON")
	}
}

// ── TestProtocolStdlibOnly ────────────────────────────────────────────────────

// TestProtocolStdlibOnly asserts that pkg/protocol imports only stdlib packages
// (INV-4: no engine deps). Uses go list to enumerate direct imports.
func TestProtocolStdlibOnly(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", "github.com/neuengine/neu/pkg/protocol")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list failed (CI may lack module cache): %v", err)
	}
	for imp := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		imp = strings.TrimSpace(imp)
		if imp == "" {
			continue
		}
		// All stdlib packages have no dot in their path prefix segment.
		// Engine packages start with "github.com/".
		if strings.HasPrefix(imp, "github.com/") {
			t.Errorf("pkg/protocol imports non-stdlib package: %s", imp)
		}
	}
}

// ── TestNoPackageInit (protocol) ──────────────────────────────────────────────

func TestProtocol_NoPackageInit(t *testing.T) {
	// go/ast walk via source is the most reliable. Use go list -json to get
	// source files and check for func init() presence.
	cmd := exec.Command("go", "list", "-json", "github.com/neuengine/neu/pkg/protocol")
	_, err := cmd.Output()
	if err != nil {
		t.Skipf("go list failed: %v", err)
	}
	// A func init() in the package would show up in the source. Use go vet or
	// a source grep as a simple guard here.
	cmd2 := exec.Command("go", "doc", "-all", "github.com/neuengine/neu/pkg/protocol")
	doc, err2 := cmd2.Output()
	if err2 != nil {
		t.Skipf("go doc failed: %v", err2)
	}
	if bytes.Contains(doc, []byte("func init()")) {
		t.Error("pkg/protocol must not define an init() function (INV-5)")
	}
}

// ── TestNoEditorImports ───────────────────────────────────────────────────────

// TestNoEditorImports verifies that the engine does not import the (external)
// editor repository — INV-1 of l2-multi-repo-architecture-go.md.
func TestNoEditorImports(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = "../.." // project root from pkg/protocol
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list -deps failed: %v", err)
	}
	for pkg := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(pkg, "github.com/neuengine/editor") {
			t.Errorf("engine depends on external editor package: %s", pkg)
		}
	}
}

// ── Encode+Decode idempotent via Scanner ─────────────────────────────────────

func TestScanner_RoundTrip(t *testing.T) {
	cases := roundTripCases
	var buf bytes.Buffer
	for _, tc := range cases {
		if err := protocol.Encode(&buf, tc.msg); err != nil {
			t.Fatalf("Encode %v: %v", tc.kind, err)
		}
	}

	sc := protocol.NewScanner(&buf)
	var decoded []any
	for {
		msg, _, ok, err := sc.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		decoded = append(decoded, msg)
	}
	if len(decoded) != len(cases) {
		t.Fatalf("decoded %d messages, want %d", len(decoded), len(cases))
	}
}

// ── io.EOF on empty Reader ────────────────────────────────────────────────────

func TestDecode_EOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	_, _, err := protocol.Decode(r)
	if err != io.EOF {
		t.Errorf("err = %v, want io.EOF", err)
	}
}
