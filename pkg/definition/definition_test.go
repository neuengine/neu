package definition

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// stubTypes is a TypeResolver backed by a known-name set.
type stubTypes struct{ known map[string]bool }

func types(names ...string) stubTypes {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return stubTypes{known: m}
}

func (s stubTypes) ResolveType(name string) bool { return s.known[name] }

// recordingSink captures interpreter output, proving Instantiate emits only
// commands (INV-2: the sink has no world-mutation surface).
type recordingSink struct {
	spawns  []string
	inserts []string
	parents []string
	actions []string
	next    EntityRef
}

func (s *recordingSink) SpawnEntity(name string) EntityRef {
	r := s.next
	s.next++
	s.spawns = append(s.spawns, name)
	return r
}
func (s *recordingSink) InsertComponent(e EntityRef, typeName string, _ map[string]any) {
	s.inserts = append(s.inserts, fmt.Sprintf("%d:%s", e, typeName))
}
func (s *recordingSink) SetParent(child, parent EntityRef) {
	s.parents = append(s.parents, fmt.Sprintf("%d<-%d", child, parent))
}
func (s *recordingSink) RunAction(a Action) { s.actions = append(s.actions, a.Action) }

const uiJSON = `{
  "definition":"ui","version":"1.0",
  "content":{"root":{"type":"Node","style":{"width":"100%"},"children":[
    {"type":"Text","value":"Hi"},
    {"type":"Button","id":"play","on_click":{"action":"transition","target":"game"}}
  ]}}}`

func TestDecodeUIAndInstantiate(t *testing.T) {
	t.Parallel()
	def, err := Decode([]byte(uiJSON), types("Node", "Text", "Button"), NewActionRegistry())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if def.Kind != KindUI {
		t.Fatalf("kind = %v, want ui", def.Kind)
	}

	// INV-2: Instantiate emits only commands. INV-1: no error path.
	var sink recordingSink
	Instantiate(&def, &sink)
	// 3 nodes: Node + Text + Button.
	if len(sink.spawns) != 3 {
		t.Errorf("spawns = %d, want 3", len(sink.spawns))
	}
	// Each node gets its type component; the Text node also gets a Text value.
	if !contains(sink.inserts, "0:Node") || !contains(sink.inserts, "1:Text") || !contains(sink.inserts, "2:Button") {
		t.Errorf("inserts missing node types: %v", sink.inserts)
	}
	// Children are parented to the root (entity 0).
	if len(sink.parents) != 2 {
		t.Errorf("parents = %v, want 2 (children of root)", sink.parents)
	}
}

func TestDecodeUnknownRootKind(t *testing.T) {
	t.Parallel()
	_, err := Decode([]byte(`{"definition":"widget","content":{}}`), types(), NewActionRegistry())
	if !errors.As(err, new(ErrSchemaInvalid)) {
		t.Errorf("unknown kind err = %v, want ErrSchemaInvalid", err)
	}
}

func TestDecodeMalformedJSON(t *testing.T) {
	t.Parallel()
	_, err := Decode([]byte(`{not json`), types(), NewActionRegistry())
	if !errors.As(err, new(ErrSchemaInvalid)) {
		t.Errorf("malformed err = %v, want ErrSchemaInvalid", err)
	}
}

func TestValidateUnknownType(t *testing.T) {
	t.Parallel()
	// Node known, but Button is not registered → INV-4 failure.
	_, err := Decode([]byte(uiJSON), types("Node", "Text"), NewActionRegistry())
	var ut ErrUnknownType
	if !errors.As(err, &ut) || ut.Name != "Button" {
		t.Errorf("err = %v, want ErrUnknownType{Button}", err)
	}
}

func TestValidateUnknownAction(t *testing.T) {
	t.Parallel()
	reg := NewActionRegistry() // "transition" is built-in; remove by using a fresh empty-ish check
	js := `{"definition":"ui","content":{"root":{"type":"Node","on_click":{"action":"frobnicate"}}}}`
	_, err := Decode([]byte(js), types("Node"), reg)
	var ua ErrUnknownAction
	if !errors.As(err, &ua) || ua.Type != "frobnicate" {
		t.Errorf("err = %v, want ErrUnknownAction{frobnicate}", err)
	}
}

func TestSceneDecodeInstantiate(t *testing.T) {
	t.Parallel()
	js := `{"definition":"scene","content":{"entities":[
		{"name":"player","components":{"Transform":{"x":0},"Health":{"max":100}},
		 "children":[{"name":"weapon","components":{"Transform":{}}}]}]}}`
	def, err := Decode([]byte(js), types("Transform", "Health"), NewActionRegistry())
	if err != nil {
		t.Fatalf("Decode scene: %v", err)
	}
	var sink recordingSink
	Instantiate(&def, &sink)
	if len(sink.spawns) != 2 { // player + weapon
		t.Errorf("scene spawns = %d, want 2", len(sink.spawns))
	}
	if !contains(sink.inserts, "0:Health") || !contains(sink.inserts, "0:Transform") {
		t.Errorf("player components missing: %v", sink.inserts)
	}
	if len(sink.parents) != 1 { // weapon child of player
		t.Errorf("scene parents = %v, want 1", sink.parents)
	}
}

