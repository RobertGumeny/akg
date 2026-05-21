# AKG v1 Requirements Traceability Audit

This audit traces normative `MUST`, `MUST NOT`, `SHOULD`, and `SHOULD NOT` requirements in `docs/spec/01-data-model.md` through `docs/spec/07-error-handling.md` to the Go reference implementation, tests, conformance fixtures, documentation-only requirements, or explicit release-blocking gaps.

Status values:

- **Implemented/tested** — covered by implementation and focused Go tests.
- **Conformance fixture** — covered by `testdata/conformance/manifest.json` and `internal/store/conformance_test.go`.
- **Documentation-only** — normative guidance for implementations or SDK authors, not directly executable in the reference implementation.
- **Release-blocking gap** — must be resolved or intentionally changed before v1 RC.

## Release-blocking gaps found

None.

## 01-data-model.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `DM-001` | Conformant node writers must write `created_at` and `updated_at`; readers missing either timestamp must default it to `0`. | Implemented/tested | `record.EncodeNodePayload`, `record.DecodeNodePayload`, `record.Node.ApplyReadDefaults`; `internal/record/*_test.go`; accepted minimal-node conformance fixture. |
| `DM-002` | Conformant edge writers must write `created_at` and `updated_at`; readers missing either timestamp must default it to `0`. | Implemented/tested | `record.EncodeEdgePayload`, `record.DecodeEdgePayload`, `record.Edge.ApplyReadDefaults`; `internal/record/*_test.go`; single-edge conformance fixture. |
| `DM-003` | Readers/writers must accept custom node `type` and edge `relation` strings without requiring registries. | Implemented/tested | `internal/keys`, `internal/state`; small-graph/full-node conformance fixtures use application-defined values. |

## 02-binary-layout.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `BL-001` | Readers must locate sections through the section table and must not infer physical order. | Implemented/tested | `format.DecodeContainer`; `internal/format/container_test.go`; `TestStoreOpenToleratesUnknownStructurallyValidSection`. |
| `BL-002` | Wrong magic at offset 0 must be rejected immediately; readers must not heuristic-parse or recover. | Conformance fixture | `format.DecodeHeader`; `m3-reject-wrong-magic.akg`. |
| `BL-003` | Header reserved bytes must be zero; writers must write zero; readers must not assign meaning to reserved bytes. | Implemented/tested | `format.EncodeHeader`, `format.DecodeHeader`; `internal/format/container_test.go`. |
| `BL-004` | Readers must reject a major version greater than implemented. | Conformance fixture | `format.DecodeHeader`; `m3-reject-unsupported-major-version.akg`. |
| `BL-005` | Section length includes payload plus trailing checksum; readers must subtract checksum size. AKG v1 section checksums are 4-byte little-endian CRC32; SHA-256/BLAKE3 IDs are reserved and rejected. | Implemented/tested | `format.EncodeHeader`, `format.DecodeHeader`, `format.EncodeSection`, `format.DecodeSection`; container tests and all valid fixtures. |
| `BL-006` | Header or section checksum failure must reject; ordinary readers must fail closed. | Conformance fixture | `m3-reject-bad-header-checksum.akg`, `m3-reject-bad-section-checksum.akg`; `format.ErrChecksumMismatch`. |
| `BL-007` | Unknown section types must be skipped if structurally valid and may appear multiple times. | Implemented/tested | `format.DecodeContainer`; `TestStoreOpenToleratesUnknownStructurallyValidSection`; container tests. |
| `BL-008` | Readers must validate section bounds, overlap, cardinality, and zero-length rules. | Conformance fixture | `format.ValidateSections`; duplicate/overlap rejection fixtures in manifest. |
| `BL-009` | Bloom miss/hit semantics require normal lookup to continue on hit. | Documentation-only | The core validates/rebuilds Bloom data but does not expose a query engine or Bloom API in v1. |

## 03-encoding.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `ENC-001` | Node and edge payloads must be MessagePack maps; positional arrays are non-conformant. | Implemented/tested | `record.DecodeNodePayload`, `record.DecodeEdgePayload`; invalid payload rejection fixtures. |
| `ENC-002` | Writers must encode node payload fields `type`, `title`, `created_at`, and `updated_at`. | Implemented/tested | `record.EncodeNodePayload`; state/store tests. |
| `ENC-003` | Readers must reject node payloads missing `type` or `title`; optional fields get defaults. | Implemented/tested | `record.DecodeNodePayload`; record tests; invalid-node-data conformance fixture. |
| `ENC-004` | Writers must encode edge payload fields `from_node`, `to_node`, `relation`, `created_at`, and `updated_at`. | Implemented/tested | `record.EncodeEdgePayload`; state/store tests. |
| `ENC-005` | Readers must reject edge payloads missing required identity fields; optional fields get defaults. | Implemented/tested | `record.DecodeEdgePayload`; WAL payload rejection fixtures. |
| `ENC-006` | Applying read defaults is normal behavior and must not be treated as an error. | Implemented/tested | `ApplyReadDefaults` methods and record tests. |
| `ENC-007` | Payload strings must be UTF-8. | Implemented/tested / Conformance fixture | `record.decodeMsgpack` rejects invalid UTF-8 for all MessagePack string values and map keys, including nested `meta`; `record.encodeMsgpack` rejects invalid UTF-8 on write; `internal/record/codec_utf8_test.go`; UTF-8 rejection fixtures in manifest. |

