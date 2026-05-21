# AKG conformance fixtures

This directory is the home for binary `.akg` conformance fixtures and golden files shared by the Go reference implementation tests.

## Milestone 1

- `m1-data-bloom-wal.akg` — whole-container fixture containing Data, Bloom, and WAL sections for binary round-trip coverage.

## Milestone 2

Valid store/open fixtures:

- `m2-empty-create.akg` — empty graph produced by the Milestone 2 create path.
- `m2-minimal-node.akg` — compacted graph with one minimal node.
- `m2-full-node.akg` — compacted graph with one populated node including body, tags, and meta.
- `m2-single-edge.akg` — compacted graph with two nodes and one edge.
- `m2-small-graph.akg` — compacted small graph with mixed node types, tags, and edges.
- `m2-committed-wal-replay.akg` — base Data plus committed WAL that ordinary open must replay.
- `m2-uncommitted-wal-tail.akg` — committed WAL followed by trailing uncommitted records/bytes ordinary open must ignore.
- `m2-compacted.akg` — compacted file with live Data/Bloom and no carried-forward WAL.
- `m2-deletes-before-compaction.akg` — WAL history with logical deletes before the final committed state.

Rejection fixtures:

- `m2-reject-malformed-committed-wal.akg` — malformed committed WAL must be rejected by ordinary open.
- `m2-reject-derived-index-mismatch.akg` — Data primary/derived index mismatch must be rejected by ordinary open.
