# Functional Clusters

## Goal

Identify functional communities inside a repository.

Examples:

- Authentication
- Billing
- Notifications
- Order Management

Clusters represent logical capabilities rather than packages or directories.

## Philosophy

Clusters are advisory.

They are generated from repository structure.

They are not treated as ground truth.

## Inputs

Required:

- one or more SCIP Search graph exports
- Stacklit architecture export

Optional:

- repository metadata:
  - repository name
  - default branch
  - language summary
- ADR metadata:
  - ADR identifier
  - title
  - status
  - related package/component paths

Optional metadata may improve labels and summaries. It must not be required to
build clusters. When optional metadata is absent, label confidence may be lower
but cluster membership remains derived from SCIP and Stacklit exports.

Optional metadata may influence labels and summaries only. Cluster membership
must remain derived exclusively from graph structure.

## Artifact Metadata

Every cluster artifact includes:

- `schema_version`: `"functional-clusters.v1"`
- `generator`: tool name and version string
- `generated_at`: UTC RFC3339 timestamp
- `inputs.scip_graph.fingerprint`: fingerprint from the SCIP graph export, or
  a deterministic composite fingerprint when multiple SCIP graph exports are
  supplied
- `inputs.stacklit_architecture.fingerprint`: fingerprint from the Stacklit architecture export
- `inputs.repository_metadata.fingerprint`: optional metadata fingerprint, if supplied
- `inputs.adr_metadata.fingerprint`: optional metadata fingerprint, if supplied

## Processing

### Graph Construction

Combine:

- symbol graph
- package graph
- component graph

into a unified repository graph.

### Input Identity Mapping

SCIP and Stacklit inputs are joined by repository-relative document path.

Rules:

- Full SCIP symbols remain the stable symbol identifiers.
- Stacklit package/component identifiers remain the stable architecture identifiers.
- SCIP document paths and Stacklit membership paths must be normalized to
  slash-separated repository-relative paths before joining.
- A SCIP symbol whose document path has no Stacklit membership remains in the
  graph with `architecture_status: "unmapped"`.
- A Stacklit package/component with no SCIP symbols remains in the graph with
  `symbol_status: "unmapped"`.

The clusterer must not drop unmapped nodes silently.

### Determinism

Given identical input artifacts and generator version, regenerated cluster
artifacts must be byte-stable except for `generated_at`.

Rules:

- Symbol node iteration is sorted by full SCIP symbol string.
- Architecture node iteration is sorted by Stacklit package/component identifier.
- Edge iteration is sorted by source identifier, target identifier, edge type, and
  provenance.
- Stochastic algorithms must use a fixed seed recorded by the generator.
- Cluster identifiers are assigned from deterministic cluster keys, not community
  detection order.
- The default cluster key is the lexically smallest member SCIP symbol. If a
  cluster has no symbols, use the lexically smallest package/component
  identifier.
- Output arrays are sorted deterministically by identifier unless a section
  explicitly defines another stable ordering.

### Community Detection

Identify densely connected regions.

The implementation may use:

- Leiden
- Louvain
- future algorithms

Algorithm choice is not part of the public contract.

### Edge Weighting

Unified graph edges use provenance-typed weights before community detection.

Default v1 weight classes:

- `implementation`: strong structural tie
- `contained_dependency`: medium structural tie
- `reference`: weak structural tie
- `package_dependency`: bounded architecture tie
- `component_dependency`: bounded architecture tie

The implementation must not treat all edge types as equal by raw occurrence
count.

High-degree utility symbols must be damped or excluded by deterministic rules so
shared utilities, logging, error handling, generated code, or framework glue do
not fuse unrelated clusters. The artifact records the damping policy name and
version in generator metadata.

Package and component edges must have bounded contribution relative to symbol
edges so coarse architecture edges do not dominate symbol evidence.

### Label Generation

Generate cluster labels using:

- package names
- component names
- symbol names
- architectural descriptions

Labels are deterministic and source-free in v1.

Preferred label sources, in order:

1. ADR title when ADR metadata maps to the cluster's package/component paths
2. most-central component name
3. most-central package name
4. most-central symbol display name

LLM-generated or network-derived labels are out of scope for v1. ADR metadata
may improve `label_quality`; absence of ADR metadata must not change cluster
membership.

### Confidence Calculation

Produce confidence metrics describing:

- cohesion
- separation
- label quality

Each confidence metric is a number from `0.0` to `1.0`.

- `cohesion`: internal connectivity strength within the cluster
- `separation`: weakness of external connectivity compared with internal connectivity
- `label_quality`: evidence strength for the generated human-readable label

Overall confidence is the arithmetic mean of these three metrics.

Confidence bands:

- `high`: `>= 0.75`
- `medium`: `>= 0.50` and `< 0.75`
- `low`: `< 0.50`

