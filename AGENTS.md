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

## Error model

Both application SDKs use the same three-tier error model. Know which tier to use before writing or reviewing SDK code.

| Tier | Go | TypeScript | When |
|---|---|---|---|
| Not found | `ErrNotFound` | `NotFoundError` | A `Delete*` target or a required node ref does not exist. |
| Invalid input | `ErrInvalidInput` | `InvalidInputError` | Bad argument: invalid type/tag/relation name, violated constraint (node has edges), closed store, negative limit, etc. |
| Missing required field | `ErrMissingRequiredField` | `MissingRequiredFieldError` | A required field is structurally absent — either a `PutNode` without `Title`, or a decoded file record missing a required field. |

**`GetNode` is a special case:** a missing node returns `(nil, nil)` / `null` (not an error). Check the pointer/value, not the error.

**Filter helpers** return empty results (not errors) for unknown-but-valid types, tags, relations, or metadata keys.

**Negative `Limit`** on `RecentNodes` / `RecentEdges` is `ErrInvalidInput` / `InvalidInputError`.

## SDK parity

The behavioral contract is defined by the shared fixtures in `testdata/behavior/` (`parity-graph.akg` and `parity-spec.json`). Each SDK has a `behavior_parity` test that loads those fixtures and asserts against the spec. If both SDKs pass, they agree on behavior.

A new SDK must pass ≥80% of the behavior parity test cases before it can be officially released (v1.0.0). See [`docs/sdk-author-guide.md`](docs/sdk-author-guide.md) for details.

## Testing

Run all Go tests (reference SDK + conformance fixtures):

```sh
go test -count=1 ./...
```

Run an individual SDK's tests:

```sh
cd sdk/akg-go && go test ./...   # Go SDK
cd sdk/akg-ts && npm test         # TypeScript SDK
```

Each SDK directory contains its own README with full test and build instructions.
