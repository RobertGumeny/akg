# AKG Reference Implementation Plan

## Goal

Build a tiny reference implementation for the AKG format layer.

It should prove that AKG v1 can be implemented clearly, correctly, and with minimal machinery.

The implementation scope is intentionally narrow:

- create AKG files
- apply the four core mutation actions:
  - `PutNode`
  - `PutEdge`
  - `DeleteNode`
  - `DeleteEdge`
- read current logical state
- validate files and records
- open files with ordinary committed-WAL application
- compact files
- produce and grow a conformance corpus

## Non-Goals

Do not build:

- a product SDK surface beyond the minimal reference API
- a query language
- a query planner
- a traversal engine
- a merge engine
- a server
- an MCP integration
- background services
- pluggable storage backends
- policy-level graph constraints such as referential integrity
- automatic salvage during ordinary reads

The reference implementation exists to exercise the format, not to become a full database.

## Design Rules

Keep the implementation painfully simple.

Rules:

- single-process, single-writer design
- plain structs and plain functions
- current-state-oriented design
- explicit validation
- explicit errors
- full-record replacement only
- no patch semantics
- no speculative abstractions
- no optimization work unless the v1 spec requires it
- no support for future versions beyond what the v1 spec needs

If something exists only because it might be useful later, cut it.

## MVP Architecture

The MVP should be organized around one idea:

- the authoritative in-memory state is only nodes plus edges

Everything else is derived from that authoritative in-memory state when needed.

That means:

- `ei:` entries are derived secondary index entries
- `t:` entries are derived secondary index entries
- `ts:` entries are derived secondary index entries
- the bloom filter is derived from the written key set
- the Data section contains only current live state
- committed mutations may also remain represented in the WAL until compaction
- delete intent lives in WAL semantics, not as tombstones in the Data section

Reads must expose only current logical state. They must never expose:

- stale superseded records
- tombstones
- partial WAL batches
- raw WAL internals

## Minimal Implementation Shape

Suggested package layout:

- `format/`
  - file header
  - section table
  - checksums
  - Data-section KV encoding
- `record/`
  - node and edge structs
  - validation
  - MessagePack encode/decode
- `keys/`
  - key builders/parsers
- `wal/`
  - append
  - commit
  - replay through last valid commit
- `state/`
  - authoritative in-memory nodes + edges
  - apply mutations
  - derive secondary keys
- `store/`
  - sorted KV read/write helpers
  - optional bloom filter helpers
- `cmd/akg/`
  - tiny CLI

This should stay small. Do not add layers unless a real need appears.

## Core Semantics

### Mutation model

The four core mutation actions are first-class and must stay central everywhere in the implementation:

- `PutNode`
- `PutEdge`
- `DeleteNode`
- `DeleteEdge`

Semantics:

- `PutNode` is an upsert by `(type, id)`
- if `PutNode` omits `id`, the writer generates one
- generated node IDs should be 16 random hex characters
- caller-provided IDs are allowed if they satisfy format constraints
- changing node `type` changes node identity; it is not an in-place mutation
- `PutEdge` is an upsert by `(from_node, relation, to_node)`
- deletes are strict, not idempotent
- deleting a missing node or edge returns not-found

### Writer-owned fields

In MVP, the writer owns:

- `created_at`
- `updated_at`
- `version`

There is no import/restore mode in MVP.

### Validation and tolerance

- duplicate tags are rejected
- unknown MessagePack fields are tolerated on read
- unknown MessagePack fields are dropped on rewrite
- validator enforces format correctness, not graph policy
- dangling edges are allowed
- no cascade deletes

## On-Disk Data Shape

The Data section should be a flat sorted KV entry file.

Repeated entry format:

- `key_len: uint32`
- `value_len: uint32`
- `key_bytes`
- `value_bytes`

Rules:

- `key_len` and `value_len` are little-endian `uint32`
- entries are sorted by raw UTF-8 bytewise lexicographic key order
- a valid Data section contains no duplicate keys
- the Data section contains only current live state
- the Data section contains no tombstones
- entries are concatenated with no padding
- empty values are encoded with `value_len = 0`

Value storage decisions:

- `n:{type}:{id}` → full node payload
- `e:{from}:{relation}:{to}` → full edge payload
- `ei:{to}:{relation}:{from}` → empty value
- `t:{tag}:{node_id}` → empty value
- `ts:{timestamp}:n:{type}:{id}` → empty value
- `ts:{timestamp}:e:{from}:{relation}:{to}` → empty value

Temporal keys must stay explicit and self-describing exactly as above.

## Open, Commit, and Compaction Semantics

### Ordinary open

Ordinary open must automatically apply valid committed WAL through the last valid `COMMIT`.

That is normal behavior, not a special recovery mode.

Ordinary open must:

- validate the file structure
- read the Data section as current state
- apply WAL records through the last valid committed boundary
- ignore trailing uncommitted WAL records after the last valid `COMMIT`
- expose only the resulting current logical state

Explicit salvage or recovery for corrupted files remains separate.

### Ordinary commit

Use the accumulating-WAL strategy defined by the spec:

1. mutate authoritative in-memory state
2. append WAL mutation records
3. append `COMMIT`
4. fsync
5. leave the committed WAL in place for future ordinary open replay until compaction

