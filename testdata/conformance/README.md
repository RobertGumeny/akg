# AKG conformance fixtures

This directory is the home for binary `.akg` conformance fixtures and golden files shared by the Go reference implementation tests.

## Manifest

`manifest.json` is the machine-readable index for this corpus. Every `*.akg` fixture in this directory MUST appear exactly once in the manifest, and the Go conformance tests fail if the manifest and filesystem drift.

Top-level format:

```json
{
  "version": 1,
  "fixtures": [
    {
      "path": "fixture-name.akg",
      "purpose": "Human-readable reason this fixture exists.",
      "expected_result": "accept",
      "validation_scope": "store"
    }
  ]
}
```

Fixture fields:

- `path` — fixture file name relative to this directory.
- `purpose` — short description of the behavior covered.
- `expected_result` — `accept` or `reject`.
- `expected_error_category` — required for `reject` fixtures; stable category for conformance runners. Exact error strings are implementation-specific.
- `validation_scope` — the intended validation level. `format` fixtures exercise binary container/section behavior; `store` fixtures exercise ordinary open/validation semantics.
- `store_expectation` — optional Go reference metadata for accepted store fixtures, such as node/edge counts and WAL sequence expectations. Alternate implementations may use it as extra assertions but should treat `expected_result` and `expected_error_category` as the portable contract.

Alternate implementations should load `manifest.json`, open each `path`, and assert that accepted fixtures pass ordinary validation while rejected fixtures fail in the named `expected_error_category` where a category is supplied. Do not depend on Go test names or exact Go error strings.

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
