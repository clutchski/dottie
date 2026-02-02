package main

import (
	"os"

	"github.com/clutchski/dottie/internal/cli"
)

// Build-time variables (set via ldflags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cli.SetVersion(version, commit, date)
	os.Exit(cli.Run(os.Args[1:]))
}
