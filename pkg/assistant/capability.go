//go:build editor

package assistant

import "strings"

// Capability is the bitfield of permissions an agent declares at registration
// (L1 §4.3). The user approves or restricts them; runtime operations outside the
// granted set are rejected and logged (INV-2).
type Capability uint32

const (
	// Read capabilities.
	ReadTypeRegistry Capability = 1 << iota
	ReadScenes
	ReadDefinitions
	ReadAssetManifest
	ReadDiagnostics
	// Write capabilities.
	WriteDefinitions
	WriteScenes
	SpawnEntities
	ModifyComponents
	ExecuteCommands
	// Advanced capabilities.
	FileSystemAccess
	NetworkAccess
	CodeGeneration
)

// Has reports whether all capabilities in f are granted in c.
func (c Capability) Has(f Capability) bool { return c&f == f }

// With returns c with the capabilities in f added.
func (c Capability) With(f Capability) Capability { return c | f }

// capNames pairs single-bit capabilities with labels for String().
var capNames = []struct {
	bit  Capability
	name string
}{
	{ReadTypeRegistry, "ReadTypeRegistry"},
	{ReadScenes, "ReadScenes"},
	{ReadDefinitions, "ReadDefinitions"},
	{ReadAssetManifest, "ReadAssetManifest"},
	{ReadDiagnostics, "ReadDiagnostics"},
	{WriteDefinitions, "WriteDefinitions"},
	{WriteScenes, "WriteScenes"},
	{SpawnEntities, "SpawnEntities"},
	{ModifyComponents, "ModifyComponents"},
	{ExecuteCommands, "ExecuteCommands"},
	{FileSystemAccess, "FileSystemAccess"},
	{NetworkAccess, "NetworkAccess"},
	{CodeGeneration, "CodeGeneration"},
}

// String lists the granted capabilities, e.g. "ReadScenes|WriteScenes".
func (c Capability) String() string {
	if c == 0 {
		return "none"
	}
	var b strings.Builder
	for _, cn := range capNames {
		if c.Has(cn.bit) {
			if b.Len() > 0 {
				b.WriteByte('|')
			}
			b.WriteString(cn.name)
		}
	}
	return b.String()
}
