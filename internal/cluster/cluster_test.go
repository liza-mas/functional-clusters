package cluster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildKeepsWeakUtilityBridgeFromMergingDenseRegions(t *testing.T) {
	artifact, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := len(artifact.Clusters), 3; got != want {
		t.Fatalf("clusters = %d, want %d: %#v", got, want, artifact.Clusters)
	}
	assertClusterMembers(t, artifact, "Authentication", []string{"auth.Check", "auth.Login", "logger.Log"})
	assertClusterMembers(t, artifact, "Billing", []string{"billing.Charge", "billing.Invoice"})
}

func TestBuildDoesNotAttachWeakNodeThroughAnotherWeakSingleton(t *testing.T) {
	artifact, err := Build([]byte(weakChainSCIPFixture()), []byte(weakChainArchitectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertClusterMembers(t, artifact, "Authentication", []string{"auth.Check", "auth.Login", "weak.A"})
	assertClusterMembers(t, artifact, "B", []string{"weak.B"})
}

func TestBuildRecordsUnmappedAndOptionalMetadataDiagnostics(t *testing.T) {
	artifact, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := artifact.Unmapped.Symbols, []string{"logger.Log"}; !equal(got, want) {
		t.Fatalf("unmapped symbols = %#v, want %#v", got, want)
	}
	if !hasDiagnostic(artifact.Diagnostics, "repository_metadata_absent") {
		t.Fatalf("expected repository metadata absence diagnostic: %#v", artifact.Diagnostics)
	}
	if !hasDiagnostic(artifact.Diagnostics, "adr_metadata_absent") {
		t.Fatalf("expected ADR metadata absence diagnostic: %#v", artifact.Diagnostics)
	}
}

func TestBuildRecordsValidOptionalMetadataFingerprintsAndMalformedDiagnostics(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo.json")
	adrPath := filepath.Join(dir, "adr.json")
	repoBytes := []byte(`{"repository_name":"demo"}`)
	if err := os.WriteFile(repoPath, repoBytes, 0644); err != nil {
		t.Fatalf("write repo metadata: %v", err)
	}
	if err := os.WriteFile(adrPath, []byte(`{`), 0644); err != nil {
		t.Fatalf("write ADR metadata: %v", err)
	}

	artifact, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:            time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion:       "test",
		RepositoryMetadataPath: repoPath,
		ADRMetadataPath:        adrPath,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if artifact.Inputs.RepositoryMetadata == nil {
		t.Fatal("repository metadata input = nil, want fingerprint")
	}
	if got, want := artifact.Inputs.RepositoryMetadata.Fingerprint, Fingerprint(repoBytes); got != want {
		t.Fatalf("repository metadata fingerprint = %q, want %q", got, want)
	}
	if artifact.Inputs.ADRMetadata != nil {
		t.Fatalf("ADR metadata input = %#v, want nil for malformed optional metadata", artifact.Inputs.ADRMetadata)
	}
	if !hasDiagnostic(artifact.Diagnostics, "adr_metadata_malformed") {
		t.Fatalf("expected malformed ADR diagnostic: %#v", artifact.Diagnostics)
	}
}

func TestBuildUsesMappedADRTitleForLabelWithoutChangingMembership(t *testing.T) {
	dir := t.TempDir()
	adrPath := filepath.Join(dir, "adr.json")
	if err := os.WriteFile(adrPath, []byte(`{"adrs":[{"id":"ADR-1","title":"Identity Access","related_package_paths":["internal/auth"]}]}`), 0644); err != nil {
		t.Fatalf("write ADR metadata: %v", err)
	}

	withoutADR, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() without ADR error = %v", err)
	}
	withADR, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
		ADRMetadataPath:  adrPath,
	})
	if err != nil {
		t.Fatalf("Build() with ADR error = %v", err)
	}

	assertClusterMembers(t, withADR, "Identity Access", []string{"auth.Check", "auth.Login", "logger.Log"})
	if !sameMembership(withoutADR, withADR) {
		t.Fatalf("ADR metadata changed membership:\nwithout=%#v\nwith=%#v", withoutADR.Clusters, withADR.Clusters)
	}
}

