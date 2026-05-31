package main

import "os"

// buildRouter registers all available commands. Only implemented commands are
// registered, so help never advertises a command its subsystem can't back (INV-3).
func buildRouter() *Router {
	r := NewRouter()
	r.Register(doctorCmd{})
	r.Register(scaffoldCmd{})
	r.Register(pluginCmd{})
	return r
}

func main() {
	os.Exit(buildRouter().Run(os.Args[1:], os.Stdout, os.Stderr))
}
