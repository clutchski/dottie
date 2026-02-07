package console

import (
	"io"
	"os"
)

// Console holds output writers determined by verbosity.
type Console struct {
	Stdout io.Writer
	Stderr io.Writer
}

// New creates a Console. Quiet mode discards all output.
func New(quiet bool) *Console {
	if quiet {
		return &Console{Stdout: io.Discard, Stderr: io.Discard}
	}
	return &Console{Stdout: os.Stdout, Stderr: os.Stderr}
}
