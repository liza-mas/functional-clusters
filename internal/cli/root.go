package cli

import (
	"fmt"
	"io"
)

// Version is set via ldflags at build time.
var Version = "dev"

const usage = `functional-clusters

Usage:
  functional-clusters --help
  functional-clusters --version
`

// Run executes the functional-clusters command line.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "--help", "-h", "help":
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	case "--version", "-v", "version":
		_, _ = fmt.Fprintf(stdout, "functional-clusters %s\n", Version)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command or flag: %s\n", args[0])
		_, _ = fmt.Fprint(stderr, usage)
		return 1
	}
}
