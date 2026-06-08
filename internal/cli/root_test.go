package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/functional-clusters/internal/cluster"
)

func TestRunVersion(t *testing.T) {
	originalVersion := Version
	Version = "source:test:abc123"
	t.Cleanup(func() {
		Version = originalVersion
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"--version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "functional-clusters source:test:abc123\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"--help"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknown(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"--missing"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command or flag: --missing") {
		t.Fatalf("stderr = %q, want unknown command message", stderr.String())
	}
}

func TestRunBuildListExplain(t *testing.T) {
	dir := t.TempDir()
	scipPath := filepath.Join(dir, "scip.json")
	stacklitPath := filepath.Join(dir, "stacklit.json")
	clustersPath := filepath.Join(dir, "clusters.json")
	if err := os.WriteFile(scipPath, []byte(cliSCIPFixture()), 0644); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
	if err := os.WriteFile(stacklitPath, []byte(cliStacklitFixture()), 0644); err != nil {
		t.Fatalf("write stacklit fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"build", "--scip-graph", scipPath, "--stacklit-architecture", stacklitPath, "-o", clustersPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("build exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("build stdout = %q, want empty", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"list", "--clusters", clustersPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("list exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Command Handling") {
		t.Fatalf("list stdout = %q, want cluster label", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"explain", "--clusters", clustersPath, "cmd.Main"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("explain exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Dependency boundaries:") {
		t.Fatalf("explain stdout = %q, want dependency boundaries", stdout.String())
	}
}

func TestRunListAllIncludesLowConfidenceClusters(t *testing.T) {
	dir := t.TempDir()
	clustersPath := filepath.Join(dir, "clusters.json")
	artifact := cluster.Artifact{
		SchemaVersion: cluster.SchemaVersion,
		Clusters: []cluster.Cluster{
			{ID: "cluster-001", Label: "Command Handling", Confidence: 0.75, ConfidenceBand: "high"},
			{ID: "cluster-002", Label: "err", Confidence: 0.28, ConfidenceBand: "low"},
		},
	}
	if err := cluster.WriteArtifact(clustersPath, artifact); err != nil {
		t.Fatalf("write cluster artifact: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"list", "--clusters", clustersPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("list exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if strings.Contains(stdout.String(), "err") {
		t.Fatalf("filtered list stdout = %q, want low-confidence cluster omitted", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"list", "--clusters", clustersPath, "--all"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("list --all exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "err") {
		t.Fatalf("list --all stdout = %q, want low-confidence cluster included", stdout.String())
	}
}

func TestRunBuildAcceptsMultipleSCIPGraphs(t *testing.T) {
	dir := t.TempDir()
	scipPath := filepath.Join(dir, "scip.json")
	extraSCIPPath := filepath.Join(dir, "extra-scip.json")
	stacklitPath := filepath.Join(dir, "stacklit.json")
	clustersPath := filepath.Join(dir, "clusters.json")
	if err := os.WriteFile(scipPath, []byte(cliSCIPFixture()), 0644); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
	if err := os.WriteFile(extraSCIPPath, []byte(cliExtraSCIPFixture()), 0644); err != nil {
		t.Fatalf("write extra scip fixture: %v", err)
	}
	if err := os.WriteFile(stacklitPath, []byte(cliStacklitFixture()), 0644); err != nil {
		t.Fatalf("write stacklit fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{
		"build",
		"--scip-graph", scipPath,
		"--scip-graph", extraSCIPPath,
		"--stacklit-architecture", stacklitPath,
		"-o", clustersPath,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("build exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	data, err := os.ReadFile(clustersPath)
	if err != nil {
		t.Fatalf("read clusters output: %v", err)
	}
	if !strings.Contains(string(data), "worker.Run") {
		t.Fatalf("clusters output = %s, want symbol from repeated --scip-graph", data)
	}
}

func TestRunBuildFailureDoesNotEmitPartialOutput(t *testing.T) {
	dir := t.TempDir()
	scipPath := filepath.Join(dir, "scip.json")
	stacklitPath := filepath.Join(dir, "stacklit.json")
	clustersPath := filepath.Join(dir, "clusters.json")
	if err := os.WriteFile(scipPath, []byte(`{"schema_version":"wrong"}`), 0644); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
	if err := os.WriteFile(stacklitPath, []byte(cliStacklitFixture()), 0644); err != nil {
		t.Fatalf("write stacklit fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"build", "--scip-graph", scipPath, "--stacklit-architecture", stacklitPath, "-o", clustersPath}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if _, err := os.Stat(clustersPath); !os.IsNotExist(err) {
		t.Fatalf("output file exists after failed build, stat err = %v", err)
	}
}

func TestRunBuildStdoutWriteFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	scipPath := filepath.Join(dir, "scip.json")
	stacklitPath := filepath.Join(dir, "stacklit.json")
	if err := os.WriteFile(scipPath, []byte(cliSCIPFixture()), 0644); err != nil {
		t.Fatalf("write scip fixture: %v", err)
	}
	if err := os.WriteFile(stacklitPath, []byte(cliStacklitFixture()), 0644); err != nil {
		t.Fatalf("write stacklit fixture: %v", err)
	}

	var stderr bytes.Buffer
	exitCode := Run([]string{"build", "--scip-graph", scipPath, "--stacklit-architecture", stacklitPath}, failingWriter{}, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "write stdout") {
		t.Fatalf("stderr = %q, want stdout write context", stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func cliSCIPFixture() string {
	return `{
  "schema_version":"scip.graph-export.v1",
  "inputs":{"scip_index":{"fingerprint":"sha256:scip"}},
  "nodes":[
    {"id":"cmd.Main","display_name":"Main","document_path":"cmd/main.go"},
    {"id":"internal.Run","display_name":"Run","document_path":"internal/cli/root.go"}
  ],
  "edges":[{"source":"cmd.Main","target":"internal.Run","type":"implementation","provenance":"test","occurrence_count":1}]
	}`
}

func cliExtraSCIPFixture() string {
	return `{
  "schema_version":"scip.graph-export.v1",
  "inputs":{"scip_index":{"fingerprint":"sha256:extra-scip"}},
  "nodes":[{"id":"worker.Run","display_name":"Run","document_path":"internal/worker/run.go"}],
  "edges":[]
}`
}

func cliStacklitFixture() string {
	return `{
  "schema_version":"stacklit.architecture-export.v1",
  "inputs":{"stacklit_index":{"fingerprint":"sha256:stacklit"}},
  "packages":[{"id":"internal/cli","name":"Command Handling","description":"Handles CLI commands."}],
  "components":[{"id":"internal/cli","name":"Command Handling","description":"Handles CLI commands."}],
  "membership":[
    {"path":"cmd/main.go","package":"internal/cli","component":"internal/cli"},
    {"path":"internal/cli/root.go","package":"internal/cli","component":"internal/cli"}
  ],
  "relationships":[],
  "entry_points":[{"path":"cmd/main.go","kind":"command"}]
}`
}
