# AKG Spec Readiness Checklist

This checklist captures the most important spec gaps to resolve before starting the reference implementation.

## Priority 0 — Must resolve before coding

### 1. Pick one write model and make it normative
**Status:** Resolved

**Decision:** AKG v1 uses a conventional accumulating WAL between compactions.

**Normative direction:**
- ordinary commits append WAL records and `COMMIT`
- ordinary open replays committed WAL through the last valid `COMMIT`
- WAL normally remains non-empty between compactions
- compaction is explicit, rewrites current live state, and discards prior WAL
- v1 does **not** rewrite the full file on every commit

**Files to update/already affected:**
- `akg-reference-implementation-plan.md`
- `spec/05-wal.md`
- `spec/06-compaction.md`
- `spec/02-binary-layout.md`
- `spec/07-error-handling.md`

**Why this mattered:**
This sets file lifecycle, open semantics, commit semantics, compaction behavior, test expectations, and CLI behavior.

---

### 2. Make the Data section binary format normative
**Status:** Resolved

**Decision:** The Data section is a flat sorted KV stream with the following repeated entry structure:
- `key_len: uint32`
- `value_len: uint32`
- `key_bytes`
- `value_bytes`
- repeated until the end of the Data payload

**Locked rules:**
- `key_len` and `value_len` are `uint32`
- entries are concatenated directly with no padding
- empty values are encoded as `value_len = 0`
- entries are sorted by canonical key order
- duplicate keys are invalid

**Files to update/already affected:**
- `spec/02-binary-layout.md`
- `spec/04-key-layout.md`
- `spec/09-appendix.md`

**Why this mattered:**
There is no interoperable reader/writer without a normative Data section layout.

---

### 3. Define endianness for all fixed-width binary fields
**Status:** Resolved

**Decision:** All fixed-width binary integer fields in AKG v1 use little-endian encoding.

**Applies to:**
- header fields
- section table fields
- Data section length fields
- WAL fixed-width fields
- checksum field byte encoding

**Files to update/already affected:**
- `spec/02-binary-layout.md`
- `spec/05-wal.md`
- `spec/09-appendix.md`

**Why this mattered:**
Without endianness, independent implementations can produce incompatible files.

---

### 4. Fully specify the bloom filter section format
**Status:** Resolved

**Decision:** The Bloom section has an explicit deterministic wire format.

**Locked format:**
- `key_count: uint64`
- `bit_count: uint64`
- `hash_function_count: uint8` (`7` in v1)
- `hash_seed: uint32` (`0` in v1)
- `bit_array_bytes`

**Locked rules:**
- bit array is serialized as raw bytes
- bit ordering within each byte is least-significant-bit first
- hash function is MurmurHash3 x64 128
- the 7 hash functions are derived by double hashing from the 128-bit output
- bits-per-key remains `10` as already decided; `bit_count` is stored explicitly in the section payload

**Files to update/already affected:**
- `spec/02-binary-layout.md`
- `spec/09-appendix.md`

**Why this mattered:**
If the bloom filter is part of conformance, its byte-level representation must be deterministic.

---

### 5. Resolve the temporal key inconsistency
**Status:** Resolved

**Decision:** Temporal keys are self-describing and include record kind plus full logical identity.

**Canonical forms:**
- node: `ts:{timestamp}:n:{type}:{id}`
- edge: `ts:{timestamp}:e:{from}:{relation}:{to}`

**Rationale:**
- avoids collisions
- preserves full identity in the key itself
- makes temporal scans interpretable without extra lookup context
- guarantees uniqueness even when records share timestamps

**Files to update/already affected:**
- `akg-spec-decisions.md`
- `akg-reference-implementation-plan.md`
- `spec/04-key-layout.md`
- `spec/09-appendix.md`

**Why this mattered:**
This affects derived key generation, scans, fixtures, and conformance expectations.

---

### 6. Add section validation and cardinality rules
**Status:** Resolved

**Decision:** Section validation and cardinality rules are part of ordinary structural validation.

**Locked rules:**
- exactly one Data section is required
- at most one Bloom section is allowed
- at most one WAL section is allowed
- unknown section types are allowed and may appear multiple times if structurally valid
- section ranges must not overlap
- every section offset/length range must lie fully within file bounds
- zero-length Data sections are invalid
- zero-length Bloom sections are invalid
- zero-length WAL sections are allowed
- readers must validate section-table entries for bounds, overlap, and cardinality before trusting section contents

**Files to update/already affected:**
- `spec/02-binary-layout.md`
- `spec/07-error-handling.md`
- `spec/09-appendix.md`

**Why this mattered:**
Reader validation behavior will otherwise vary a lot between implementations.

## Priority 1 — Should tighten before implementation starts in earnest

### 7. Align the WAL spec with the chosen MVP lifecycle
**Status:** Resolved

**Decision:** The WAL lifecycle follows the accumulating-WAL model chosen in item 1.

**Locked clarifications:**
- WAL replay is part of ordinary steady-state open, not just special crash recovery
- ordinary committed operation may and often will leave a non-empty WAL behind
- `commit()` durably appends mutation records plus `COMMIT`; it does not imply immediate whole-file rewrite
- compaction is explicit and is the operation that rewrites live state into a fresh file and discards the WAL
- WAL flush thresholds remain relevant as guardrails against unbounded WAL growth, not as evidence that AKG is a high-throughput database

