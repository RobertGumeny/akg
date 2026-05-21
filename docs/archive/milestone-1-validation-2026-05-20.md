# AKG Validation Plan

This document tracks validation work beyond ordinary unit tests. It is intended to be usable by both humans and coding agents before Milestone 2 work builds on Milestone 1 binary/container internals.

## Purpose

Milestone 1 establishes the low-level AKG wire formats:

- container header
- section table
- section checksums
- Data section entries
- Bloom section payloads
- WAL records and committed-tail replay behavior
- first binary conformance fixtures

The main validation goal is confidence that malformed or corrupted bytes fail closed, valid bytes round-trip deterministically, and implementation behavior matches `docs/spec/` before higher-level store/open/write APIs are introduced.

## Validation Levels

### Level 1 — Normal test suite

Run after every change:

```bash
go test ./...
```

Expected result: all packages pass.

### Level 2 — Milestone 1 acceptance audit

Manual or agent-assisted checklist:

- [x] Re-read `docs/TASKS.md` Tasks 1–5.
- [x] Confirm every Task 1–5 acceptance criterion has either:
  - direct test coverage,
  - fixture-backed coverage,
  - or an intentional note explaining why it is deferred.
- [x] Confirm implementation remains internal-only for Milestone 1.
- [x] Confirm no public SDK/API design was introduced accidentally.
- [x] Confirm implementation aligns with:
  - `docs/spec/02-binary-layout.md`
  - `docs/spec/03-encoding.md`
  - `docs/spec/05-wal.md`
  - `docs/spec/07-error-handling.md`
  - `docs/spec/09-appendix.md`
- [x] Run `gofmt` on changed Go files.
- [x] Run `go test ./...` from repo root.

#### Milestone 1 acceptance audit — 2026-05-20

Level 1 command: `go test ./...` passed from the repository root.

Level 2 command sequence: re-read Tasks 1–5, reviewed the relevant spec files listed above, ran `gofmt` on changed Go files, and ran `go test ./...` from the repository root.

Acceptance coverage summary:

- Task 1: module scaffold, internal package layout, conformance fixture directory documentation, compile smoke tests, and root `go test ./...` are covered by the repository structure and package tests.
- Task 2: canonical record/types, WAL operation constants, delete payload required fields, write-time validation, and read-time tolerance for unknown delete fields are covered in `internal/record/*_test.go` and `internal/wal/*_test.go`.
- Task 3: canonical key builders/parsers, malformed-key rejection, round trips, and bytewise ordering assumptions are covered in `internal/keys/keys_test.go`.
- Task 4: header, section table, checksum, section-cardinality/range validation, Data entry sorting, duplicate/truncation rejection, and fixed-width little-endian checks are covered in `internal/format/container_test.go`; fixture-backed container/header/section/Data behavior is covered in `internal/format/conformance_test.go`.
- Task 5: deterministic Bloom payloads, fixed v1 Bloom parameters, WAL record round trips for all operations, delete payload tolerance/rejection, committed-tail replay, ignored uncommitted tails, malformed committed-WAL rejection, and a byte-identical whole-container fixture round trip are covered in `internal/format/bloom_test.go`, `internal/wal/codec_test.go`, and `internal/format/conformance_test.go`.

Internal-only/API audit:

- Milestone 1 implementation remains under `internal/` plus the placeholder `cmd/akg` command.
- No non-internal Go package or public SDK/open/store API was added.
- `internal/state` and `internal/store` remain scaffold packages only; Milestone 2 store/open/write behavior has not been started.

Known deferred validation beyond Level 2:

- Level 4 corruption matrix and Level 5/property/fuzz work remain future validation work as listed below; Level 3 fixture hex/golden checks are complete as of 2026-05-20.
- WAL lifecycle behavior requiring fsync, automatic flush thresholds, compaction, and recovery tooling is intentionally deferred beyond Milestone 1 because no public store/open/write API exists yet.

### Level 3 — Fixture validation

Fixture tests should prove that binary files are not just logically correct, but byte-layout correct.

Recommended checks:

- [x] Parse `testdata/conformance/m1-data-bloom-wal.akg` successfully.
- [x] Assert exact header size is 64 bytes.
- [x] Assert magic bytes are `AKG\0`.
- [x] Assert version fields match AKG v1.
- [x] Assert checksum algorithm is CRC32.
- [x] Assert section count is expected.
- [x] Assert section-table entries have expected types, offsets, and lengths.
- [x] Assert Data section decodes into expected sorted keys.
- [x] Assert Bloom section decodes with fixed v1 parameters.
- [x] Assert Bloom reports fixture Data keys as possible members.
- [x] Assert WAL decodes and replays through last valid `COMMIT`.
- [x] Assert whole-container re-encode is byte-identical where deterministic helpers are used.

