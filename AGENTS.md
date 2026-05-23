# Agent Guide

This file explains what is authoritative in this repo so agents and contributors know where to look and what to trust.

## Repo structure

```
akg/
├── docs/spec/              # Authoritative format specification (v1)
├── docs/conformance.md     # How to run conformance tests
├── docs/sdk-author-guide.md# Guide for implementing AKG in a new language
├── docs/lifecycle.md       # File lifecycle: create, mutate, commit, compact
├── internal/               # Go Reference SDK internals (spec proof, not a blueprint)
├── akg.go                  # Go Reference SDK public surface
├── sdk/akg-go/             # Go application SDK (full public API)
├── sdk/akg-ts/             # TypeScript application SDK (full public API)
├── testdata/conformance/   # Conformance fixtures — the correctness contract
└── examples/               # Runnable examples
```

## What is authoritative

- **`docs/spec/`** is the authoritative format contract. When the spec and any implementation disagree, the spec wins.
- **`testdata/conformance/`** are the conformance fixtures. Every SDK must pass all fixtures. `manifest.json` describes each fixture and its expected outcome.
- **`sdk/akg-go/`** and **`sdk/akg-ts/`** are the application SDKs. Their READMEs document the public API.
- **The root package** (`akg.go`, `internal/`) is the Go Reference SDK. It proves the spec is implementable and is the behavior target for other implementations. It is not a blueprint for internal architecture — application SDKs implement the format independently.

## What is not authoritative

- `docs/PRD.md` and `docs/backlog.md` are process artifacts for the current work cycle. They describe ongoing implementation goals, not permanent project truth.

## Testing

Run all Go tests (reference SDK + conformance fixtures):

```sh
go test -count=1 ./...
```

Run the Go application SDK tests:

```sh
cd sdk/akg-go && go test ./...
```

Run the TypeScript application SDK tests:

```sh
cd sdk/akg-ts && npm test
```