## Outputs

### Clusters

For each cluster:

- identifier
- label
- confidence score
- confidence band
- confidence metrics

### Members

For each cluster:

- symbols
- packages
- components

### Relationships

Inter-cluster dependencies.

Relationship `weight` is the deterministic sum of eligible directed underlying
edge weights crossing from one cluster to another.

The rollup preserves direction. Undirected summaries may be rendered by clients,
but the artifact stores directed relationships.

### Summaries

Agent-oriented descriptions:

- purpose
- important symbols
- important entry points

## JSON Schema

The v1 JSON shape is:

```json
{
  "schema_version": "functional-clusters.v1",
  "generator": {
    "name": "functional-clusters",
    "version": "...",
    "random_seed": 0,
    "weighting_policy": "v1-default",
    "damping_policy": "v1-default"
  },
  "generated_at": "2026-06-07T00:00:00Z",
  "inputs": {
    "scip_graph": {
      "schema_version": "scip.graph-export.v1",
      "fingerprint": "sha256:..."
    },
    "stacklit_architecture": {
      "schema_version": "stacklit.architecture-export.v1",
      "fingerprint": "sha256:..."
    }
  },
  "clusters": [
    {
      "id": "cluster-001",
      "label": "Command Handling",
      "confidence": 0.82,
      "confidence_band": "high",
      "confidence_metrics": {
        "cohesion": 0.86,
        "separation": 0.78,
        "label_quality": 0.82
      },
      "members": {
        "symbols": [
          "scip-go gomod example.com/project internal/commands/Run()."
        ],
        "packages": [
          "internal/commands"
        ],
        "components": [
          "internal/commands"
        ]
      },
      "summary": {
        "purpose": "Handles user-facing CLI command execution.",
        "important_symbols": [
          "scip-go gomod example.com/project internal/commands/Run()."
        ],
        "important_entry_points": [
          "cmd/liza/main.go"
        ]
      }
    }
  ],
  "relationships": [
    {
      "source": "cluster-001",
      "target": "cluster-002",
      "type": "dependency",
      "weight": 3
    }
  ],
  "unmapped": {
    "symbols": [],
    "packages": [],
    "components": []
  }
}
```

Arrays are present even when empty.

## Commands

### Build

Generate cluster artifacts.

Required inputs:

- one or more SCIP Search graph export JSON files, supplied by repeating `--scip-graph`
- Stacklit architecture export JSON

Output:

- functional clusters JSON artifact

### Explain

Given a symbol, return:

- cluster membership
- nearby clusters
- dependency boundaries

The command reads the cluster artifact and does not read source files.

### List

List medium- and high-confidence clusters by default, sorted by decreasing
confidence. Include all discovered clusters when `--all` is passed.

## Freshness

Clusters are derived artifacts.

They may be regenerated:

- on demand
- periodically
- during release preparation

They are not required to be rebuilt on every commit.

Consumers can detect stale artifacts by comparing input fingerprints recorded in
the cluster artifact with the current SCIP and Stacklit export fingerprints. When
multiple SCIP graph exports are supplied, the recorded SCIP fingerprint is a
deterministic composite fingerprint derived from the individual SCIP export
fingerprints.

## Error / Degraded Behavior

If a required input artifact cannot be read, has an unsupported
`schema_version`, or lacks required metadata, build fails and emits no partial
cluster artifact.

If graph construction succeeds but produces zero symbols and zero packages,
build succeeds with an empty `clusters` array and `low` aggregate confidence.

If community detection produces a single cluster, build succeeds and records one
cluster. The confidence metrics must make the lack of separation visible.

If optional repository or ADR metadata is absent or malformed, build continues
without that metadata and records the omission in diagnostics.

If clustering would produce different memberships for identical inputs because
of stochastic algorithm behavior or map iteration order, the implementation is
invalid. The generator must enforce fixed seed and deterministic ordering before
writing the artifact.

## Constraints

- No daemon.
- No hidden state.
- No centralized database.
- Worktree compatible.

## Success Criteria

Given valid SCIP graph exports and a valid Stacklit architecture export:

- `build` writes a versioned functional cluster JSON artifact without reading source files
- `list` returns medium- and high-confidence cluster identifiers, labels, confidence scores, and confidence bands by default, sorted by decreasing confidence
- `list --all` returns every cluster identifier, label, confidence score, and confidence band from the artifact
- `explain <symbol>` returns the symbol's cluster membership, nearby clusters, and dependency boundaries from the artifact
- stale inputs can be detected from recorded input fingerprints
- zero-cluster, one-cluster, and unmapped-node cases have deterministic output
- normal multi-cluster output is deterministic for identical inputs
- cluster identifiers and memberships do not change because of algorithm iteration order
