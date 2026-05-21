# Compaction

Compaction rewrites an AKG file into a new equivalent file containing only live records. Its purpose is to reclaim space consumed by tombstones and accumulated WAL state, and to rebuild derived structures such as the bloom filter from the current live key set.

Compaction is a whole-file operation. It does not modify the existing file in place.

## Purpose

Over time, an AKG file accumulates tombstones from deletions and WAL records from committed mutations. These are necessary for normal write and recovery behavior, but they are not part of the minimal steady-state representation of the graph.

Compaction produces that steady-state representation:

- live keys are rewritten into a fresh sorted file
- tombstoned records are omitted
- the bloom filter is rebuilt from the live key set
- the existing WAL is discarded

The logical graph content after a successful compaction must be the same as the logical graph content before compaction, excluding the removal of obsolete tombstones and WAL history.

## Trigger

Compaction is explicit only.

A conformant implementation must run compaction only when the caller explicitly requests it, for example through `compact()`.

The WAL flush thresholds defined in Section 5 are not compaction triggers. Reaching 1,000 uncompacted WAL entries or 10 MB of WAL data requires a flush, but does not require compaction.

## Compaction Process

A conformant compaction implementation must perform the following steps:

1. perform ordinary open semantics on the current file, including replay of committed WAL through the last valid `COMMIT`
2. materialize the complete live key set
3. exclude tombstoned records
4. write all live keys into a new AKG file in sorted-key form
5. rebuild the bloom filter from the live key set
6. write the new file with a fresh section table and no carried-forward WAL contents
7. atomically rename the new file over the old file

The compacted file must contain only the current live representation of the graph. It must not preserve superseded records, tombstones, or the previous WAL section.

## Sorted Output Requirement

The rewritten Data section produced by compaction must preserve the AKG key-layout rules defined in Section 4.

All live keys written into the new file must appear in sorted order and must include all required primary and index entries for the logical records they represent.

Compaction is not permitted to change logical identity, omit required index keys, or rewrite records into an alternative key layout.

## Bloom Filter Rebuild

Compaction must rebuild the bloom filter from scratch using the live key set written into the compacted file.

The rebuilt bloom filter must use the format-defined parameters from Section 2:

- MurmurHash3 x64 128
- 10 bits per key
- 7 hash functions derived by double hashing
- hash seed `0`
- least-significant-bit-first bit ordering within each byte

A conformant implementation must not copy the previous bloom filter section forward unchanged.

## Atomic Replacement

Compaction must replace the old file by atomic rename of the newly written file.

This replacement rule is the compaction crash-safety guarantee. If a crash occurs before the rename completes, the old file remains the valid file. If the rename completes, the new compacted file becomes the valid file. A conformant implementation must not expose a partially rewritten hybrid of the two.

## Tombstone Handling

Tombstones are dropped permanently during compaction.

A compacted AKG file contains only live records and the index entries derived from them. It does not contain a retained deletion history.

This rule applies equally to node tombstones and edge tombstones.

## WAL Handling

The WAL is discarded during compaction.

A compacted file must not retain the prior WAL section. Any mutations represented in that WAL that are part of the live graph state must be reflected in the rewritten live keys of the compacted file instead.

After successful compaction, the file's durable state is represented entirely by the newly written sections. The resulting compacted file contains no carried-forward WAL history.

## Forward Constraint on Merge

Because compaction removes tombstones, a compacted file does not preserve evidence that a now-absent record was explicitly deleted.

Merge logic defined in Section 8 must account for this property. In particular, absence from a compacted file is not sufficient to distinguish deletion from nonexistence.