func TestFlowValidation(t *testing.T) {
	t.Parallel()
	valid := `{"definition":"flow","content":{"initial_state":"menu","states":{
		"menu":{"transitions":[{"event":"play","target":"game"}]},
		"game":{"on_enter":[{"action":"log","message":"start"}]}}}}`
	def, err := Decode([]byte(valid), types(), NewActionRegistry())
	if err != nil {
		t.Fatalf("valid flow: %v", err)
	}
	var sink recordingSink
	Instantiate(&def, &sink) // initial state "menu" has no on_enter → no actions
	if len(sink.actions) != 0 {
		t.Errorf("menu on_enter actions = %v, want none", sink.actions)
	}

	// Missing initial_state target.
	bad := `{"definition":"flow","content":{"initial_state":"nope","states":{"menu":{}}}}`
	if _, err := Decode([]byte(bad), types(), NewActionRegistry()); !errors.As(err, new(ErrSchemaInvalid)) {
		t.Errorf("bad initial_state err = %v, want ErrSchemaInvalid", err)
	}

	// Transition to unknown state.
	badT := `{"definition":"flow","content":{"initial_state":"a","states":{"a":{"transitions":[{"event":"x","target":"ghost"}]}}}}`
	if _, err := Decode([]byte(badT), types(), NewActionRegistry()); !errors.As(err, new(ErrSchemaInvalid)) {
		t.Errorf("bad transition target err = %v, want ErrSchemaInvalid", err)
	}
}

func TestCheckIncludeDAG(t *testing.T) {
	t.Parallel()
	// Diamond: a→b, a→c, b→d, c→d — acyclic.
	acyclic := map[string][]string{
		"a.json": {"b.json", "c.json"},
		"b.json": {"d.json"},
		"c.json": {"d.json"},
		"d.json": {},
	}
	if err := CheckIncludeDAG(acyclic); err != nil {
		t.Errorf("diamond DAG should be acyclic, got %v", err)
	}

	// Cycle: a→b→a.
	cyclic := map[string][]string{
		"a.json": {"b.json"},
		"b.json": {"a.json"},
	}
	var cyc ErrDefinitionCycle
	if err := CheckIncludeDAG(cyclic); !errors.As(err, &cyc) {
		t.Errorf("a→b→a should cycle, got %v", err)
	}
}

func TestKindRoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"ui", "scene", "flow", "template"} {
		k, ok := ParseKind(s)
		if !ok || k.String() != s {
			t.Errorf("ParseKind(%q) round-trip failed: %v %v", s, k, ok)
		}
	}
	if _, ok := ParseKind("bogus"); ok {
		t.Error("bogus kind should not parse")
	}
	if KindUnknown.String() != "unknown" {
		t.Error("KindUnknown.String")
	}
}

func TestActionUnmarshal(t *testing.T) {
	t.Parallel()
	var a Action
	if err := a.UnmarshalJSON([]byte(`{"action":"play_audio","source":"a.wav","spatial":true}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if a.Action != "play_audio" || a.Params["source"] != "a.wav" || a.Params["spatial"] != true {
		t.Errorf("action = %+v", a)
	}
}

func TestInstantiateInfallibleProperty(t *testing.T) {
	t.Parallel()
	// INV-1: every definition that validates instantiates without panic.
	defs := []struct {
		js    string
		types stubTypes
	}{
		{uiJSON, types("Node", "Text", "Button")},
		{`{"definition":"template","content":{"name":"goblin","components":{"Health":{"max":30}}}}`, types("Health")},
		{`{"definition":"scene","content":{"entities":[{"name":"e","components":{"T":{}}}]}}`, types("T")},
	}
	for i, d := range defs {
		def, err := Decode([]byte(d.js), d.types, NewActionRegistry())
		if err != nil {
			t.Fatalf("def %d Decode: %v", i, err)
		}
		var sink recordingSink
		Instantiate(&def, &sink) // must not panic / error
	}
}

func TestLoadFromReader(t *testing.T) {
	t.Parallel()
	def, err := Load(strings.NewReader(uiJSON), types("Node", "Text", "Button"), NewActionRegistry())
	if err != nil || def.Kind != KindUI {
		t.Fatalf("Load: def=%v err=%v", def.Kind, err)
	}
}

func TestActionRegistryRegister(t *testing.T) {
	t.Parallel()
	r := NewActionRegistry()
	if r.Has("frobnicate") {
		t.Error("custom action should be unknown before Register")
	}
	r.Register("frobnicate")
	if !r.Has("frobnicate") || !r.Has("transition") {
		t.Error("Register should add custom action and keep built-ins")
	}
}

func TestErrorMessages(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want string
	}{
		{ErrUnknownType{Name: "Foo"}, "Foo"},
		{ErrDefinitionCycle{Path: "a.json"}, "a.json"},
		{ErrSchemaInvalid{Reason: "bad", Err: errors.New("x")}, "bad"},
		{ErrSchemaInvalid{Reason: "bad"}, "bad"},
		{ErrUnknownAction{Type: "zap"}, "zap"},
	}
	for _, c := range cases {
		if !strings.Contains(c.err.Error(), c.want) {
			t.Errorf("%T.Error() = %q, want it to contain %q", c.err, c.err.Error(), c.want)
		}
	}
	// ErrSchemaInvalid wraps its cause.
	cause := errors.New("root")
	if !errors.Is(ErrSchemaInvalid{Reason: "r", Err: cause}, cause) {
		t.Error("ErrSchemaInvalid should unwrap to its cause")
	}
}

func TestDecodeMalformedSubContent(t *testing.T) {
	t.Parallel()
	// Valid envelope, but content is the wrong shape for a scene.
	js := `{"definition":"scene","content":{"entities":"not-an-array"}}`
	if _, err := Decode([]byte(js), types(), NewActionRegistry()); !errors.As(err, new(ErrSchemaInvalid)) {
		t.Errorf("malformed scene content err = %v, want ErrSchemaInvalid", err)
	}
}

func contains(xs []string, want string) bool {
	return strings.Contains(strings.Join(xs, "|"), want)
}