Ordinary commit must not rewrite the full file or reset the WAL.

### Compaction

Compaction is the explicit whole-file rewrite path:

- read current logical state
- write a fresh full AKG file from that state
- rebuild derived indexes and bloom filter
- atomically replace the old file
- produce a file with no carried-forward WAL history

Do not attempt in-place compaction.

## Read API Direction

Read helpers should stay small and explicit.

Allowed direction:

- exact node lookup
- exact edge lookup
- outbound edge listing
- inbound edge listing
- tag lookup

`ListNodesByTag` may return node references or identities instead of hydrated nodes.

Do not build:

- a general query language
- a planner
- arbitrary traversal operators

## Bloom Filter

If bloom filter support is required by v1, implement it minimally.

That means:

- fixed spec parameters only
- build from written key set
- read and test membership
- serialize and deserialize
- treat it as an optional negative-lookup optimization
- do not make correctness depend on it

## Milestones

### 1. Records and mutation semantics

Implement:

- node and edge structs
- identity helpers
- required-field validation
- default application on read
- MessagePack encode/decode
- `PutNode`/`PutEdge` upsert semantics
- `DeleteNode`/`DeleteEdge` strict not-found semantics
- writer-owned field behavior
- 16-hex-character node ID generation

This milestone should lock down the logical semantics first.

### 2. Keys and Data-section KV encoding

Implement:

- node keys
- edge keys
- inbound edge keys
- tag keys
- explicit temporal keys
- flat sorted KV entry writer/reader
- input validation for key components
- duplicate-key rejection in Data validation

This milestone should make the on-disk current-state model concrete.

### 3. File format

Implement:

- header read/write
- section table read/write
- checksum calculation and verification
- Data section read/write
- section validation

Keep the reader and writer straightforward.

### 4. Authoritative in-memory state and derived indexes

Implement:

- authoritative in-memory nodes + edges only
- mutation application to in-memory state
- derived generation of `ei:`, `t:`, and `ts:` entries when writing files
- current-state materialization helpers

Do not maintain derived indexes as separate authoritative mutable structures.

### 5. WAL and ordinary open semantics

Implement:

- append WAL records for all four mutation actions
- write `COMMIT` records
- replay through the last valid `COMMIT`
- strict handling of truncation and checksum failure
- ordinary open that automatically applies committed WAL
- explicit separation between ordinary open and salvage/recovery tooling

### 6. Ordinary commit path

Implement the normal write path as:

- validate input
- mutate authoritative in-memory state
- append WAL entry or entries
- append `COMMIT`
- fsync
- leave committed WAL state in place until explicit compaction

No transaction framework beyond what the spec requires.

### 7. Minimal read helpers

Implement:

- open file into current logical state
- exact lookup helpers
- outbound edge listing
- inbound edge listing
- tag lookup

Ensure reads never surface tombstones, stale versions, or WAL internals.

### 8. Compaction

Implement compaction as:

- load current logical state
- write a fresh file from live state only
- rebuild derived indexes
- rebuild bloom filter if present
- atomically replace the old file

### 9. CLI

Keep the CLI tiny.

Commands:

- `akg inspect <file>`
- `akg validate <file>`
- `akg compact <file>`

Do not add mutation or query-heavy CLI features unless they are needed to validate the format work.

### 10. Conformance corpus

Build the corpus incrementally across milestones, not only at the end.

Add fixtures for:

- record encoding/decoding
- key layout
- delete semantics
- WAL commit boundaries
- ordinary open with committed WAL application
- accumulating WAL behavior between compactions
- compaction behavior
- rejection cases

## Testing Priorities

Focus on tests that make the finalized MVP semantics concrete.

Required test areas:

- valid node decoding
- valid edge decoding
- missing required fields
- optional field defaults
- unknown MessagePack field tolerance on read
- unknown MessagePack field dropping on rewrite
- invalid key components
- duplicate tag rejection
- duplicate key rejection in Data validation
- node ID generation length and format
- `PutNode` upsert by `(type, id)`
- `PutEdge` upsert by `(from_node, relation, to_node)`
- strict delete-not-found behavior for nodes and edges
- node `type` change treated as identity change
- checksum verification
- truncated WAL handling
- replay to last valid `COMMIT`
- ordinary open applying committed WAL automatically
- ignoring trailing uncommitted WAL records
- Data section containing only current live state
- derived index regeneration from authoritative in-memory state
- compaction preserving current logical state
- dangling-edge tolerance
- bloom filter correctness as an optimization aid, not a correctness dependency

Use golden files where useful. Grow the conformance corpus as each milestone lands.

## Success Criteria

The reference implementation is successful if it:

- proves the v1 spec can be implemented cleanly
- reflects the finalized MVP architecture without extra layers
- makes deletes first-class across semantics, implementation, and tests
- demonstrates a current-state-oriented file representation
- demonstrates ordinary open with committed-WAL application through the last valid `COMMIT`
- uses accumulating WAL for ordinary commits and whole-file replacement only for compaction
- keeps derived indexes non-authoritative
- produces an incrementally grown conformance corpus other implementations can use

It does not need to be feature-rich. It needs to be clear, small, and correct.