## 04-key-layout.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `KEY-001` | Writers must ensure node IDs are opaque strings, contain no `:`, are at most 64 characters, and are unique within type. | Implemented/tested | `keys.BuildNodeKey`, `state.PutNode`; `internal/keys`, `internal/state` tests. |
| `KEY-002` | Writers must reject invalid node IDs and must not normalize, truncate, or rewrite them. | Implemented/tested | `keys.validateNodeID`; keys/state tests. |
| `KEY-003` | Every edge write must produce an inbound `ei:` index entry, atomically with the primary edge entry. | Implemented/tested | `store.MaterializeDataEntries`; `store.HydrateDataEntries` validates derived keys; derived-index mismatch fixtures. |
| `KEY-004` | Every node tag must produce one `t:` index entry. | Implemented/tested | `store.MaterializeDataEntries`; tag-derived mismatch fixtures/tests. |
| `KEY-005` | Tags must be lowercase snake_case, max 32 per node, and reject spaces/non-conformant input without correction. | Implemented/tested | `keys.BuildTagKey`, `state.validateTags`; keys/state tests; max-tag conformance fixture. |
| `KEY-006` | Writers must produce exactly one temporal `ts:` index entry per logical record and no creation-time index. | Implemented/tested | `store.MaterializeDataEntries`; hydrate derived-key validation tests. |
| `KEY-007` | Writers/readers must not assume a title-prefix index. | Documentation-only | No title index exists in implementation or public API; future query behavior remains out of scope. |
| `KEY-008` | Malformed key construction inputs must be rejected clearly; silent correction is non-conformant. | Implemented/tested | `internal/keys`, `internal/state`, `internal/store/hydrate.go`. |

## 05-wal.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `WAL-001` | Writers must append WAL records before treating mutations as durable state. | Implemented/tested | `store.Put*` stages records; `store.Commit`; commit/reopen tests. |
| `WAL-002` | After successful commit, enough WAL state must remain to recover committed mutations. | Implemented/tested | `store.Commit`, `store.Open`; committed-WAL replay fixtures/tests. |
| `WAL-003` | Unknown WAL opcodes must reject. | Conformance fixture | `wal.DecodeRecord`; `m3-reject-invalid-wal-opcode.akg`. |
| `WAL-004` | `COMMIT` records must have `length = 0` and empty payload. | Implemented/tested | `wal.EncodeRecord`, `wal.DecodeRecord`, `wal.ValidatePayload`; wal tests. |
| `WAL-005` | Writers must assign distinct, strictly increasing sequence numbers that are not reused/reset across sessions; readers must reject duplicate or non-increasing sequence numbers in committed WAL prefixes. | Implemented/tested / Conformance fixture | Writer path covered by `TestMultipleCommittedBatchesReplayInSequenceAcrossSessions`; reader validation covered by `TestOpenRejectsCommittedWALWithNonIncreasingSequence` and duplicate/decreasing WAL sequence rejection fixtures. |
| `WAL-006` | PUT/DELETE WAL payloads must match the specified MessagePack identity/payload shapes; unknown delete fields are tolerated. | Conformance fixture | `wal.ValidatePayload`, `record.Decode*`; invalid WAL payload fixtures. |
| `WAL-007` | WAL readers must verify length, checksum, and known operation for every relevant record. | Conformance fixture | `wal.DecodeRecord`; malformed committed WAL and opcode fixtures. |
| `WAL-008` | Ordinary open must reject invalid committed WAL and must not salvage automatically. | Conformance fixture | `store.inspectWAL`; malformed committed WAL fixtures. |
| `WAL-009` | Ordinary open must replay through last valid `COMMIT` and ignore trailing uncommitted records. | Implemented/tested / Conformance fixture | `store.inspectWAL`, `replayWAL`; uncommitted-tail fixture and tests. |
| `WAL-010` | WAL contents with no valid `COMMIT` are uncommitted and not applied. | Implemented/tested | `TestOpenWithNoValidCommitAppliesNoWALMutations`. |
| `WAL-011` | Writers must not partially clear WAL before compaction. | Implemented/tested | `store.Commit` preserves WAL prefix through last commit; commit-discard-tail and compaction tests. |
| `WAL-012` | Implementations must provide a policy that prevents unbounded pending mutation or WAL growth; exact flush policy is implementation-defined, with 1,000 entries or 10 MB recommended. | Documentation-only | `05-wal.md` now treats flush thresholds as writer-side policy rather than file-format conformance. |
| `WAL-013` | `commit()` is the durability boundary; it appends mutation records and `COMMIT`, fsyncs durable state, and does not imply compaction. | Implemented/tested | `store.Commit`, `writeFileSync`; commit/reopen and compact tests. |
| `WAL-014` | Clean close must automatically call `commit()` unless already committed. | Implemented/tested | `store.Close`; `TestCloseCommitsOutstandingMutation`. |

