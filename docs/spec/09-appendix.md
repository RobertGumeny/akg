# Appendix

This appendix collects AKG v1 reference tables and data structures in one place. It is informative in presentation, but each entry reflects normative rules defined in Sections 1 through 8.

## Node Schema

| Field | Type | Required on Write | Read-Time Default if Omitted |
| --- | --- | --- | --- |
| `type` | `string` | yes | none; reader rejects if missing |
| `title` | `string` | yes | none; reader rejects if missing |
| `body` | `string` | no | `""` |
| `meta` | `map<string, any>` | no | `{}` |
| `tags` | `string[]` | no | `[]` |
| `created_at` | `uint64` Unix microseconds | yes | `0` |
| `updated_at` | `uint64` Unix microseconds | yes | `0` |
| `version` | `uint32` | no | `1` |

A node's identifier is not part of the payload. The node-key identity is `n:{type}:{id}`.

## Edge Schema

| Field | Type | Required on Write | Read-Time Default if Omitted |
| --- | --- | --- | --- |
| `from_node_type` | `string` | yes | none; reader rejects if missing |
| `from_node` | `string` | yes | none; reader rejects if missing |
| `to_node_type` | `string` | yes | none; reader rejects if missing |
| `to_node` | `string` | yes | none; reader rejects if missing |
| `relation` | `string` | yes | none; reader rejects if missing |
| `strength` | `float` | no | `0.5` |
| `confidence` | `float | null` | no | `null` |
| `meta` | `map<string, any>` | no | `{}` |
| `created_at` | `uint64` Unix microseconds | yes | `0` |
| `updated_at` | `uint64` Unix microseconds | yes | `0` |
| `version` | `uint32` | no | `1` |

The identity of an edge is the tuple `(from_node_type, from_node, relation, to_node_type, to_node)`. AKG does not define an edge `id` field.

## Key Prefix Table

| Prefix | Key structure | Purpose |
| --- | --- | --- |
| `n:` | `n:{type}:{id}` | Node primary store, grouped by type |
| `e:` | `e:{fromType}:{fromID}:{relation}:{toType}:{toID}` | Outbound edge lookup |
| `ei:` | `ei:{toType}:{toID}:{relation}:{fromType}:{fromID}` | Inbound edge lookup |
| `t:` | `t:{tag}:{node_id}` | Tag inverted index |
| `ts:` | `ts:{timestamp}:n:{type}:{id}` or `ts:{timestamp}:e:{fromType}:{fromID}:{relation}:{toType}:{toID}` | Temporal index keyed on `updated_at` |

## WAL Operation Codes

| Code | Name |
| --- | --- |
| `0x01` | `PUT_NODE` |
| `0x02` | `DELETE_NODE` |
| `0x03` | `PUT_EDGE` |
| `0x04` | `DELETE_EDGE` |
| `0x05` | `COMMIT` |

## WAL Record Structure

A WAL record has the following fields, in order:

- `sequence: uint64`
- `operation: uint8`
- `length: uint32`
- `payload: bytes`
- `checksum: uint32`

All fixed-width integer fields use little-endian encoding. The checksum is a CRC32 over `sequence`, `operation`, `length`, and `payload`.

`COMMIT` closes a committed WAL batch. On ordinary open, implementations replay all records up to the last valid `COMMIT` and ignore any trailing records after that boundary.

## Checksum Algorithm Codes

| Code | Algorithm |
| --- | --- |
| `0x01` | CRC32 |
| `0x02` | SHA-256 |
| `0x03` | BLAKE3 |

CRC32 is the default algorithm for AKG v1.

Each section's payload is followed immediately by that section's checksum bytes on disk. The checksum covers the payload bytes only, and the section-table `length` includes both payload and checksum bytes.

## Data Section Entry Structure

The Data section payload is a repeated flat key/value entry stream:

- `key_len: uint32`
- `value_len: uint32`
- `key_bytes`
- `value_bytes`

Entries are concatenated with no padding. Empty values are encoded with `value_len = 0`. Entries are sorted by bytewise ascending lexicographic order over the raw UTF-8 key bytes. Duplicate keys are invalid.

## Bloom Section Wire Format

The Bloom section payload is:

- `key_count: uint64`
- `bit_count: uint64`
- `hash_function_count: uint8` (`7` in v1)
- `hash_seed: uint32` (`0` in v1)
- `bit_array_bytes`

The bit array is serialized as raw bytes with least-significant-bit-first ordering within each byte. AKG v1 uses MurmurHash3 x64 128 with 7 functions derived by double hashing.

## WAL Delete Payload Shapes

- `DELETE_NODE` payload: MessagePack map with required `type: string` and `id: string`
- `DELETE_EDGE` payload: MessagePack map with required `from_node_type: string`, `from_node: string`, `relation: string`, `to_node_type: string`, and `to_node: string`

Unknown extra fields in these delete payloads are tolerated on read and ignored.

## Container Invariants Summary

| Invariant | Rule |
| --- | --- |
| Magic bytes | bytes 0-3 are `AKG\0` |
| Header size | exactly 64 bytes |
| Integer endianness | all fixed-width binary integers are little-endian |
| Section table placement | begins immediately after the header |
| Data section cardinality | exactly one required |
| Bloom section cardinality | at most one |
| WAL section cardinality | at most one |
| Unknown sections | allowed and skipped if structurally valid |
| Section ranges | must be fully in bounds and non-overlapping |
| Zero-length sections | Data invalid, Bloom invalid, WAL allowed |
| Data entry structure | repeated `uint32 key_len`, `uint32 value_len`, key bytes, value bytes |
| Data ordering | keys sorted by raw UTF-8 bytewise lexicographic order |
| Duplicate Data keys | invalid |
| WAL replay | ordinary open replays through the last valid `COMMIT` |
| Trailing WAL tail | uncommitted records after last valid `COMMIT` are ignored on ordinary open |
| Compaction result | rewrites live state, rebuilds Bloom, discards old WAL |
| Ordinary-open failures | reject corruption, malformed known sections, and section-table violations |

## Default SDK Taxonomy Conventions

AKG does not require a fixed node-type taxonomy or relation vocabulary. Implementations may nevertheless publish defaults for interoperability.

Typical default node-type conventions include values such as:

- `entity`
- `decision`
- `preference`
- `task`
- `fact`
- `note`

Typical default relation conventions include values such as:

- `depends_on`
- `supports`
- `contradicts`
- `mentions`
- `caused_by`
- `related_to`

These are SDK conventions only. Any UTF-8 string remains valid for `type` and `relation` at the format level.

## Glossary

**atomic rename** — replacement of one file path with another in a single filesystem operation, so readers observe either the old file or the new file, never a mixed state.

**bloom filter** — probabilistic membership structure used for fast negative key lookups. A miss is definitive; a hit is only a possible match.

**compaction** — whole-file rewrite that keeps only live records, rebuilds derived sections, and discards tombstones and WAL history.

**natural key** — identity derived from meaningful record fields rather than from a separate synthetic identifier. In AKG, edge identity is `(from_node_type, from_node, relation, to_node_type, to_node)`.

**prefix scan** — lookup over a sorted key space using a shared leading string such as `n:decision:` or `t:auth:`.

**tombstone** — deletion marker indicating that a previously existing logical record has been removed.

**WAL** — write-ahead log. Ordered mutation log used to provide crash recovery and durable commit boundaries.