**Files to update/already affected:**
- `spec/05-wal.md`
- `akg-reference-implementation-plan.md`
- `akg-comprehensive-test-plan.md`

---

### 8. Make node and edge identity encoding fully precise in WAL delete payloads
**Status:** Resolved

**Decision:** WAL delete payloads are MessagePack-encoded maps with precise identity-only shapes.

**Locked payload shapes:**
- `DELETE_NODE` → MessagePack map with required fields:
  - `type: string`
  - `id: string`
- `DELETE_EDGE` → MessagePack map with required fields:
  - `from_node: string`
  - `relation: string`
  - `to_node: string`

**Locked rules:**
- identity fields listed above are required
- unknown extra fields are tolerated on read and ignored
- delete payloads carry identity only, not full record bodies

**Files to update/already affected:**
- `spec/05-wal.md`
- `spec/09-appendix.md`

---

### 9. State explicit rules for key sorting
**Status:** Resolved

**Decision:** Keys are sorted by raw UTF-8 byte sequence using bytewise ascending lexicographic order.

**Locked rules:**
- comparison is performed on the encoded key bytes exactly as written
- locale rules do not apply
- Unicode normalization does not apply
- case folding does not apply
- the bytewise sorting rule is what produces the desired prefix-grouping behavior for scans

**Files to update/already affected:**
- `spec/04-key-layout.md`
- `spec/02-binary-layout.md`

---

### 10. Clarify what ordinary validation checks versus salvage/recovery checks
**Status:** Resolved

**Decision:** Ordinary open is strict and recovery/salvage is explicit and separate.

**Locked ordinary-open behavior:**
- reject bad magic
- reject unsupported major version
- reject bad header checksum
- reject bad section checksum
- reject invalid section table structure, including overlap, out-of-bounds ranges, and cardinality violations
- reject malformed Data sections
- reject malformed Bloom sections
- reject malformed or truncated WAL content
- reject invalid WAL record structure
- reject invalid MessagePack payloads when required fields or required field types are wrong

**Locked ordinary-open tolerances:**
- tolerate unknown MessagePack fields and ignore them
- tolerate unknown section types if structurally valid
- ignore trailing uncommitted WAL records after the last valid `COMMIT`

**Locked recovery boundary:**
- salvage/recovery is available only through an explicit tool or API
- ordinary open must not attempt salvage automatically

**Files to update/already affected:**
- `spec/07-error-handling.md`
- `spec/05-wal.md`
- `akg-reference-implementation-plan.md`

## Priority 2 — Nice to clean up now so tests and docs don’t drift

### 11. Reconcile the spec and the test plan
**Status:** Resolved as follow-up alignment work

**Decision:** The test plan must be brought into explicit alignment with the reconciled normative spec.

**Locked follow-up requirements:**
- conformance tests must depend only on normative spec text
- remove assumptions that reflect rewrite-on-every-commit behavior
- update tests to reflect accumulating WAL semantics
- update tests to reflect self-describing temporal keys
- update tests to reflect the normative Data-section wire format
- update tests to reflect little-endian fixed-width field encoding
- update tests to reflect the Bloom-section wire format
- update tests to reflect strict section/cardinality validation rules
- keep higher-level SDK/API tests separate from pure format conformance tests

**Files to update/already affected:**
- `akg-comprehensive-test-plan.md`
- all `spec/*.md`

---

### 12. Add a concise file-lifecycle example
**Status:** Resolved as documentation follow-up

**Decision:** Add a short worked lifecycle example spanning create, commit, reopen, crash-before-commit, and compaction.

**Locked example shape:**
- create empty file
- write `PutNode`
- `commit()`
- reopen and replay committed WAL
- write another mutation and crash before `COMMIT`
- reopen and ignore trailing uncommitted WAL
- run `compact()`
- resulting file contains only current live state, rebuilt Bloom filter, and no carried-forward WAL

**Files to update/already affected:**
- `spec/05-wal.md`
- `spec/06-compaction.md`
- maybe `spec/00-introduction.md`

---

### 13. Add one normative summary table for container invariants
**Status:** Resolved as documentation follow-up

**Decision:** Add a compact normative summary table covering AKG container invariants.

**Locked summary scope:**
- magic bytes
- header size
- endian rule
- section types
- section cardinality rules
- section overlap/bounds rules
- Data-section entry structure
- key sorting rule
- WAL replay rule
- compaction result rule
- unknown section handling
- ordinary-open rejection rules at a high level

**Files to update/already affected:**
- `spec/09-appendix.md`

**Suggested contents:**
- exactly one header
- section table immediately after header
- section lengths include trailing checksum bytes
- unknown sections skipped
- known-section cardinality rules
- fixed-width integer endianness
- sorted Data entries
- duplicate Data keys invalid

## Suggested execution order

1. Resolve the single write/WAL model
2. Define Data section wire format
3. Define endianness everywhere
4. Define bloom section format
5. Fix temporal key shape
6. Add section/cardinality validation rules
7. Reconcile WAL prose, tests, and appendices with the above

## Bottom line

The spec is close, but not yet implementation-tight. The main blocker is not lack of ideas; it is that a few byte-level and lifecycle-level rules still need to be made fully normative and internally consistent.