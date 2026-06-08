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

A merge conflict exists when two candidate records have the same identity and different logical content.

For this purpose, logical content is evaluated after normal AKG decoding rules have been applied, including:

- MessagePack map decoding
- application of all defined read-time defaults for omitted optional fields
- interpretation of node and edge payloads according to Sections 1 and 3

If two same-identity records decode to the same logical field values after default application, they are not in conflict.

If they differ in any logical field value, including `version`, timestamps, payload fields, or other defined record content, they are in conflict.

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

A merge implementation must not infer a definite deletion solely from absence in a compacted input file.

If correct deletion-aware merge requires distinguishing deletion from nonexistence, the implementation must rely on information outside the compacted live-record set. In AKG v1, this information is not standardized.

## Deferred Deletion History

A persistent deletion log that survives compaction is the expected mechanism for resolving the absence ambiguity described above.

AKG v1 does not define that mechanism. Its design is deferred to a future version.

Until such a mechanism is standardized, deletion-aware merge across independently compacted files is inherently incomplete.

## v1 Status

Merge behavior is intentionally underspecified in AKG v1.

This specification defines only:

- how identity is determined for merge comparison
- what constitutes a conflict
- the requirement not to discard conflicting versions implicitly
- the limitation created by compaction erasing tombstones

All richer merge behavior remains outside the v1 format contract.