# Milestone 1 Tasks

Source: `docs/akg-go-sdk-execution-tracker.md`

Milestone 1 goal: scaffold the Go module and lock in the spec-shaped binary/container primitives before higher-level API work.

## Task 1 — Scaffold the Go module, package layout, and test harness

Create the initial Go project structure for the Phase 1 AKG reference SDK.

### Scope

- Add `go.mod`.
- Create package directories:
  - `internal/record/`
  - `internal/keys/`
  - `internal/format/`
  - `internal/wal/`
  - `internal/state/`
  - `internal/store/`
  - `cmd/akg/`
  - `testdata/conformance/`
- Add baseline package files and smoke tests so `go test ./...` runs cleanly.

### Acceptance criteria

- `go test ./...` passes from the repo root.
- All Milestone 1 packages compile, even if some contain only foundational types or stubs.
- `testdata/conformance/` exists and is documented as the home for binary fixtures.
- No public SDK/API design is introduced beyond what is necessary for compiling Milestone 1 internals.

## Task 2 — Define canonical records, IDs, versions, sections, and WAL types

Add the core structs and enums shared by the format, record, and WAL packages.

### Scope

- Define node and edge record structs.
- Define canonical ID, timestamp, version, and sequence-number types where useful.
- Define section descriptors and known section type constants.
- Define WAL operation enums, including `COMMIT`, `PUT_NODE`, `PUT_EDGE`, `DELETE_NODE`, and `DELETE_EDGE`.
- Define explicit delete payload structs/maps for node and edge deletes.

### Acceptance criteria

- Canonical types are available to downstream packages without circular imports.
- WAL operation constants are stable and covered by tests.
- Delete payload shapes match the tracker/spec requirement: explicit MessagePack map identities with required fields.
- Read-time handling allows unknown extra fields on delete payloads where applicable.
- Write-time validation rejects missing required fields for records and delete payloads.

## Task 3 — Implement canonical key builders and parsers

Implement the spec-shaped key layer for primary, secondary, type, and temporal keys.

### Scope

- Implement builders/parsers for:
  - `n:{type}:{id}` node keys
  - `e:{from}:{relation}:{to}` edge keys
  - `ei:` edge-index keys
  - `t:` type keys
  - `ts:{timestamp}:n:{type}:{id}` temporal node keys
  - `ts:{timestamp}:e:{from}:{relation}:{to}` temporal edge keys
- Ensure byte-level canonical output is deterministic.
- Add table-driven tests for valid and invalid keys.

### Acceptance criteria

- Builders emit exactly the propagated spec key shapes, including self-describing `ts:` keys.
- Parsers round-trip all builder output.
- Parsers reject malformed, ambiguous, or incomplete keys.
- Tests cover raw bytewise lexicographic ordering assumptions where relevant.
- `go test ./...` passes.

## Task 4 — Implement binary container primitives: header, section table, checksums, and Data entries

Implement the low-level `.akg` container encoding/decoding required before open/validate behavior can be built.

### Scope

- Implement header encode/decode with explicit little-endian fixed-width integers.
- Implement section-table encode/decode.
- Implement checksum validation for known validated regions.
- Implement section cardinality validation:
  - exactly one Data section required
  - at most one Bloom section
  - at most one WAL section
  - zero-length Data/Bloom invalid
  - zero-length WAL allowed
  - structurally valid unknown sections skipped
- Implement section range/overlap validation.
- Implement Data-section entry encode/decode using repeated flat entries:
  - `key_len:uint32`
  - `value_len:uint32`
  - raw key bytes
  - raw value bytes
  - little-endian integers
  - no padding

### Acceptance criteria

- Fixture-backed round-trip tests pass for header, section table, and Data entries.
- Validation rejects missing Data sections, duplicate Data sections, duplicate Bloom/WAL sections, invalid zero-length Data/Bloom sections, overlapping ranges, and out-of-file ranges.
- Validation accepts zero-length WAL sections.
- Data decoding rejects malformed/truncated entries and duplicate keys.
- Data encoding produces bytewise lexicographically sorted keys.
- All fixed-width integers are explicitly tested as little-endian.

## Task 5 — Implement Bloom and WAL wire formats with first conformance fixtures

Finish the remaining Milestone 1 wire-format pieces and prove them with binary round-trip fixtures.

### Scope

- Implement deterministic Bloom-section encode/decode with fixed v1 parameters from the propagated spec.
- Implement WAL record encode/decode.
- Implement committed-tail detection/replay helper behavior through the last valid `COMMIT`.
- Ensure ordinary replay ignores trailing uncommitted WAL records but rejects malformed committed WAL.
- Add initial binary conformance fixtures covering Data, Bloom, and WAL round trips.

### Acceptance criteria

- Bloom encode/decode is deterministic and validates fixed v1 parameters.
- WAL round-trip tests cover `PUT_NODE`, `PUT_EDGE`, `DELETE_NODE`, `DELETE_EDGE`, and `COMMIT`.
- WAL delete tests cover explicit required payload fields and tolerance for unknown extra fields on read.
- Replay tests apply records through the last valid `COMMIT` only.
- Tests show trailing uncommitted WAL is ignored.
- Tests show malformed committed WAL is rejected.
- At least one fixture-backed whole-container binary round-trip test passes.
- `go test ./...` passes.
