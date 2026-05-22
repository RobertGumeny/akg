---
title: AKG — Agent Knowledge Graph File Format
status: release-candidate docs
---

# AKG — Agent Knowledge Graph File Format

AKG is a portable, single-file knowledge graph format for persistent agent memory.
The idea is simple: a knowledge graph is a great structure for what an agent
knows, but most graph storage is trapped inside servers or framework-specific
stores. AKG makes it a file — something an agent can open, update, compact, and
carry across tools and hosts.

This repository is the open-source home for the format: the v1 spec, a Go
Reference SDK that lives alongside the spec, conformance tests, and examples.

## Who this is for

**Building an app in Go?** Use the [akg-go SDK](sdk/akg-go/README.md). It is
the production Go library with the full public API — tag lookup, edge traversal,
and everything you need to build on top of AKG.

**Implementing AKG in another language?** Start with the
[v1 specification](docs/spec/00-introduction.md) and the
[conformance guide](docs/conformance.md). The conformance fixtures in
`testdata/conformance/` are your compatibility contract — you do not need to
import or copy any Go code. The Go Reference SDK in this repo exists to prove the
spec works and to give you a concrete behavior target; study it, but do not treat
it as a blueprint for your own internal architecture.

**Exploring the format?** The [overview](docs/core.md) and
[lifecycle guide](docs/lifecycle.md) are the best starting points.

## Repo contents

- [Overview](docs/core.md) — what AKG is, what it is not, and the main repo pieces.
- [Lifecycle guide](docs/lifecycle.md) — create, mutate, commit, reopen, compact, and validate.
- [Public API](docs/API.md) — the minimal Go Reference SDK API.
- [Conformance guide](docs/conformance.md) — using fixtures and `manifest.json` from another implementation.
- [SDK author guide](docs/sdk-author-guide.md) — implementing AKG support in a new language.
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
