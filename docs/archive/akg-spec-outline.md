# AKG Spec — Writing Outline

Each section is a self-contained writing session. Feed the session: this outline entry + the full decisions log + the data model reference. The decisions log is the source of truth — the spec is its prose translation.

---

## Section 0 — `00-introduction.md`

**Title:** Introduction  
**Decisions:** None — draws from the data model reference (Philosophy, What AKG Is Not) and the overall framing of the project.

**Covers:**
- What AKG is: a structured, single-file knowledge graph format for AI agents
- What problem it solves: persistent, portable, inspectable working memory for agents
- Who it's for: SDK authors implementing the format in any language
- Design philosophy: structured over fuzzy, documents over triples, agents name what they want, no embeddings, no vector search
- What AKG is not: not a vector database, not a conversation store, not a replacement for application databases, not an MCP server
- A note on the reference implementation: Go, Phase 1 (format layer), lives alongside the conformance corpus

---

## Section 1 — `01-data-model.md`

**Title:** Data Model  
**Decisions:** 1.1, 1.2, 1.3, 1.4, 1.5, 2.3, 2.4, 2.5

**Covers:**
- Canonical node schema:
  - `type: string` — required
  - `title: string` — required
  - `body: string` — optional, default `""`
  - `meta: map<string, any>` — optional, default `{}`
  - `tags: string[]` — optional, default `[]`
  - `created_at: timestamp` — unix microseconds, `uint64`
  - `updated_at: timestamp` — unix microseconds, `uint64`
  - `version: uint32` — optional, default `1`
- Canonical edge schema:
  - `from_node: string` — required
  - `to_node: string` — required
  - `relation: string` — required
  - `strength: float` — optional, default `0.5`
  - `confidence: float | null` — optional, default `null`
  - `meta: map<string, any>` — optional, default `{}`
  - `created_at: timestamp` — unix microseconds, `uint64`
  - `updated_at: timestamp` — unix microseconds, `uint64`
  - `version: uint32` — optional, default `1`
- Why edges have no `id` field — natural key is `from_node + relation + to_node`
- Edge mutability: `strength`, `confidence`, and `meta` can change after creation
- Versioning: `version: uint32` on both nodes and edges, incremented on every mutation
- The strength/confidence split: what each means, why null confidence is distinct from 0.5
- Type system: format is agnostic, any string is valid, the type registry is an SDK convention not a format requirement
- Default type taxonomy and edge relations (from data model reference) — noted as SDK conventions only

---

## Section 2 — `02-binary-layout.md`

**Title:** Binary Layout  
**Decisions:** 4.1, 4.2, 4.3, 4.4, 4.5, 4.9

**Covers:**
- High-level file structure: header → section table → sections (any order) → WAL
- Magic bytes: `AKG\0` at bytes 0-3, must reject on mismatch
- Header: fixed 64 bytes, always first, contains magic, version, checksum algorithm byte, header checksum
- Version semantics: minor = backwards compatible, major = reject if unsupported
- Checksum algorithm byte: `0x01` CRC32 (default), `0x02` SHA-256 (future), `0x03` BLAKE3 (future)
- Checksum scope: header checksummed (excluding checksum field), every section checksummed independently
- Section checksum structure: each section payload is followed immediately by its checksum bytes; section-table length includes both payload and checksum bytes
- Failure policy: reject on any checksum failure, `akg.recover()` is the explicit rescue path
- Section table: authoritative source of section locations, readers must use it not assume order
- Unknown section types: readers must skip, not reject
- Bloom filter section: variable size, MurmurHash3, 10 bits per key, 7 hash functions — parameters are fixed so all implementations produce identical filters

---

## Section 3 — `03-encoding.md`

**Title:** Encoding  
**Decisions:** 2.1, 2.2, 2.3, 2.4, 2.5

**Covers:**
- MessagePack as the serialization format for all node and edge payloads
- Maps not arrays: field names stored alongside values, rationale (forward compatibility)
- Timestamps: Unix microseconds as plain uint64, used for both header fields and node/edge `created_at`/`updated_at`
- Required fields and defaults: exact list for nodes and edges, reader must apply defaults silently on missing optional fields
- Valid types in `meta`: all MessagePack value types (string, int, float, bool, array, nested map, nil), arbitrary nesting depth
- String encoding: UTF-8 throughout
- A note on MessagePack's dynamic integer sizing vs fixed-width header fields (header fields are fixed-width, MessagePack payload fields use MessagePack's native encoding)

---

## Section 4 — `04-key-layout.md`

**Title:** Key Layout and Index Design  
**Decisions:** 3.1, 3.2, 3.3, 3.4, 3.5, 3.6

