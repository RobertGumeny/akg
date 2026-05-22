# Error Handling and Conformance

AKG defines a fail-closed reader model and a strict writer model.

A conformant reader must reject files that violate the integrity and compatibility rules of the format. It must tolerate forward-compatible additions and apply defined defaults where this specification requires them. A conformant writer must produce files and payloads that satisfy all mandatory format rules.

This section summarizes the required rejection behavior, the required tolerance behavior, the role of explicit recovery, and the conformance test suite used to validate implementations.

## Rejection Requirements

A conformant reader must reject an AKG file in at least the following cases:

- the file magic at byte offset 0 is not `AKG\0`
- the file major version is greater than the highest major version the reader implements
- the header checksum does not validate
- any section checksum does not validate
- the section table is structurally invalid, including out-of-bounds ranges, overlapping ranges, missing required Data section, duplicate Bloom section, duplicate WAL section, or invalid zero-length Data/Bloom section
- the Data section is malformed, including duplicate keys or incomplete entry structure
- the Bloom section is malformed
- a WAL record is truncated, including a record whose declared payload length exceeds the remaining bytes available for that record
- a WAL record uses an unknown operation code
- a required WAL payload field is missing or has the wrong type

These are mandatory rejection conditions. Ordinary read behavior must not attempt heuristic parsing, partial acceptance, or silent repair.

A checksum failure is an integrity failure, not a warning condition.

## Tolerance Requirements

A conformant reader must tolerate the following cases:

- an unknown section type in the section table, which must be skipped if structurally valid
- an unknown MessagePack field in a node, edge, or WAL delete payload, which must be ignored
- a missing optional node or edge field, for which the reader must apply the read-time default defined by Section 3
- a missing `created_at` or `updated_at` field, which must decode as `0` as defined by Section 3
- trailing uncommitted WAL records after the last valid `COMMIT`, which must be ignored during ordinary open

These tolerance rules are part of the AKG backward-compatibility model. A reader must not reject a file merely because it contains forward-compatible fields or section types that the reader does not interpret.

## Writer Strictness

Reader leniency does not relax writer requirements.

A conformant writer must emit all required container structures, required payload fields, and checksums exactly as defined by this specification. Writers may omit only those payload fields for which this specification defines omission as valid and supplies a read-time default. In particular, writers must not omit fields that are required on write and rely on read-time defaults to compensate.

Where this specification requires rejection of invalid input at write time, such rejection is part of conformance.

## Recovery Is Explicit

`akg.recover()` is the explicit rescue path for corrupted files.

Recovery is outside normal read semantics. A conformant ordinary reader must reject a corrupted file rather than attempting salvage automatically. Implementations may provide recovery tooling that extracts readable sections or valid WAL content from a damaged file, but that behavior must be explicit and opt-in. Ordinary open is strict even when a separate salvage path exists.

## Conformance Test Suite

The AKG conformance test suite lives in the Reference SDK repository.

This is the cross-implementation standard for reader behavior. An implementation that claims AKG compatibility should be validated against it.

The test suite must cover at least the following categories.

### Baseline Cases

Baseline cases verify ordinary successful decoding and include:

- empty graph
- minimal node containing required fields only
- fully populated node
- single edge
- small realistic graph with mixed node types, tags, and edges

### Encoding Edge Cases

Encoding edge cases verify payload decoding behavior and include:

- node `meta` containing all valid MessagePack value kinds
- node with the maximum tag count of 32
- node with a very large `body`
- edge with `confidence: null`
- edge with `confidence: 0.5`
- edge with `strength` exactly `0.0`
- edge with `strength` exactly `1.0`

### Format State Cases

Format state cases verify file-lifecycle behavior and include:

- file with an uncompacted WAL that requires replay
- file with trailing uncommitted WAL that must be ignored on ordinary open
- compacted file with no WAL state to replay
- file containing logical deletions in WAL history before compaction

### Rejection Cases

Rejection cases verify mandatory failure behavior and include:

- wrong magic bytes
- unsupported major version
- bad header checksum
- bad section checksum
- overlapping or out-of-bounds sections
- duplicate Data/Bloom/WAL cardinality violations
- malformed Data section
- malformed Bloom section
- truncated WAL record
- invalid WAL opcode
- invalid WAL delete payload shape

## Round-Trip Invariant

A conformant read-write-read cycle must preserve logical graph content.

After reading an AKG file, writing an equivalent AKG file, and reading that rewritten file again, the resulting logical set of nodes, edges, field values, defaults, and identities must remain equivalent.

Byte-for-byte preservation is not required. Physical section order, exact MessagePack encoding choices permitted by the format, bloom filter bit layout derived from the same live keys, and other non-logical layout details may change across a conformant rewrite.
