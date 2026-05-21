---
title: AKG core
status: release-candidate docs
---

# AKG core

AKG is a structured, single-file knowledge graph format for durable agent memory.
Knowledge graphs already work well for agent context, but they are often trapped
inside graph servers, framework-specific stores, or app-specific schemas. AKG
makes the knowledge graph a portable file an agent can carry with it.

This repository is the core/open-source home for the format: the v1 spec, the Go
reference implementation, the conformance corpus, examples, and release-readiness
docs.

AKG core is intentionally small. It defines how nodes and edges are represented
on disk, how files are opened and validated, how committed mutations are stored,
and how implementations can prove they agree on the format.

AKG core is **not** a product SDK, memory-file ingestion system, query engine,
traversal language, merge service, vector database, background daemon, or
multi-writer coordination layer. Those systems can be useful alongside AKG; they
belong above the portable file format rather than inside it.

## Start here

- [Core concepts](docs/core.md) — what AKG is, what it is not, and the main repo pieces.
- [Lifecycle guide](docs/lifecycle.md) — create, mutate, commit, reopen, compact, and validate.
- [Public API](docs/API.md) — intentionally minimal Go reference API.
- [Conformance guide](docs/conformance.md) — using fixtures and `manifest.json` from another implementation.
- [SDK author guide](docs/sdk-author-guide.md) — what to build above core without changing the format.
- [Repository boundaries](docs/repository-boundaries.md) — how spec, conformance, reference code, SDKs, and examples fit together.
- [v1 specification](docs/spec/00-introduction.md) — technical format details.

## Try the example

```sh
go run ./examples/lifecycle
```

The example creates a tiny graph, commits it, reopens it, reads records back,
compacts the file, and validates the result. See
[`examples/lifecycle/README.md`](examples/lifecycle/README.md).

## Validate the repo

```sh
go test -count=1 ./...
```

Conformance fixture checks are described in
[`testdata/conformance/README.md`](testdata/conformance/README.md).
