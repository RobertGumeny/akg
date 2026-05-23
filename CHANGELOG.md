# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-22

### Added

- The AKG v1 binary format specification lives in `docs/spec/` — this is the authoritative contract for all implementations.
- A Go Reference SDK (`akg.go`, `internal/`) that proves the spec is implementable end-to-end.
- A Go application SDK (`sdk/akg-go/`) with a full public API: `Open`, `PutNode`, `PutEdge`, `GetNode`, `ListNodes`, `ListNodesByTag`, `OutboundEdges`, `InboundEdges`, `DeleteNode`, `DeleteEdge`, `Commit`, `Close`.
- A TypeScript application SDK (`sdk/akg-ts/`) with the same API surface using async/await.
- 36 conformance fixtures in `testdata/conformance/` that cover the full range of valid files, malformed-file rejection, WAL replay, compaction, and edge deletion. These are the correctness contract for any future SDK port.
- A conformance guide (`docs/conformance.md`) and SDK author guide (`docs/sdk-author-guide.md`) for anyone who wants to implement AKG in another language.
- Apache 2.0 license.