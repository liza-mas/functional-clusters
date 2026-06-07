# Functional Clusters

Functional Clusters is a Go CLI project.

It builds deterministic advisory functional-cluster artifacts from:

- SCIP Search graph export JSON (`scip.graph-export.v1`)
- Stacklit architecture export JSON (`stacklit.architecture-export.v1`)

## Install and Run

```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/functional-clusters/main/install.sh | sh
functional-clusters --version
```

## Usage

Build a cluster artifact:

```bash
functional-clusters build \
  --scip-graph scip-graph.json \
  --stacklit-architecture stacklit-architecture.json \
  -o functional-clusters.json
```

Optional repository and ADR metadata can be supplied with
`--repository-metadata` and `--adr-metadata`. Missing or malformed optional
metadata is recorded in artifact diagnostics and does not change cluster
membership.

List clusters:

```bash
functional-clusters list --clusters functional-clusters.json
```

Explain a symbol's cluster membership:

```bash
functional-clusters explain --clusters functional-clusters.json 'scip-go gomod example.com/project internal/commands/Run().'
```

`list` and `explain` produce deterministic plain text in v1. The cluster
artifact itself is JSON with schema version `functional-clusters.v1`.

From a local clone:

```bash
make install
functional-clusters --version
```

Use `INSTALL_DIR=<directory> make install` to install from a local clone into a custom directory.

## Development

```bash
make build
make test
make run
```
