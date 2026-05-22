---
title: AKG repository boundaries
status: release-candidate docs
---

# AKG repository boundaries

This repository should stay understandable as the AKG project. The current
release-candidate boundary is:

## Specification

Path: [`docs/spec`](spec/00-introduction.md)

The spec is the detailed format contract. It is allowed to be technical. It
defines data model, binary layout, encoding, key layout, WAL, compaction, and
error-handling requirements.

## Conformance tests

Path: [`testdata/conformance`](../testdata/conformance/README.md)

The conformance test collection contains `.akg` test fixtures plus a `manifest.json`. It lets alternate
implementation SDKs test accept/reject behavior without depending on Go test names or Go error strings.

## Go Reference SDK

Paths: [`akg.go`](../akg.go), [`internal`](../internal), [`cmd/akg`](../cmd/akg)

The Go code is the Reference SDK — it lives here to prove the spec works and
give implementers a concrete behavior target. The root package exposes the small
public API documented in [API.md](API.md). Internals may be organized however
the Reference SDK needs, but they are not a public SDK contract.

## Examples

Path: [`examples`](../examples)

Examples should demonstrate AKG file lifecycle or format usage. They should not
become product prototypes or memory ingestion systems during v1 RC.

## Future SDKs

Future official or community SDKs may live in separate repositories or in a
clearly named area such as `sdks/` if the project later chooses that layout. They
should document their own product-level APIs and keep AKG file compatibility
anchored to the spec and conformance tests.

## Documentation graph

Use small linked Markdown documents instead of one large document:

- [Overview](core.md)
- [Lifecycle guide](lifecycle.md)
- [Conformance guide](conformance.md)
- [SDK author guide](sdk-author-guide.md)
- [Public API](API.md)
- [Spec introduction](spec/00-introduction.md)

New docs should include YAML frontmatter so they can be rendered by static-site
or documentation tooling later.
