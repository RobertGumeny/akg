---
title: AKG write-ahead log
status: v1 draft
---

# Write-Ahead Log

AKG defines a write-ahead log (WAL) as a file section that records committed mutations. It serves two purposes: crash-safe recovery of committed state, and — for writers that choose it — cheap append-on-commit that avoids rewriting the whole file between compactions.

The commit write strategy is an implementation choice, not a format mandate. A writer MAY append WAL records as its durable commit path, leaving the Data section as the last-compacted baseline; or it MAY rewrite the whole file on commit, folding mutations into the Data section and leaving the WAL empty. Both are conformant, and both must be crash-atomic. The rules in this section govern the WAL when a writer uses it, and govern how any reader replays whatever WAL is present.

The WAL is represented as a section in the AKG file, as defined in Section 2. Its contents are an ordered sequence of WAL records.

## Purpose

The WAL records committed mutation intent so that committed state is recoverable after a crash.

For a writer using the append strategy, the writer-first ordering is:

1. append WAL records
2. reach a `COMMIT` record at the chosen durability boundary
3. fsync on `commit()`
4. treat the committed mutation set as durable

A process crash before `commit()` may lose recent buffered work. A process crash after a successful `commit()` must leave enough durable state to recover the committed logical mutations — an appended-and-fsynced WAL prefix for the append strategy, or an atomically-renamed rewritten file for the rewrite strategy.

## Operation Types

AKG defines exactly five WAL operation types:

- `PUT_NODE` = `0x01`
- `DELETE_NODE` = `0x02`
- `PUT_EDGE` = `0x03`
- `DELETE_EDGE` = `0x04`
- `COMMIT` = `0x05`

`PUT_NODE` creates or updates a node. `PUT_EDGE` creates or updates an edge. AKG does not define separate update operations in the WAL; a mutation of an existing record is represented as another `PUT` carrying the new payload and incremented `version`.

`DELETE_NODE` and `DELETE_EDGE` represent tombstoning of the corresponding logical record.

`COMMIT` marks a consistency point in the WAL. Its payload is empty.

A conformant reader that encounters an unknown WAL operation code must reject the WAL as non-conformant.

## Record Structure

Each WAL record has the following structure:

- `sequence: uint64`
- `operation: uint8`
- `length: uint32`
- `payload: bytes`
- `checksum: uint32`

All fixed-width integer fields in WAL records use little-endian encoding. The checksum is a CRC32 computed over `sequence`, `operation`, `length`, and `payload`.

`payload` contains MessagePack-encoded data for node and edge operations. `COMMIT` records must have `length = 0` and an empty payload.

## Sequence Numbers

WAL sequence numbers are monotonically increasing `uint64` values.

A conformant writer must assign a distinct sequence number to every WAL record. Sequence numbers must never be reused and must never reset across sessions for the same file.

Physical record order in the WAL byte stream is authoritative for replay and commit-boundary semantics. Sequence numbers must be strictly increasing in that physical order. Readers must reject a committed WAL prefix containing duplicate or non-increasing sequence numbers.

Sequence numbers validate physical append order; readers must not sort WAL records by sequence during ordinary replay.

## Payload Semantics

For `PUT_NODE`, the payload must be a MessagePack node payload as defined in Sections 1 and 3.

For `PUT_EDGE`, the payload must be a MessagePack edge payload as defined in Sections 1 and 3.

For `DELETE_NODE`, the payload must be a MessagePack map containing the required identity fields `type: string` and `id: string`. Unknown extra fields are tolerated on read and ignored.

For `DELETE_EDGE`, the payload must be a MessagePack map containing the required identity fields `from_node_type: string`, `from_node: string`, `relation: string`, `to_node_type: string`, and `to_node: string`. Unknown extra fields are tolerated on read and ignored.

For `COMMIT`, the payload must be empty.

Readers must decode WAL payloads using the same field rules, required fields, and default application rules used for ordinary node and edge payload decoding.

## Length and Integrity Checks

The `length` field gives the byte length of `payload`. It exists so that truncated or partially written records can be detected precisely.

A conformant reader processing a WAL must verify all of the following for every record:

- the remaining bytes are sufficient for the declared payload length and trailing checksum
- the checksum matches the record contents
- the operation code is defined by this specification

If any of these checks fails, the WAL is invalid. Ordinary read or open behavior must reject the file rather than guessing at writer intent.

Because each record has an independent checksum, corruption of one record does not change the validity rules for other records. Recovery tooling may salvage valid earlier records, but that behavior is outside normal read semantics. Ordinary open must not attempt salvage automatically.

## Replay Semantics

WAL replay follows physical record order in the WAL byte stream.

Replay through the last valid `COMMIT` is part of ordinary open, not a special recovery-only mode.

`COMMIT` records close committed batches. On ordinary open, the implementation must locate the last valid `COMMIT` record and replay all preceding records in physical order up to and including that `COMMIT` boundary, excluding the `COMMIT` record's empty payload itself. The committed prefix must have strictly increasing sequence numbers.

Any records that appear after the last valid `COMMIT` are uncommitted state. A conformant implementation must ignore them during ordinary open rather than apply them.

If no valid `COMMIT` record is present, the WAL contains no committed batch. A conformant implementation must treat the WAL contents as uncommitted state and decline to apply them during ordinary open.

## Lifecycle

Under the append strategy, the WAL accumulates between compactions: a conformant writer must not partially clear it as individual mutations are absorbed into the Data section, so an ordinary committed file may contain a non-empty WAL that remains until compaction writes a fresh file. Under the rewrite strategy, each commit rewrites the Data section to reflect all committed mutations and the WAL is written empty. Either way, a reader replays whatever committed WAL prefix is present; an empty WAL replays to nothing.

During compaction, live data is rewritten into the new file and the old WAL is discarded entirely.

## Automatic Flush Policy

Implementations must provide a policy that prevents unbounded pending mutation or WAL growth.

The exact flush policy is implementation-defined and is not part of AKG file-format conformance. Implementations should document their policy for when pending mutations are committed, flushed, or otherwise made durable.

The recommended safety thresholds are:

- 1,000 pending or uncompacted WAL entries
- 10 MB of pending or uncompacted WAL data

The first threshold reached should control.

This flush policy is a writer-side safety valve. It is not a compaction trigger.

## Durability Boundary

`commit()` is the durability boundary.

A conformant writer may buffer writes in memory before `commit()`. On explicit `commit()`, it must make the committed batch durable and crash-atomic, fsyncing the file state required for recovery. A writer using the append strategy appends the mutation record or records followed by a `COMMIT` record; a writer using the rewrite strategy rewrites the file and atomically renames it into place. `commit()` does not by itself require compaction.

A conformant implementation must call `commit()` automatically on clean close unless the current mutation set has already been committed.

If the process terminates before `commit()`, recent buffered mutations may be lost. That loss is permitted. After a successful `commit()`, the committed mutation set must be recoverable from the file.

## Lifecycle Example

A typical AKG file lifecycle is:

1. create a file with an empty Data section and empty or absent WAL
2. append a `PUT_NODE` and `COMMIT`
3. reopen the file and replay the committed WAL during ordinary open
4. append another mutation but crash before `COMMIT`
5. reopen and ignore the trailing uncommitted WAL tail
6. run compaction to rewrite only current live state, rebuild the Bloom filter, and discard the old WAL
