# Merge Semantics

AKG v1 does not define a complete on-disk merge protocol.

This section defines the minimum semantic contract for conflict detection and for the preservation of conflicting graph state when an implementation chooses to merge AKG files or logical AKG states. It does not define a mandatory conflict-resolution policy, and it does not define a v1-native persistent representation for unresolved conflicts.

Merge is therefore an SDK-level operation built on top of the format, not a required part of ordinary AKG read or write behavior.

## Scope

A conformant AKG reader is not required to implement merge.

A conformant AKG writer is not required to emit merge metadata, conflict markers, or any other merge-specific structure in AKG v1.

An implementation that exposes merge behavior must follow the rules in this section.

## Identity Basis

Merge comparison is defined over AKG record identity.

For nodes, identity is the node-key identity defined by `n:{type}:{id}`.

For edges, identity is the natural key `(from_node_type, from_node, relation, to_node_type, to_node)`.

Two records with different identities are not in conflict merely because their payload contents are similar or identical.

## Conflict Detection

Conflict detection in AKG v1 is based on the scalar `version` counter, not on timestamps. Wall-clock timestamps are not reliable across independent devices and must not be used to determine ordering; an implementation may use them only as input to its own resolution policy (see Resolution Policy). Logical content is evaluated after normal AKG decoding — MessagePack map decoding, application of all defined read-time defaults for omitted optional fields, and interpretation per Sections 1 and 3.

For two candidate records with the same identity:

- **Different `version`:** the higher-versioned record is treated as the presumed successor (a fast-forward) and supersedes the lower-versioned one. This is not a conflict.
- **Same `version`, different logical content:** the records have diverged from a common base and are in **conflict**. Both must be preserved (see Required Preservation of Conflicting State).
- **Same `version`, same logical content:** the records are equivalent; not a conflict.

**Known limitation (scalar version).** A scalar counter cannot distinguish a true successor from a concurrent edit that happens to carry a higher version — for example, two replicas diverging from a common base and making unequal numbers of subsequent edits. In that case the lower-versioned concurrent edit is silently superseded rather than reported as a conflict. Reliable concurrency detection requires per-record causal metadata (a version vector), which AKG v1 does not define; it may be added as an optional, additive structure in a future minor version without invalidating existing files.

## Required Preservation of Conflicting State

A merge implementation must not silently discard one side of a detected conflict merely because both records share the same identity.

When a conflict is detected, the implementation must preserve enough information to make both conflicting versions available to the resolution layer. At minimum, this preserved information must include:

- the shared record identity
- both conflicting logical record versions
- an explicit indication that the identity is conflicted

AKG v1 does not require a particular in-memory API shape or on-disk encoding for this preserved conflict state. It does require that the state be preserved rather than collapsed implicitly.

## Resolution Policy

AKG does not define a mandatory conflict-resolution policy.

Policies such as last-write-wins, caller-directed resolution, or resolution by a consolidation agent are all permitted. None is required by the format.

If an implementation applies an automatic resolution policy, that policy is SDK behavior, not AKG format behavior.

A conformant implementation must document any automatic resolution policy it applies.

## Consolidation Agents

A consolidation agent is an optional SDK-level component that inspects preserved conflicts and produces a resolved graph state.

AKG does not require such an agent. The format defines neither its interface nor its decision procedure.

## Absence and Deletion Ambiguity

Compaction removes tombstones permanently, as defined in Section 6.

As a result, absence of a record from a compacted AKG file is not sufficient evidence that the record was deleted. The record may instead never have existed in that file's history, or it may have been deleted before compaction erased the tombstone.

A merge implementation must not infer a definite deletion solely from absence in a compacted input file. The conformant v1 default is therefore a **union-biased merge**: a record present on either side is kept, and a deletion does not propagate across a compacted boundary. Today's delete is effectively an eviction that a later merge may resurrect; making a deletion durable across independently compacted files requires the deletion log deferred below.

If correct deletion-aware merge requires distinguishing deletion from nonexistence, the implementation must rely on information outside the compacted live-record set. In AKG v1, this information is not standardized.

## Deferred Deletion History

A persistent deletion log that survives compaction is the expected mechanism for resolving the absence ambiguity described above.

AKG v1 does not define that mechanism. Its design is deferred to a future version.

Until such a mechanism is standardized, deletion-aware merge across independently compacted files is inherently incomplete.

When it is defined, its format surface is intended to be minimal: a deletion record needs only the record identity (key) and the deletion `version` — never the deleted payload. If opaque deletion entries are wanted, so that the log does not expose which identities were removed, the keys may be hashed with a format-fixed hash, exactly as the bloom filter fixes MurmurHash3 for cross-implementation determinism. Indexing, retention and erasure policy, and techniques such as crypto-shredding are implementation and deployment concerns, not part of the format.

## v1 Status

Merge in AKG v1 is deliberately minimal: the format defines conflict *detection* and the *preservation* of conflicting state, and leaves *resolution* to the implementer — a consolidation layer or a managed service above the format. This is a settled scope decision, not a placeholder.

This specification defines only:

- how identity is determined for merge comparison
- what constitutes a conflict, on the scalar `version` basis, including the known concurrency limitation
- the requirement not to discard a *detected* conflict implicitly
- the union-biased default and the limitation created by compaction erasing tombstones

Two capabilities are deliberately deferred and, when added, are additive — files written to this version stay valid: per-record **version vectors** for reliable concurrency detection, and a **persistent deletion log** for deletion-aware merge across compacted files. Until then, automatic resolution and cross-device deletion are outside the v1 format contract.