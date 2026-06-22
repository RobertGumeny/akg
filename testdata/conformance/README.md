---
title: AKG conformance test files
status: release-candidate docs
---

# AKG test files

This directory contains small `.akg` files that other implementations can use to check whether they read AKG the same way the Go reference implementation does.

Think of each file as an example with an expected answer:

- **accept** means “a normal AKG reader should open this.”
- **reject** means “a normal AKG reader should refuse this.”

Some files are healthy examples. Some are intentionally broken. The broken ones are just as important: they make sure readers fail closed instead of silently accepting bad data.

## Start here: `manifest.json`

`manifest.json` is the table of contents for this directory. Every `.akg` file must appear there exactly once. The Go tests fail if a file is missing from the manifest, if the manifest points at a file that does not exist, or if the recorded hash no longer matches the file bytes.

A minimal entry looks like this:

```json
{
  "path": "fixture-name.akg",
  "purpose": "Human-readable reason this file exists.",
  "expected_result": "accept",
  "validation_scope": "store",
  "sha256": "..."
}
```

Fields:

- `path` — file name, relative to this directory.
- `purpose` — why this file exists.
- `expected_result` — `accept` or `reject`.
- `expected_error_category` — required for `reject` files. Exact error strings can differ between implementations; this is the stable category to compare.
- `validation_scope` — `format` for low-level container/section behavior, `store` for ordinary open/validation behavior.
- `sha256` — hash of the exact file bytes. If the bytes change, this changes.
- `generated_by` — for accepted files, notes the deterministic Go reference workflow that produced the file.
- `corruption` — for rejected files, explains what was intentionally damaged.
- `store_expectation` — optional extra checks used by the Go reference tests, such as node count, edge count, and WAL sequence expectations.
- `features` — array of logical capabilities exercised by the fixture (e.g. `["nodes", "edges", "wal", "delete_node", "delete_edge", "tags", "bloom", "compaction"]`). Informational only; ignored by the conformance runner. Use it to assess blast radius before a format change: `grep '"edges"' manifest.json` lists every fixture that contains edge records and would need regeneration or revalidation if the edge format changes.

If you are writing another AKG reader, the main loop is simple: load `manifest.json`, read each `path`, then check whether your reader accepts or rejects it as declared. Go test names and Go error messages can be useful clues while debugging, but they are not the cross-implementation contract. The stable contract is the manifest: `expected_result` plus `expected_error_category` for rejected files.

## Checking the files

Run the focused checks:

```sh
go test -count=1 ./internal/format ./internal/store
go run ./internal/cmd/conformance-fixtures -dir testdata/conformance
```

The helper command verifies two things:

1. the bytes on disk still match the `sha256` values in `manifest.json`;
2. `store`-scoped files still accept or reject as declared.

To deterministically rewrite the Milestone 3 rejection fixtures before auditing their hashes:

```sh
go run ./internal/cmd/conformance-fixtures -dir testdata/conformance -write-task3-rejections -print-hashes
```

To deterministically rewrite the accepted unknown-section tolerance fixture before auditing its hash:

```sh
go run ./internal/cmd/conformance-fixtures -dir testdata/conformance -write-unknown-section-accept -print-hashes
```

To print the current hashes while reviewing a change:

```sh
go run ./internal/cmd/conformance-fixtures -dir testdata/conformance -print-hashes
```

## Updating a test file safely

Changing one of these files is allowed, but it should be rare and deliberate. Treat `.akg` files in this directory as reference specimens: if they change in a PR, reviewers should slow down and ask why.

Use this workflow:

1. Make the code or fixture change.
2. Run the focused checks above.
3. Run the full suite: `go test -count=1 ./...`.
4. Review the fixture-byte change.
5. Update the file’s `sha256` in `manifest.json` only after you understand why the bytes changed.
6. If the file is intentionally broken, update its `corruption` note so the damage is easy to audit later.

Please do not submit hash-only churn. A hash change without an explained file change is suspicious by design. In reviews, any changed `.akg` file or changed manifest hash deserves extra attention, even if the tests pass.

Generated valid files should be stable across repeated local runs. Fixture generation should avoid wall-clock timestamps and random IDs; timestamps, IDs, section ordering, Data key ordering, Bloom parameters, and WAL sequences should be fixed by the fixture workflow.

## Current files

### Milestone 1

- `m1-data-bloom-wal.akg` — a low-level container example with Data, Bloom, and WAL sections.

## Binary major versions (read-compat)

The binary major is **2**: the tag-index key is type-qualified (`t:{tag}:{type}:{id}`). A conformant major-2 reader **must also read major-1 files** (the legacy `t:{tag}:{id}` shape), distinguished by the tag key's component count, but **always writes major 2** — so a major-1 file self-upgrades to major 2 on its next compaction. The following accepted files are deliberately left at **major 1** to lock that read-compat contract; their bytes (and hashes) must not change to major 2:

- `m2-empty-create.akg`, `m2-minimal-node.akg` — major-1 node/empty specimens.
- `m2-compacted.akg` — a major-1 file that **carries tags**, so it exercises the major-1 (`t:{tag}:{id}`) tag-key read path under a major-2 reader.

A major-2 reader must reject **major 3** (`m3-reject-unsupported-major-version.akg`).

### Milestone 2 accepted files

These should open normally:

- `m2-empty-create.akg` — an empty graph created by the reference implementation.
- `m2-minimal-node.akg` — one minimal node after compaction.
- `m2-full-node.akg` — one populated node with body, tags, and metadata.
- `m2-single-edge.akg` — two nodes and one edge after compaction.
- `m2-small-graph.akg` — a small graph with mixed node types, tags, and edges.
- `m2-collision-type-qualified.akg` — the major-2 regression lock for the tag-index key collision: two nodes share the id `shared` across types (`person`, `project`) and both carry the tag `topic`. The type-qualified tag key keeps them distinct, so the file reads clean; under the major-1 key shape it could not exist.
- `m2-committed-wal-replay.akg` — base Data plus committed WAL records that ordinary open must replay.
- `m2-uncommitted-wal-tail.akg` — committed WAL followed by uncommitted trailing bytes that ordinary open must ignore.
- `m2-compacted.akg` — live Data/Bloom after compaction, with no carried-forward WAL.
- `m2-deletes-before-compaction.akg` — WAL history with deletes before the final committed state.
- `m2-utf8-keys.akg` — a compacted graph whose type (`Person`), relation (`KNOWS`), and tags (`café`, `Active`) are key-safe UTF-8 strings rather than snake_case, plus a node whose type is exactly 64 bytes. A conformant reader must accept all of these: casing and word-separation are an SDK convention, not a format rule, and the length cap is 64 bytes.
- `m3-unknown-section-tolerated.akg` — a normal store file with a structurally valid unknown section that readers skip while hydrating known sections.

### Milestone 2 rejected files

These are intentionally damaged and should not open normally:

- `m2-reject-malformed-committed-wal.akg` — starts from a valid empty store plus a committed WAL batch, then flips a byte inside the committed WAL record. Ordinary open must reject it.
- `m2-reject-derived-index-mismatch.akg` — starts from a valid graph, then damages the derived-index Data keys so they no longer match the primary node/edge records. Ordinary open must reject it.

### Milestone 3 rejected files

These expand fail-closed coverage for v1 format and validation errors:

- `m3-reject-wrong-magic.akg` — wrong container magic bytes.
- `m3-reject-unsupported-major-version.akg` — major 3 (one past the current supported major 2) with a valid header checksum. A reader must reject it via the `major > currentMajor` gate while still accepting major 1 (read-compat) and major 2 (current).
- `m3-reject-bad-header-checksum.akg` — damaged header checksum.
- `m3-reject-bad-section-checksum.akg` — damaged section checksum.
- `m3-reject-duplicate-data-sections.akg` — duplicate Data sections where v1 requires exactly one.
- `m3-reject-overlapping-sections.akg` — overlapping Data/Bloom section ranges.
- `m3-reject-malformed-bloom.akg` — invalid Bloom payload shape.
- `m3-reject-invalid-wal-opcode.akg` — unknown WAL opcode.
- `m3-reject-invalid-wal-put-node-payload.akg` — malformed committed `PUT_NODE` payload.
- `m3-reject-invalid-wal-put-node-utf8-payload.akg` — syntactically valid committed `PUT_NODE` payload with invalid nested UTF-8.
- `m3-reject-invalid-wal-delete-node-payload.akg` — malformed committed `DELETE_NODE` payload.
- `m3-reject-invalid-wal-put-edge-payload.akg` — malformed committed `PUT_EDGE` payload.
- `m3-reject-invalid-wal-delete-edge-payload.akg` — malformed committed `DELETE_EDGE` payload.
- `m3-reject-malformed-committed-wal-checksum.akg` — damaged checksum in a committed WAL batch.
- `m3-reject-duplicate-wal-sequence.akg` — committed WAL prefix with duplicate sequence numbers.
- `m3-reject-decreasing-wal-sequence.akg` — committed WAL prefix with decreasing sequence numbers.
- `m3-reject-invalid-node-data-payload.akg` — primary node Data key with an invalid node payload.
- `m3-reject-invalid-node-data-utf8-payload.akg` — primary node Data key with invalid nested UTF-8.
- `m3-reject-missing-derived-tag-index.akg` — node tag without the required derived tag index key.
- `m3-reject-oversize-type-key.akg` — primary node Data key whose type is 65 bytes, one over the 64-byte cap. Built from a valid at-cap store, then the type is pushed one byte over in every key that embeds it (Bloom regenerated), so the byte cap is what fails.