func TestBuildSingleClusterReportsLowSeparation(t *testing.T) {
	artifact, err := Build([]byte(singleClusterSCIPFixture()), []byte(singleClusterArchitectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := len(artifact.Clusters), 1; got != want {
		t.Fatalf("clusters = %d, want %d", got, want)
	}
	if got := artifact.Clusters[0].ConfidenceMetrics.Separation; got != 0 {
		t.Fatalf("single cluster separation = %.2f, want 0", got)
	}
}

func sameMembership(left, right Artifact) bool {
	if len(left.Clusters) != len(right.Clusters) {
		return false
	}
	leftMembers := map[string]Members{}
	rightMembers := map[string]Members{}
	for _, cluster := range left.Clusters {
		leftMembers[strings.Join(cluster.Members.Symbols, "\x00")] = cluster.Members
	}
	for _, cluster := range right.Clusters {
		rightMembers[strings.Join(cluster.Members.Symbols, "\x00")] = cluster.Members
	}
	if len(leftMembers) != len(rightMembers) {
		return false
	}
	for key, leftMember := range leftMembers {
		rightMember, ok := rightMembers[key]
		if !ok {
			return false
		}
		if !equal(leftMember.Packages, rightMember.Packages) || !equal(leftMember.Components, rightMember.Components) {
			return false
		}
	}
	return true
}

func TestBuildRejectsMissingRequiredFingerprint(t *testing.T) {
	graph := strings.Replace(scipFixture(), `"fingerprint":"sha256:scip"`, `"fingerprint":""`, 1)

	_, err := Build([]byte(graph), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err == nil {
		t.Fatal("Build() error = nil, want missing fingerprint error")
	}
	if !strings.Contains(err.Error(), "inputs.scip_index.fingerprint") {
		t.Fatalf("error = %q, want fingerprint context", err)
	}
}

func TestRenderListAndExplainUseArtifactOnly(t *testing.T) {
	artifact, err := Build([]byte(scipFixture()), []byte(architectureFixture()), BuildOptions{
		GeneratedAt:      time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: "test",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	var list strings.Builder
	if err := RenderList(artifact, &list); err != nil {
		t.Fatalf("RenderList() error = %v", err)
	}
	if !strings.Contains(list.String(), "ID\tLABEL\tCONFIDENCE\tBAND") {
		t.Fatalf("list output = %q, want header", list.String())
	}
	if !strings.Contains(list.String(), "Authentication") {
		t.Fatalf("list output = %q, want Authentication", list.String())
	}

	var explain strings.Builder
	if err := RenderExplain(artifact, "auth.Login", &explain); err != nil {
		t.Fatalf("RenderExplain() error = %v", err)
	}
	if !strings.Contains(explain.String(), "Cluster:") || !strings.Contains(explain.String(), "Dependency boundaries:") {
		t.Fatalf("explain output = %q, want cluster and boundary sections", explain.String())
	}
}

func TestBuildOutputIsDeterministicExceptGeneratedAt(t *testing.T) {
	opts := BuildOptions{GeneratedAt: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), GeneratorVersion: "test"}
	first, err := Build([]byte(scipFixture()), []byte(architectureFixture()), opts)
	if err != nil {
		t.Fatalf("Build() first error = %v", err)
	}
	second, err := Build([]byte(scipFixture()), []byte(architectureFixture()), opts)
	if err != nil {
		t.Fatalf("Build() second error = %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("artifacts differ:\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
}

func assertClusterMembers(t *testing.T, artifact Artifact, label string, symbols []string) {
	t.Helper()
	for _, cluster := range artifact.Clusters {
		if cluster.Label == label {
			if !equal(cluster.Members.Symbols, symbols) {
				t.Fatalf("%s symbols = %#v, want %#v", label, cluster.Members.Symbols, symbols)
			}
			return
		}
	}
	t.Fatalf("cluster label %q not found in %#v", label, artifact.Clusters)
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func scipFixture() string {
	return `{
  "schema_version":"scip.graph-export.v1",
  "inputs":{"scip_index":{"fingerprint":"sha256:scip"}},
  "nodes":[
    {"id":"auth.Login","display_name":"Login","document_path":"internal/auth/login.go"},
    {"id":"auth.Check","display_name":"Check","document_path":"internal/auth/check.go"},
    {"id":"billing.Invoice","display_name":"Invoice","document_path":"internal/billing/invoice.go"},
    {"id":"billing.Charge","display_name":"Charge","document_path":"internal/billing/charge.go"},
    {"id":"logger.Log","display_name":"Log","document_path":"internal/logging/log.go"}
  ],
  "edges":[
    {"source":"auth.Login","target":"auth.Check","type":"implementation","provenance":"test","occurrence_count":1},
    {"source":"billing.Invoice","target":"billing.Charge","type":"implementation","provenance":"test","occurrence_count":1},
    {"source":"auth.Login","target":"logger.Log","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"auth.Check","target":"logger.Log","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"billing.Invoice","target":"logger.Log","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"billing.Charge","target":"logger.Log","type":"reference","provenance":"test","occurrence_count":1}
  ]
}`
}

func architectureFixture() string {
	return `{
  "schema_version":"stacklit.architecture-export.v1",
  "inputs":{"stacklit_index":{"fingerprint":"sha256:stacklit"}},
  "packages":[
    {"id":"internal/auth","name":"Authentication","description":"Handles authentication."},
    {"id":"internal/billing","name":"Billing","description":"Handles billing."},
    {"id":"internal/unused","name":"Unused","description":"Unmapped package."}
  ],
  "components":[
    {"id":"internal/auth","name":"Authentication","description":"Handles authentication."},
    {"id":"internal/billing","name":"Billing","description":"Handles billing."},
    {"id":"internal/unused","name":"Unused","description":"Unmapped component."}
  ],
  "membership":[
    {"path":"internal/auth/login.go","package":"internal/auth","component":"internal/auth"},
    {"path":"internal/auth/check.go","package":"internal/auth","component":"internal/auth"},
    {"path":"internal/billing/invoice.go","package":"internal/billing","component":"internal/billing"},
    {"path":"internal/billing/charge.go","package":"internal/billing","component":"internal/billing"}
  ],
  "relationships":[
    {"source":"internal/auth","target":"internal/billing","type":"package_dependency"}
  ],
  "entry_points":[{"path":"internal/auth/login.go","kind":"command"}]
}`
}

func weakChainSCIPFixture() string {
	return `{
  "schema_version":"scip.graph-export.v1",
  "inputs":{"scip_index":{"fingerprint":"sha256:scip"}},
  "nodes":[
    {"id":"auth.Login","display_name":"Login","document_path":"internal/auth/login.go"},
    {"id":"auth.Check","display_name":"Check","document_path":"internal/auth/check.go"},
    {"id":"weak.A","display_name":"A","document_path":"internal/weak/a.go"},
    {"id":"weak.B","display_name":"B","document_path":"internal/weak/b.go"}
  ],
  "edges":[
    {"source":"auth.Login","target":"auth.Check","type":"implementation","provenance":"test","occurrence_count":1},
    {"source":"auth.Check","target":"weak.A","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"auth.Login","target":"weak.A","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"auth.Check","target":"weak.A","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"auth.Login","target":"weak.A","type":"reference","provenance":"test","occurrence_count":1},
    {"source":"weak.A","target":"weak.B","type":"reference","provenance":"test","occurrence_count":1}
  ]
}`
}

func weakChainArchitectureFixture() string {
	return `{
  "schema_version":"stacklit.architecture-export.v1",
  "inputs":{"stacklit_index":{"fingerprint":"sha256:stacklit"}},
  "packages":[{"id":"internal/auth","name":"Authentication","description":"Handles authentication."}],
  "components":[{"id":"internal/auth","name":"Authentication","description":"Handles authentication."}],
  "membership":[
    {"path":"internal/auth/login.go","package":"internal/auth","component":"internal/auth"},
    {"path":"internal/auth/check.go","package":"internal/auth","component":"internal/auth"}
  ],
  "relationships":[],
  "entry_points":[]
}`
}

func singleClusterSCIPFixture() string {
	return `{
  "schema_version":"scip.graph-export.v1",
  "inputs":{"scip_index":{"fingerprint":"sha256:scip"}},
  "nodes":[
    {"id":"only.Run","display_name":"Run","document_path":"internal/only/run.go"},
    {"id":"only.Helper","display_name":"Helper","document_path":"internal/only/helper.go"}
  ],
  "edges":[{"source":"only.Run","target":"only.Helper","type":"implementation","provenance":"test","occurrence_count":1}]
}`
}

func singleClusterArchitectureFixture() string {
	return `{
  "schema_version":"stacklit.architecture-export.v1",
  "inputs":{"stacklit_index":{"fingerprint":"sha256:stacklit"}},
  "packages":[{"id":"internal/only","name":"Only","description":"Only cluster."}],
  "components":[{"id":"internal/only","name":"Only","description":"Only cluster."}],
  "membership":[
    {"path":"internal/only/run.go","package":"internal/only","component":"internal/only"},
    {"path":"internal/only/helper.go","package":"internal/only","component":"internal/only"}
  ],
  "relationships":[],
  "entry_points":[]
}`
}
