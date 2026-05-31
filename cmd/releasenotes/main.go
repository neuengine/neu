package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. Exit codes: 0 = ok, 2 = usage/IO error.
func run(argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("releasenotes", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specsDir := fs.String("specs", filepath.Join(".design", "main", "specifications"), "directory of spec markdown files")
	since := fs.String("since", "", "include only Document History entries on/after this YYYY-MM-DD date")
	breaking := fs.Bool("breaking", false, "emit a breaking-change report instead of release notes")
	outPath := fs.String("o", "", "output file (default: stdout)")
	if err := fs.Parse(argv); err != nil {
		return 2
	}

	specs, err := scanSpecs(*specsDir)
	if err != nil {
		fmt.Fprintf(stderr, "releasenotes: %v\n", err)
		return 2
	}
	if len(specs) == 0 {
		fmt.Fprintf(stderr, "releasenotes: no spec files found in %s\n", *specsDir)
		return 2
	}

	var output string
	if *breaking {
		output = RenderBreaking(BreakingChanges(specs))
	} else {
		output = ReleaseNotes(specs, *since)
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(output), 0o644); err != nil {
			fmt.Fprintf(stderr, "releasenotes: %v\n", err)
			return 2
		}
		return 0
	}
	fmt.Fprint(stdout, output)
	return 0
}

// scanSpecs reads and parses every *.md file directly under dir.
func scanSpecs(dir string) ([]SpecDoc, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read specs dir: %w", err)
	}
	var out []SpecDoc
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, parseSpec(e.Name(), string(data)))
	}
	return out, nil
}
