package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/liza-mas/functional-clusters/internal/cluster"
)

// Version is set via ldflags at build time.
var Version = "dev"

const usage = `functional-clusters

Usage:
  functional-clusters build --scip-graph <file> [--scip-graph <file>...] --stacklit-architecture <file> -o <file>
  functional-clusters list --clusters <file>
  functional-clusters explain --clusters <file> <symbol>
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
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
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

func runBuild(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, output, err := parseBuildArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	opts.GeneratedAt = time.Now().UTC()
	opts.GeneratorVersion = Version
	artifact, err := cluster.BuildFromFiles(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "build failed: %v\n", err)
		return 1
	}
	if output == "" {
		output = "-"
	}
	if output == "-" {
		data, err := clusterJSON(artifact)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "build failed: %v\n", err)
			return 1
		}
		if _, err := stdout.Write(data); err != nil {
			_, _ = fmt.Fprintf(stderr, "build failed: write stdout: %v\n", err)
			return 1
		}
		return 0
	}
	if err := cluster.WriteArtifact(output, artifact); err != nil {
		_, _ = fmt.Fprintf(stderr, "build failed: %v\n", err)
		return 1
	}
	return 0
}

func runList(args []string, stdout io.Writer, stderr io.Writer) int {
	path, err := parseClusterPath(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	artifact, err := cluster.ReadArtifact(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "list failed: %v\n", err)
		return 1
	}
	if err := cluster.RenderList(artifact, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "list failed: %v\n", err)
		return 1
	}
	return 0
}

func runExplain(args []string, stdout io.Writer, stderr io.Writer) int {
	path, symbol, err := parseExplainArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	artifact, err := cluster.ReadArtifact(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "explain failed: %v\n", err)
		return 1
	}
	if err := cluster.RenderExplain(artifact, symbol, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "explain failed: %v\n", err)
		return 1
	}
	return 0
}

func parseBuildArgs(args []string) (cluster.BuildOptions, string, error) {
	var opts cluster.BuildOptions
	var output string
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) {
			return opts, output, fmt.Errorf("missing value for %s", args[i])
		}
		value := args[i+1]
		switch args[i] {
		case "--scip-graph":
			opts.SCIPGraphPaths = append(opts.SCIPGraphPaths, value)
		case "--stacklit-architecture":
			opts.StacklitArchitecturePath = value
		case "-o", "--output":
			output = value
		case "--repository-metadata":
			opts.RepositoryMetadataPath = value
		case "--adr-metadata":
			opts.ADRMetadataPath = value
		default:
			return opts, output, fmt.Errorf("unknown build flag: %s", args[i])
		}
		i++
	}
	if len(opts.SCIPGraphPaths) == 0 && opts.SCIPGraphPath == "" {
		return opts, output, fmt.Errorf("missing required --scip-graph")
	}
	if opts.StacklitArchitecturePath == "" {
		return opts, output, fmt.Errorf("missing required --stacklit-architecture")
	}
	return opts, output, nil
}

func parseClusterPath(args []string) (string, error) {
	if len(args) != 2 || args[0] != "--clusters" {
		return "", fmt.Errorf("usage: functional-clusters list --clusters <file>")
	}
	return args[1], nil
}

func parseExplainArgs(args []string) (string, string, error) {
	if len(args) != 3 || args[0] != "--clusters" {
		return "", "", fmt.Errorf("usage: functional-clusters explain --clusters <file> <symbol>")
	}
	return args[1], args[2], nil
}
