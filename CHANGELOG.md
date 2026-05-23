# Changelog

All notable changes to AKG are documented here.

## v0.1.0 — 2026-05-22

### Added

- AKG v1 binary format specification (`docs/spec/`)
- Go Reference SDK (`akg.go`, `internal/`) proving the spec
- Go application SDK (`sdk/akg-go/`) with full public API: `Open`, `PutNode`, `PutEdge`, `GetNode`, `ListNodes`, `ListNodesByTag`, `OutboundEdges`, `InboundEdges`, `DeleteNode`, `DeleteEdge`, `Commit`, `Close`
- TypeScript application SDK (`sdk/akg-ts/`) with equivalent API using async/await
- 36 conformance fixtures in `testdata/conformance/` covering accepted files, malformed-file rejection, WAL replay, compaction, and edge deletion
- Conformance guide (`docs/conformance.md`) for implementing AKG in a new language
- SDK author guide (`docs/sdk-author-guide.md`)
- Apache 2.0 license
