---
title: "Go module cache permission fallback"
trigger: "When Go commands fail because module cache paths are read-only or temporary test cleanup reports permission denied"
keywords: [go, GOMODCACHE, GOCACHE, GOFLAGS, "-modcacherw", "read-only file system", "go.mod cache", "permission denied"]
date: 2026-06-07
---

## Context

Temporary Liza worktrees may inherit `GOPATH=~/go` while the module cache under `~/go/pkg/mod` is not writable from the agent sandbox.
Installer and end-to-end tests may also create temporary home directories that copy Go module cache contents; Go's default read-only module directories can then make `testing.T.TempDir` cleanup fail.

## Failure Mode

Go commands that need module metadata or downloads fail before reaching the project code, for example:

```text
go: writing go.mod cache: mkdir ~/go/pkg/mod/cache/download/...: read-only file system
```

Full-suite tests can also fail after test logic succeeds, during temporary directory cleanup:

```text
TempDir RemoveAll cleanup: unlinkat .../go/pkg/mod/.../index_test.go: permission denied
```

## Solution

Run Go commands with per-command writable caches under `~/.cache` instead of changing global Go configuration:

```bash
GOPATH=~/.cache/go GOMODCACHE=~/.cache/go/pkg/mod GOCACHE=~/.cache/go/build go test ./...
```

When test cleanup fails on copied module cache contents, add `GOFLAGS=-modcacherw` so Go leaves downloaded module directories writable:

```bash
GOPATH=~/.cache/go GOMODCACHE=~/.cache/go/pkg/mod GOCACHE=~/.cache/go/build GOFLAGS=-modcacherw go test ./...
```

For pre-commit hooks, also set `GOPATH=~/.cache/go` so hook-managed Go tools use the writable cache tree.

## References

- `.pre-commit-config.yaml`
- `Makefile`