**Covers:**
- Core principle: sort order is architecture — indexes are not separate structures, they are the sort order of the data
- Node primary key: `n:{type}:{id}` — all nodes of a type are physically adjacent
- Node ID constraints: opaque, no `:` characters, max 64 characters, unique within file
- Edge primary key: `e:{from_node}:{relation}:{to_node}` — outbound lookup
- Inverted edge index: `ei:{to_node}:{relation}:{from_node}` — written atomically with every edge write
- Tag index: `t:{tag}:{node_id}` — one entry per tag per node
- Tag rules: lowercase, snake_case, max 32 tags, spaces rejected not normalized
- Temporal index: `ts:{timestamp}:{id}`, keyed on `updated_at`, one entry per logical record
- Title prefix index: explicitly dropped and why
- Complete key prefix table
- General principle: fail fast, fail clearly — SDK rejects bad input, never silently corrects

---

## Section 5 — `05-wal.md`

**Title:** Write-Ahead Log  
**Decisions:** 5.1, 5.2, 5.3, 5.4, 5.5

**Covers:**
- Purpose: crash safety — WAL is written before main data structures are mutated
- Five operation types: `PUT_NODE`, `DELETE_NODE`, `PUT_EDGE`, `DELETE_EDGE`, `COMMIT` — with their byte values
- WAL record structure: sequence (uint64), operation (uint8), length (uint32), payload (MessagePack bytes), checksum (CRC32)
- Sequence numbers: monotonically increasing, never reused, never resets across sessions
- Length prefix: enables partial write detection on crash
- Per-record checksum: a corrupted record doesn't poison subsequent records
- `COMMIT` record: marks a consistency point, empty payload
- WAL lifecycle: accumulates between compactions, discarded entirely on compaction
- Safety valve: automatic flush at 1,000 entries or 10MB
- Recovery: replay committed entries up to the last valid `COMMIT` marker and discard any trailing uncommitted records
- fsync semantics: fsync on explicit `commit()`, `commit()` is the durability boundary, called automatically on clean close

---

## Section 6 — `06-compaction.md`

**Title:** Compaction  
**Decisions:** 6.1, 6.2, 6.3

**Covers:**
- Purpose: reclaim space from tombstones and WAL accumulation, rebuild bloom filter
- Trigger: explicit `compact()` call only — no automatic trigger
- Process: write all live keys (tombstones excluded) sorted into a new file → rebuild bloom filter → atomic rename over old file → discard WAL
- Atomic rename: crash safety guarantee — file is always either old version or new version, never a hybrid
- Tombstone handling: tombstones are dropped permanently on compaction, compacted file contains only live records
- Forward note: merge logic (Section 8) must account for the fact that a compacted file carries no record of deletions

---

## Section 7 — `07-error-handling.md`

**Title:** Error Handling and Conformance  
**Decisions:** 4.1, 4.2, 4.3, 5.2, 7.1

**Covers:**
- What a conformant reader must reject: wrong magic bytes, unsupported major version, any checksum failure, truncated WAL record
- What a conformant reader must tolerate: unknown section types (skip), unknown MessagePack fields (ignore), missing optional fields (apply default)
- `akg.recover()`: the explicit rescue path for corrupted files, not normal operation
- Conformance test corpus: lives in the reference implementation repo, is the canonical cross-implementation standard
- Corpus categories: baseline cases, encoding edge cases, format state cases, rejection cases (full list per decision 7.1)
- Round-trip invariant: logical content (nodes, edges, fields) must survive a read-write-read cycle; physical ordering may not

---

## Section 8 — `08-merge.md`

**Title:** Merge Semantics  
**Decisions:** 9.1, 9.2

**Covers:**
- Merge philosophy: the spec defines conflict detection, not conflict resolution
- What constitutes a conflict: same node or edge identity (natural key), differing content
- What must be preserved: both versions, flagged — enough information for any resolution strategy
- Resolution policy is SDK territory: last write wins, caller resolves, consolidation agent — all valid, none mandated
- The consolidation agent: an optional SDK-level concern, not a format requirement
- The tombstone/merge gap: compaction erases tombstones, creating unresolvable ambiguity (absent record = deleted or never existed?)
- A persistent deletion log is the likely solution — design deferred to Phase 2
- This section is a Phase 2 deliverable; placeholder language should note that merge behavior is intentionally underspecified in v1

---

## Appendix — `09-appendix.md`

**Title:** Appendix  
**Decisions:** All

**Covers:**
- Complete key prefix table (pulled from 3.6)
- Canonical node and edge schemas with all fields, types, and defaults (pulled from 2.5)
- WAL record structure (pulled from 5.2)
- Checksum algorithm byte values
- Default type taxonomy and edge relations (SDK conventions)
- Glossary: tombstone, compaction, WAL, bloom filter, natural key, prefix scan, atomic rename

---

## Writing Session Instructions

Each session should receive:
1. This outline (the entry for the section being written)
2. The full `akg-spec-decisions.md`
3. The `akg-data-model-reference.md`

The spec should read as authoritative prose — "the format does X", not "we decided X". Tone: precise, direct, technical but not terse. Closer to the SQLite file format doc or the MessagePack spec than to an RFC. No unnecessary padding, no hedging.
