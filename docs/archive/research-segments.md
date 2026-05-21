Top 3 research segments

1) Embedded Storage Engines & Binary File Format Design

Why this matters:
This is the backbone of AKG. Your spec is really defining a tiny embedded database format: header, section table, sorted KV data section, checksums, indexes-as-key-layout, bloom filter, forward compatibility.

Segment brief to initialize:
“Research how embedded storage engines and binary file formats are designed for portability, inspectability, forward compatibility, and efficient point/prefix lookups. Focus on sorted keyspaces, sectioned file containers, data/index co-design, binary encoding choices, and how real systems specify deterministic on-disk layouts.”

Expert profile:
Use a specialized profile if possible: “storage engine / database internals engineer”.
Default researcher is okay, but this one is worth upgrading.

Suggested threads:
- SQLite file format and b-tree/page design
- SSTables / LSM-style sorted-string storage
- Bitcask, BoltDB, RocksDB, LMDB, Pebble/Badger comparisons
- Binary file format design patterns: headers, versioning, endianness, checksums
- MessagePack vs protobuf/cbor for long-lived storage
- Prefix indexes and lexicographic key design
- Bloom filters in storage engines

2) Crash Consistency, WALs, fsync, and Atomic File Replacement

Why this matters:
This is the most important low-level systems segment for confidence. Your spec leans hard on WAL semantics, checksums, commit boundaries, and atomic rename. If you really understand this area, a lot of the rest becomes much easier to reason about.

Segment brief to initialize:
“Research crash consistency for single-file embedded databases, including WAL design, commit boundaries, fsync behavior, truncation detection, checksum strategy, atomic rename, and differences between ordinary open, recovery, and salvage paths. Emphasize practical filesystem realities, not just idealized models.”

Expert profile:
Definitely use a special profile: “filesystem / database reliability engineer”.

Suggested threads:
- SQLite rollback journal and WAL design
- ARIES-style WAL concepts at a practical level
- Linux/macOS/Windows fsync semantics
- Atomic rename guarantees and caveats
- Torn writes, partial writes, and corruption detection
- Checksums vs cryptographic hashes in storage systems
- Single-writer embedded DB reliability patterns

3) Merge, Tombstones, Deletion History, and Sync Semantics

Why this matters:
This is the biggest conceptual gap in the current spec. The merge section correctly admits that compaction destroys
deletion history, which makes absence ambiguous. That is not a small edge case — it’s the central obstacle to
multi-writer sync/merge.

Segment brief to initialize:
“Research how replicated or mergeable data systems preserve deletion intent and resolve conflicts across compacted histories. Focus on tombstones, persistent deletion logs, causal metadata, conflict preservation, and the tradeoffs
between simple last-write-wins, revision trees, event logs, and CRDT-style approaches.”

Expert profile:
Use a special profile: “distributed systems / sync engine engineer”.

Suggested threads:
- LSM tombstones and compaction semantics
- CouchDB/PouchDB revision trees and conflict handling
- CRDT sets/maps and deletion semantics
- Vector clocks / Lamport clocks / causal metadata
- Event sourcing vs current-state snapshots
- Sync engines for local-first software
- How Git-like conflict preservation differs from database merge