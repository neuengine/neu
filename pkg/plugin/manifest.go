package plugin

import (
	"fmt"
	"strings"
)

// EntrySpec is the delivery-mode-specific entry point (one of in/out-of-process).
type EntrySpec struct {
	// In-process:
	PackagePath string
	Factory     string
	// Out-of-process:
	Binary    string
	Transport string
	Endpoint  string
}

// Manifest is the parsed `plugin.toml` (L1 §4.2). It is validated up front so a
// plugin with an invalid manifest is rejected before any code runs (INV-1).
type Manifest struct {
	Entry          EntrySpec
	ID             PluginID
	Version        string
	Name           string
	Description    string
	ChecksumSHA256 string
	License        string
	EngineVersion  string
	RequiredCaps   []Capability
	Platforms      []string
	OptionalCaps   []Capability
	Authors        []string
	Mode           Mode
}

// Validate performs semantic checks beyond parse: required fields present, the
// version + engine constraint parse, and the entry block matches the mode.
func (m Manifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("%w: missing plugin.id", ErrManifestInvalid)
	}
	if !strings.Contains(string(m.ID), ".") {
		return fmt.Errorf("%w: plugin.id %q must be reverse-DNS", ErrManifestInvalid, m.ID)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("%w: plugin.version: %v", ErrManifestInvalid, err)
	}
	if m.EngineVersion == "" {
		return fmt.Errorf("%w: missing compatibility.engine_version", ErrManifestInvalid)
	}
	if _, err := ParseConstraint(m.EngineVersion); err != nil {
		return fmt.Errorf("%w: compatibility.engine_version: %v", ErrManifestInvalid, err)
	}
	switch m.Mode {
	case ModeInProcess:
		if m.Entry.PackagePath == "" || m.Entry.Factory == "" {
			return fmt.Errorf("%w: in-process entry needs package_path + factory", ErrManifestInvalid)
		}
	case ModeOutOfProcess:
		if m.Entry.Binary == "" || m.Entry.Transport == "" {
			return fmt.Errorf("%w: out-of-process entry needs binary + transport", ErrManifestInvalid)
		}
	}
	return nil
}

// CompatibleWith reports whether the manifest's engine_version constraint
// admits the running engine version (INV-2).
func (m Manifest) CompatibleWith(engine Version) (bool, error) {
	c, err := ParseConstraint(m.EngineVersion)
	if err != nil {
		return false, err
	}
	return c.Matches(engine), nil
}

// ParseManifest reads the documented `plugin.toml` subset: `[section]` headers
// and `key = "string"` / `key = ["a", "b"]` assignments. Comments (`#`) and
// blank lines are ignored. This is a minimal stdlib-only reader (C-003); a full
// TOML library is an ADR-gated future option.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	var section string
	for raw := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Manifest{}, fmt.Errorf("%w: bad line %q", ErrManifestInvalid, line)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		assign(&m, section, key, val)
	}
	return m, nil
}

// assign writes one (section,key,value) into the manifest.
func assign(m *Manifest, section, key, val string) {
	switch section {
	case "plugin":
		switch key {
		case "id":
			m.ID = PluginID(unquote(val))
		case "version":
			m.Version = unquote(val)
		case "name":
			m.Name = unquote(val)
		case "description":
			m.Description = unquote(val)
		case "authors":
			m.Authors = parseArray(val)
		case "license":
			m.License = unquote(val)
		case "mode":
			m.Mode, _ = ParseMode(unquote(val))
		}
	case "compatibility":
		switch key {
		case "engine_version":
			m.EngineVersion = unquote(val)
		case "platforms":
			m.Platforms = parseArray(val)
		}
	case "capabilities.required":
		if key == "items" {
			m.RequiredCaps = toCaps(parseArray(val))
		}
	case "capabilities.optional":
		if key == "items" {
			m.OptionalCaps = toCaps(parseArray(val))
		}
	case "entry.in_process":
		switch key {
		case "package_path":
			m.Entry.PackagePath = unquote(val)
		case "factory":
			m.Entry.Factory = unquote(val)
		}
	case "entry.out_of_process":
		switch key {
		case "binary":
			m.Entry.Binary = unquote(val)
		case "transport":
			m.Entry.Transport = unquote(val)
		case "endpoint":
			m.Entry.Endpoint = unquote(val)
		case "checksum_sha256":
			m.ChecksumSHA256 = unquote(val)
		}
	}
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	return s
}

// parseArray parses `["a", "b"]` into []string.
func parseArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if v := unquote(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func toCaps(ss []string) []Capability {
	out := make([]Capability, len(ss))
	for i, s := range ss {
		out[i] = Capability(s)
	}
	return out
}
