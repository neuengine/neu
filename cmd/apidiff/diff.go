package main

import "sort"

// Change is one API delta between two snapshots. Old/New hold the signatures
// (one side empty for Removed/Added).
type Change struct {
	Pkg  string
	Kind string
	Name string
	Old  string
	New  string
}

// Changes groups the API deltas. Removed and Changed are breaking under SemVer
// (an existing export disappeared or its signature changed); Added is a
// backward-compatible minor change.
type Changes struct {
	Removed []Change
	Changed []Change
	Added   []Change
}

// Breaking reports whether the diff contains a backward-incompatible change.
func (c Changes) Breaking() bool {
	return len(c.Removed) > 0 || len(c.Changed) > 0
}

// Diff compares two snapshots package-by-package. A symbol is matched by
// (kind, name); a signature difference is a Change, presence-only differences
// are Removed/Added. A whole package present only in old surfaces as Removed.
func Diff(old, new Snapshot) Changes {
	var ch Changes
	for _, pkg := range unionKeys(old.Packages, new.Packages) {
		oldByKey := indexSymbols(old.Packages[pkg])
		newByKey := indexSymbols(new.Packages[pkg])

		for key, os := range oldByKey {
			ns, ok := newByKey[key]
			if !ok {
				ch.Removed = append(ch.Removed, Change{Pkg: pkg, Kind: os.Kind, Name: os.Name, Old: os.Signature})
				continue
			}
			if ns.Signature != os.Signature {
				ch.Changed = append(ch.Changed, Change{Pkg: pkg, Kind: os.Kind, Name: os.Name, Old: os.Signature, New: ns.Signature})
			}
		}
		for key, ns := range newByKey {
			if _, ok := oldByKey[key]; !ok {
				ch.Added = append(ch.Added, Change{Pkg: pkg, Kind: ns.Kind, Name: ns.Name, New: ns.Signature})
			}
		}
	}
	sortChanges(ch.Removed)
	sortChanges(ch.Changed)
	sortChanges(ch.Added)
	return ch
}

// indexSymbols keys a symbol slice by "kind\x00name" for matching.
func indexSymbols(syms []Symbol) map[string]Symbol {
	m := make(map[string]Symbol, len(syms))
	for _, s := range syms {
		m[s.Kind+"\x00"+s.Name] = s
	}
	return m
}

// unionKeys returns the sorted union of map keys.
func unionKeys(a, b map[string][]Symbol) []string {
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortChanges(cs []Change) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Pkg != cs[j].Pkg {
			return cs[i].Pkg < cs[j].Pkg
		}
		if cs[i].Kind != cs[j].Kind {
			return cs[i].Kind < cs[j].Kind
		}
		return cs[i].Name < cs[j].Name
	})
}
