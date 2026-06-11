# Key Layout and Index Design

AKG stores graph records in a sorted key space. In this model, lookup behavior is determined by key prefix and lexical order rather than by separate secondary index structures. A conformant writer produces the required keys for each logical record. A conformant reader performs lookups by prefix scan over those keys.

This section defines the key layout used within the Data section.

## General Model

In AKG, sort order is part of the format architecture. The primary node store, the primary edge store, and the required inverted indexes are all represented as keys in the same sorted key space.

Adding an index in AKG therefore means writing additional prefixed keys alongside the primary record write. Readers do not infer indexes from payload content alone; they rely on the presence of the corresponding keys.

All keys defined in this section are UTF-8 strings. The colon character `:` is the reserved key delimiter.

Required key order is bytewise ascending lexicographic order over the raw UTF-8 key bytes exactly as written. Locale rules, Unicode normalization, and case folding do not apply.

## Node Primary Key

The primary key for a node is:

- `n:{type}:{id}`

`type` is the node type string from the node payload. `id` is the node identifier assigned by the writer.

This layout groups all nodes of the same type into one contiguous key range. A reader that needs all nodes of a given type performs a prefix scan on `n:{type}:`.

A conformant writer must ensure that every node identifier satisfies all of the following constraints:

- it is an opaque string
- it contains no `:` characters
- it is at most 64 characters long
- it is unique within a node type's key space

Because the node `type` is also a key component, it is subject to the same key-component constraints: a conformant writer must reject a `type` that contains a `:` character, is empty, or exceeds 64 bytes.

Node identity is the tuple `(type, id)`, matching the full `n:{type}:{id}` primary key. The same `id` string may appear under different `type` values; those are distinct logical nodes. Changing a node's `type` is therefore an identity change, not an in-place mutation of the existing node.

Writers must reject a node identifier that violates any of these constraints. They must not normalize, truncate, or silently rewrite it.

A node's identifier is part of the node key, not part of the payload. This preserves the distinction between node identity and node content defined in Section 1.

## Edge Primary Key

The primary key for an edge is:

- `e:{fromType}:{fromID}:{relation}:{toType}:{toID}`

`fromType` and `toType` are the type strings of the source and destination nodes. `fromID` and `toID` are the node identifier strings.

This key orders edges by source node type first, then by source node ID, then by relation, then by destination node type, then by destination node ID. A reader that needs outbound edges for a node of type `T` and ID `X` performs a prefix scan on `e:{T}:{X}:`.

Because edge identity in AKG is the tuple `(from_node_type, from_node, relation, to_node_type, to_node)`, the edge primary key encodes the full edge identity.

Because the `relation` string is a key component, it is subject to the same key-component constraints as node types and identifiers: a conformant writer must reject a `relation` that contains a `:` character, is empty, or exceeds 64 bytes.

## Inverted Edge Index

Every edge write must also produce an inbound index entry with the key:

- `ei:{toType}:{toID}:{relation}:{fromType}:{fromID}`

This key supports inbound traversal. A reader that needs to determine which nodes point to a given node of type `T` and ID `X` performs a prefix scan on `ei:{T}:{X}:`.

The primary edge entry and the inverted edge entry are a single logical update. A conformant writer must write or delete them atomically with respect to the edge mutation being applied.

## Tag Index

For each tag on a node, the writer must produce one tag index entry with the key:

- `t:{tag}:{node_id}`

A reader that needs all nodes carrying a given tag performs a prefix scan on `t:{tag}:`.

Tag values are subject to the following format-level constraints, which exist for key safety and resource bounds:

- a tag contains no `:` characters
- a tag is non-empty and at most 64 bytes
- a node may carry at most 32 tags

Casing and word separation (for example, lowercase and snake_case) are an SDK-level convention, not a format rule; see the SDK author guide. Writers must reject tags that violate the format-level constraints above, and must not silently correct input — no lowercasing, whitespace normalization, or space-to-underscore rewriting.

## Temporal Index

Temporal index keys are self-describing and include the full logical identity of the indexed record.

Key forms:

- node: `ts:{timestamp}:n:{type}:{id}`
- edge: `ts:{timestamp}:e:{fromType}:{fromID}:{relation}:{toType}:{toID}`

`timestamp` is the record's `updated_at` value, encoded as a Unix-microsecond timestamp.

A conformant writer must produce exactly one temporal index entry per logical record. AKG v1 does not define a separate creation-time index keyed on `created_at`.

This index supports recency-oriented scans such as retrieving records updated around a given time range. A future minor version may define additional optional temporal indexes without invalidating files written to this version.

## Omitted Title Prefix Index

AKG does not define a title-prefix index.

Writers must not assume that title-fragment lookup is part of the required on-disk index set. Readers must not expect a title-prefix index to be present. This omission is intentional: title-prefix search is not a core AKG access pattern, and a prefix index over titles does not provide general full-text search.

A future minor version may define additional optional index keys without invalidating files written to this version.

## Key Prefix Table

The complete required key prefix set for AKG v1 is:

| Prefix | Key structure | Purpose |
| --- | --- | --- |
| `n:` | `n:{type}:{id}` | Node primary store, grouped by type |
| `e:` | `e:{fromType}:{fromID}:{relation}:{toType}:{toID}` | Outbound edge lookup |
| `ei:` | `ei:{toType}:{toID}:{relation}:{fromType}:{fromID}` | Inbound edge lookup |
| `t:` | `t:{tag}:{node_id}` | Tag inverted index |
| `ts:` | `ts:{timestamp}:n:{type}:{id}` or `ts:{timestamp}:e:{fromType}:{fromID}:{relation}:{toType}:{toID}` | Temporal index keyed on `updated_at` |

## Validation and Rejection Behavior

AKG follows a fail-fast validation model for key construction inputs.

Where this section defines constraints on node identifiers, tags, or key components, a conformant writer must reject invalid input clearly. Silent correction is not conformant.

Readers may assume that keys written by conformant writers satisfy these rules. Files that contain malformed keys are non-conformant and may be rejected by implementations that validate key syntax during read.