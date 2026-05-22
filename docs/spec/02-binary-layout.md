# Binary Layout

An AKG file is a single binary container with three top-level parts:

1. a fixed-size header
2. a section table
3. zero or more sections

The header is always first. The section table begins immediately after the header. Every remaining file region, including the WAL when present, is located through the section table. Readers must not infer section locations from convention or physical order.

## File Signature

Bytes 0 through 3 of every AKG file are the magic sequence `AKG\0`.

A conformant reader that does not find this exact four-byte sequence at file offset 0 must reject the file immediately. It must not attempt heuristic parsing or implicit recovery.

## Header

The AKG header is exactly 64 bytes and always occupies file offsets 0 through 63.

All fixed-width binary integer fields in AKG v1 use little-endian encoding unless a field is explicitly defined as a single byte.

Its byte layout is:

- bytes 0-3: magic (`AKG\0`)
- byte 4: major version (`uint8`)
- byte 5: minor version (`uint8`)
- byte 6: checksum algorithm (`uint8`)
- bytes 7-10: section count (`uint32`)
- bytes 11-54: reserved, must be zero
- bytes 55-58: header checksum (`uint32`)
- bytes 59-63: reserved, must be zero

The section table begins at byte offset 64.

Writers must write zero into all reserved header bytes. Readers must not assign format meaning to reserved bytes unless a future version of this specification defines one.

## Version Semantics

The major and minor version fields define the compatibility contract for the file format.

A minor version increase denotes a backward-compatible change. Such changes may add optional fields, add new section types, or add new conventions that older readers can ignore safely.

A major version increase denotes a breaking change. A reader must reject any file whose major version is greater than the highest major version that reader implements.

Unsupported-major-version rejection is mandatory. Silent best-effort parsing is not conformant.

## Checksum Algorithm Identifier

The checksum algorithm byte in the header identifies the section checksum scheme used by the file.

AKG v1 defines one required checksum algorithm:

- `0x01` — CRC32

AKG v1 writers must write `0x01`. AKG v1 readers must reject files that declare any other checksum algorithm value.

The following values are reserved for future compatibility and are not valid AKG v1 checksum algorithms:

- `0x02` — reserved for a future SHA-256 checksum scheme
- `0x03` — reserved for a future BLAKE3 checksum scheme

## Checksum Scope and Failure Policy

AKG uses checksums at two levels:

- the header is checksummed independently
- each section is checksummed independently

The header checksum is always a 4-byte little-endian CRC32 checksum. It covers the header with the checksum field itself excluded from the calculation.

Each section consists of payload bytes followed immediately on disk by that section's checksum bytes. In AKG v1, the section checksum is a 4-byte little-endian CRC32 checksum over the payload bytes only. The `length` field in the section table includes both the payload and the trailing checksum bytes, so a reader that needs the payload alone must subtract the checksum size implied by the selected algorithm.

A checksum failure in either the header or any section is a file integrity failure. A conformant reader must reject the file on any such failure.

Recovery from corruption is outside normal read behavior. Implementations may provide an explicit recovery operation such as `akg.recover()` to salvage readable content from a damaged file, but ordinary readers must fail closed.

## Section Table

The section table immediately follows the header. It contains one entry per section. The number of entries is given by the header's section count field.

Each section table entry has the following structure:

- `type: uint8`
- `offset: uint64`
- `length: uint64`

`offset` is the byte offset of the section from the start of the file. `length` is the total section length in bytes, including the trailing checksum bytes.

The section table is the authoritative description of section locations. Readers must use it to locate sections. They must not assume that sections appear in any particular physical order.

## Section Types

The following section type identifiers are defined:

- `0x01` — Data
- `0x02` — Bloom filter
- `0x03` — WAL

Exactly one Data section is required. At most one Bloom section is allowed. At most one WAL section is allowed.

A reader encountering an unknown section type must skip that section rather than reject the file. Unknown section types may appear multiple times if each section-table entry is otherwise structurally valid. This rule is required for forward compatibility across minor format revisions.

## Section Ordering

The header is the only structure with a fixed location. It is always first and always occupies bytes 0 through 63.

All sections after the section table may appear in any order. Readers must treat the section table, not physical ordering, as the source of truth.

Before trusting any section contents, a conformant reader must validate that every section-table entry lies fully within file bounds, that section ranges do not overlap, and that section cardinality rules are satisfied. A zero-length Data section is invalid. A zero-length Bloom section is invalid. A zero-length WAL section is allowed.

## Bloom Filter Section

The bloom filter is stored in its own section rather than in the header. Its size is variable and scales with the number of keys in the file.

The Bloom section payload has the following wire format:

- `key_count: uint64`
- `bit_count: uint64`
- `hash_function_count: uint8`
- `hash_seed: uint32`
- `bit_array_bytes`

For AKG v1, the parameters are fixed as follows so that all conformant implementations produce identical filters for the same key set:

- hash function: MurmurHash3 x64 128
- bits per key: 10
- `hash_function_count = 7`
- `hash_seed = 0`
- the 7 hash functions are derived by double hashing from the 128-bit output
- the bit array is serialized as raw bytes
- bit ordering within each byte is least-significant-bit first
- `bit_count` is stored explicitly in the section payload

The bloom filter supports fast negative lookups. A bloom-filter miss means the queried key is definitely absent from the indexed key set. A bloom-filter hit means the key may be present and the reader must continue with normal lookup.

The bloom filter is rebuilt from live data during compaction. Between compactions, newly written keys are added and deleted keys may remain represented, which can increase the false-positive rate but does not create false negatives.

## Data Section

The Data section payload is a flat sorted key/value stream. It consists of repeated entries of the form:

- `key_len: uint32`
- `value_len: uint32`
- `key_bytes`
- `value_bytes`

Entries are concatenated directly with no padding and repeated until the end of the Data payload. `value_len = 0` encodes an empty value. Data entries are sorted in required key order: bytewise ascending lexicographic order over the raw UTF-8 key bytes exactly as written. Duplicate keys are invalid.

## WAL Placement

The write-ahead log is represented as a section type, not as an implied trailing byte range outside the section model. When present, it is located through the section table exactly like any other section.

The internal WAL record format and replay rules are defined in Section 5.