## 06-compaction.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `CMP-001` | Successful compaction must preserve logical graph content except tombstone/WAL history. | Implemented/tested / Conformance fixture | `store.Compact`, `encodeCompactedFile`; compacted/deletes fixtures and compaction tests. |
| `CMP-002` | Compaction must be explicit only; WAL flush thresholds are not compaction triggers. | Implemented/tested | Only `Compact` APIs call compaction; no automatic compaction path exists. |
| `CMP-003` | Compaction must perform ordinary open, materialize live keys, omit tombstones, write sorted Data, rebuild Bloom, write fresh sections without old WAL, and atomically rename. | Implemented/tested | `store.Compact`, `MaterializeDataEntries`, `encodeCompactedFile`, `writeFileAtomic`; compaction tests. |
| `CMP-004` | Compacted file must contain only current live representation; no superseded records, tombstones, or previous WAL. | Implemented/tested | Compaction tests assert empty WAL and live-state preservation. |
| `CMP-005` | Compaction output must preserve key-layout rules; live keys sorted and complete. | Implemented/tested | `MaterializeDataEntries`, `format.EncodeDataEntries`; hydrate validates derived keys. |
| `CMP-006` | Compaction must rebuild Bloom from scratch with v1 parameters and must not copy the old Bloom unchanged. | Implemented/tested | `format.EncodeBloom`, `validateBloom`; compaction tests. |
| `CMP-007` | Compaction must use atomic rename and must not expose a hybrid file. | Implemented/tested | `writeFileAtomic`; tests cover atomic replacement at API level. |
| `CMP-008` | Compacted file must not retain previous WAL; live WAL mutations must be reflected in Data. | Implemented/tested / Conformance fixture | Compacted fixture and compaction tests. |
| `CMP-009` | Merge logic must account for tombstone removal. | Documentation-only | Merge is out of Milestone 3 implementation scope. |

## 07-error-handling.md

| ID | Requirement | Status | Trace |
| --- | --- | --- | --- |
| `ERR-001` | Readers must reject integrity/compatibility violations, tolerate forward-compatible additions/defaults, and writers must produce mandatory structures. | Implemented/tested / Conformance fixture | Public `Open`/`Validate`; manifest-driven conformance tests. |
| `ERR-002` | Mandatory rejection cases: bad magic, unsupported major, checksums, invalid section table, malformed Data/Bloom, truncated/unknown WAL, required WAL payload errors. | Conformance fixture | `testdata/conformance/manifest.json`, `internal/store/conformance_test.go`. |
| `ERR-003` | Ordinary read behavior must not heuristic-parse, partially accept, or silently repair mandatory rejection failures. | Conformance fixture | Rejection fixtures open through public `Open`. |
| `ERR-004` | Readers must tolerate unknown sections, unknown MessagePack fields, optional defaults, missing timestamps defaulting to `0`, and trailing uncommitted WAL. | Implemented/tested / Conformance fixture | Unknown-section test, record defaults tests, uncommitted-tail fixture. |
| `ERR-005` | Writers must emit required structures/fields/checksums and must not rely on read-time defaults for write-required fields. | Implemented/tested | Encode paths in `internal/format`, `internal/record`, `internal/store`; create/commit tests. |
| `ERR-006` | Recovery must be explicit; ordinary open is strict. | Documentation-only / Conformance fixture | No public recovery API exists; corrupted fixtures reject through `Open`. |
| `ERR-007` | Conformance corpus should be used by compatible implementations and must cover baseline, edge, lifecycle, and rejection categories. | Documentation-only / Conformance fixture | `testdata/conformance/README.md`, manifest and fixtures. |
| `ERR-008` | Read-write-read must preserve logical graph content. | Implemented/tested | Public API round-trip/compaction tests and conformance accept fixtures. |

## Notes for follow-up tasks

- No release-blocking requirements gaps remain in this audit.
- This audit intentionally does not add query, traversal, merge, background service, or multi-writer behavior.
- Public API review remains Task 5/6 scope; this audit only records whether spec obligations are currently traceable.
