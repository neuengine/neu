// examples/_template is the scaffold for a new engine example. Copy this
// directory to examples/<your-example>/ and replace the body of run().
//
// Convention (verified by cmd/examplecheck against examples/goldens.json):
//   - run() does the deterministic work and returns a stable uint64 hash;
//   - main() prints "PASS: <name> hash=<N>" on success or "FAIL: <reason>";
//   - main_test.go asserts run() is stable across >=20 invocations.
//
// The "_"-prefixed directory name makes the Go tool skip this scaffold for
// `go build ./...` / `go test ./...`, and cmd/examplecheck excludes it too.
package main

import (
	"fmt"
	"hash/fnv"
)

// run performs the example's deterministic work and returns a hash over its
// salient output. Replace this body; keep the (uint64, error) signature.
func run() (uint64, error) {
	h := fnv.New64a()
	for i := range 8 {
		_, _ = fmt.Fprintf(h, "step-%d", i)
	}
	return h.Sum64(), nil
}

func main() {
	got, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: template hash=%d\n", got)
}