Optional stronger checks:

- [x] Add golden hex snippets for:
  - header bytes,
  - section table bytes,
  - first Data entry prefix,
  - first WAL record header,
  - Bloom fixed-parameter prefix.

#### Fixture validation audit — 2026-05-20

Level 3 fixture validation is implemented in `internal/format/conformance_test.go` by `TestConformanceFixtureLevel3Validation` and passes under `go test ./...`. The test validates the Milestone 1 conformance fixture header, section table, Data/Bloom/WAL payloads, golden byte snippets, committed WAL replay, and deterministic whole-container re-encoding.

### Level 4 — Corruption/rejection tests

These tests intentionally damage otherwise valid bytes and assert fail-closed behavior.

Recommended cases:

#### Header corruption

- [x] Wrong magic is rejected.
- [x] Unsupported major version is rejected.
- [x] Bad header checksum is rejected.
- [x] Non-zero reserved bytes are rejected.

#### Section table/range corruption

- [x] Missing Data section is rejected.
- [x] Duplicate Data section is rejected.
- [x] Duplicate Bloom section is rejected.
- [x] Duplicate WAL section is rejected.
- [x] Overlapping sections are rejected.
- [x] Out-of-file sections are rejected.
- [x] Zero-length Data section is rejected.
- [x] Zero-length Bloom section is rejected.
- [x] Zero-length WAL section is accepted.
- [x] Unknown structurally valid section type is skipped/accepted.

#### Section checksum corruption

- [x] Flipping one payload bit causes checksum validation failure.
- [x] Flipping one checksum bit causes checksum validation failure.

#### Data corruption

- [x] Truncated entry header is rejected.
- [x] Declared key length beyond payload is rejected.
- [x] Declared value length beyond payload is rejected.
- [x] Duplicate keys are rejected.
- [x] Unsorted keys are rejected.

#### Bloom corruption

- [x] Payload shorter than fixed header is rejected.
- [x] Wrong `hash_function_count` is rejected.
- [x] Wrong `hash_seed` is rejected.
- [x] Wrong `bit_count` for `key_count` is rejected.
- [x] Wrong bit-array byte length is rejected.
- [x] Non-zero padding bits beyond `bit_count` are rejected.

#### WAL corruption

- [x] Truncated record header is rejected.
- [x] Declared payload length beyond available bytes is rejected.
- [x] Bad WAL checksum is rejected.
- [x] Unknown operation code is rejected.
- [x] `COMMIT` with non-empty payload is rejected.
- [x] Missing required `DELETE_NODE` fields are rejected when committed.
- [x] Missing required `DELETE_EDGE` fields are rejected when committed.
- [x] Unknown extra delete payload fields are tolerated on read.
- [x] Trailing uncommitted records after last valid `COMMIT` are ignored.
- [x] Trailing malformed bytes after last valid `COMMIT` are ignored as uncommitted tail.
- [x] Malformed records before or within committed range are rejected.

#### Corruption/rejection audit — 2026-05-20

Level 4 corruption/rejection validation is implemented across `internal/format/container_test.go`, `internal/format/bloom_test.go`, and `internal/wal/codec_test.go`. The tests mutate valid header, section table/range, section checksum, Data, Bloom, and WAL bytes and assert fail-closed behavior, while preserving the specified accept cases for zero-length WAL sections, structurally valid unknown sections, unknown delete payload fields, and trailing uncommitted WAL tails.

### Level 5 — Round-trip/property tests

Property-style tests assert invariants over many ordinary cases.

Recommended invariants:

#### Data entries

- [x] `EncodeDataEntries -> DecodeDataEntries -> EncodeDataEntries` is stable.
- [x] Encoded keys are always bytewise lexicographically sorted.
- [x] Duplicate input keys are rejected before encoding.

#### Bloom

- [x] Same key list produces byte-identical Bloom payloads.
- [x] Decoded Bloom never reports false negatives for inserted keys.
- [x] Bloom payload fixed fields are little-endian where specified.

#### WAL

- [x] `EncodeRecord -> DecodeRecord` preserves sequence, operation, and payload.
- [x] `EncodeRecords -> DecodeRecordsStrict` preserves record order.
- [x] CRC32 checksum bytes are little-endian.
- [x] Replay returns only records before the last valid `COMMIT`, excluding `COMMIT` itself.

#### Whole container

- [x] `EncodeContainer -> DecodeContainer -> EncodeContainer` is stable for deterministic section order.
- [x] Known sections are decoded; unknown sections are skipped if structurally valid.

#### Round-trip/property audit — 2026-05-20

Level 5 round-trip/property validation is implemented across `internal/format/container_test.go`, `internal/format/bloom_test.go`, and `internal/wal/codec_test.go`. Added coverage checks Data encode/decode/re-encode stability, bytewise key sorting, duplicate input rejection, Bloom determinism/no-false-negatives/fixed-field endianness, WAL single-record and multi-record round trips, little-endian CRC32 bytes, last-commit replay semantics, whole-container stable re-encoding, and structurally valid unknown-section skipping while decoding known sections.

## Fuzz Testing

Fuzz tests feed automatically generated malformed input into parsers. The primary invariant is that decoders must never panic, hang, or read out of bounds. Most random inputs are expected to return errors.

Normal test runs compile fuzz targets:

```bash
go test ./...
```

To actively fuzz a target:

```bash
go test ./internal/format -fuzz=FuzzDecodeDataEntries -fuzztime=30s
```

Recommended fuzz targets:

### `internal/format`

- [x] `FuzzDecodeHeader`
- [x] `FuzzDecodeSectionTable`
- [x] `FuzzDecodeSection`
- [x] `FuzzDecodeDataEntries`
- [x] `FuzzDecodeBloom`
- [x] `FuzzDecodeContainer`

Example skeleton:

```go
func FuzzDecodeDataEntries(f *testing.F) {
    f.Add([]byte{})
    f.Add([]byte{1, 0, 0, 0})
    f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 'a'})

    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = DecodeDataEntries(data)
    })
}
```

### `internal/wal`

- [x] `FuzzDecodeRecord`
- [x] `FuzzDecodeRecordsStrict`
- [x] `FuzzCommittedRecords`
- [x] `FuzzReplayCommitted`

Example skeleton:

```go
func FuzzDecodeRecord(f *testing.F) {
    f.Add([]byte{})
    f.Add([]byte{1, 0, 0, 0})

    f.Fuzz(func(t *testing.T, data []byte) {
        _, _, _ = DecodeRecord(data)
    })
}
```

### `internal/record`

- [x] `FuzzDecodeNodePayload`
- [x] `FuzzDecodeEdgePayload`
- [x] `FuzzDecodeNodeDeletePayload`
- [x] `FuzzDecodeEdgeDeletePayload`

#### Fuzz target audit — 2026-05-20

Recommended panic-safety fuzz targets are implemented in `internal/format/fuzz_test.go`, `internal/wal/fuzz_test.go`, and `internal/record/fuzz_test.go`. Normal `go test ./...` compiles all fuzz targets successfully. Active fuzzing remains an optional manual validation step.

Example skeleton:

```go
func FuzzDecodeNodeDeletePayload(f *testing.F) {
    f.Add([]byte{0x82, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa2, 'i', 'd', 0xa1, '1'})

    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = DecodeNodeDeletePayload(data)
    })
}
```

## Suggested Agent Workflow

When asking an agent to perform validation, use a focused request such as:

> Perform the Milestone 1 validation pass from `docs/VALIDATION.md`. Start with the acceptance audit, then add focused corruption tests and fuzz targets for binary decoders. Do not introduce public SDK APIs. Run `gofmt` and `go test ./...`.

Recommended agent sequence:

1. Read:
   - `docs/TASKS.md`
   - `docs/VALIDATION.md`
   - relevant files in `docs/spec/`
2. Inspect current tests and implementation.
3. Identify missing checklist items.
4. Add focused tests first; avoid broad refactors.
5. Add fuzz targets for parser entry points.
6. Run `gofmt`.
7. Run `go test ./...`.
8. Optionally run selected fuzz targets for 30–60 seconds each.
9. Summarize:
   - what was validated,
   - what tests were added,
   - what remains deferred,
   - exact commands run.

## Suggested Manual Workflow

For a human reviewer:

1. Run:

```bash
go test ./...
```

2. Pick one parser and run a short fuzz session, for example:

```bash
go test ./internal/wal -fuzz=FuzzDecodeRecord -fuzztime=30s
```

3. Open the conformance fixture test and confirm it checks real binary behavior, not just in-memory structs.
4. Review any TODO/deferred checklist items in this file.
5. Only start Milestone 2 after the Milestone 1 acceptance audit is clean or known gaps are explicitly documented.

## Before Milestone 2

Milestone 2 should not start until:

- [x] `go test ./...` passes.
- [x] Milestone 1 acceptance audit is complete.
- [x] At least one whole-container fixture-backed round trip passes.
- [x] Core binary decoders have panic-safety fuzz targets.
- [x] Any known spec deviations are documented.
- [x] No accidental public SDK/API design has been introduced.

#### Before Milestone 2 audit — 2026-05-20

`go test ./...` passes, the Milestone 1 acceptance audit is complete, fixture-backed whole-container round trips pass, and recommended decoder fuzz targets compile. No active Milestone 1 binary-format spec deviations are known. Deferred WAL lifecycle/API behavior remains documented in the Level 2 audit and is outside Milestone 1. No public SDK/API design has been introduced.
