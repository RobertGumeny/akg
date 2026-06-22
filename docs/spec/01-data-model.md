---
title: AKG data model
status: v1 draft
---

# Data Model

AKG stores graph data as nodes and edges. Nodes are the primary units of knowledge. Edges express directed relationships between nodes. This section defines the logical schema for both record types and the identity rules that govern them.

## Node Model

A node payload in AKG has the following schema:

- `type: string` — required
- `title: string` — required
- `body: string` — optional, default `""`
- `meta: map<string, any>` — optional, default `{}`
- `tags: string[]` — optional, default `[]`
- `created_at: timestamp` — Unix microseconds, `uint64`
- `updated_at: timestamp` — Unix microseconds, `uint64`
- `version: uint32` — optional, default `1`

`type` classifies the node at the application level. The AKG format does not prescribe a closed vocabulary for this field. Any UTF-8 string is valid as a `type` value, subject to the key-component constraints in Section 4 (no `:` delimiter, non-empty, at most 64 bytes), because `type` is part of the node key.

`title` is the required human-readable label for the node.

`body` is optional free-form text associated with the node. If omitted, readers apply the read-time default value `""`.

`meta` is an extensible map for structured application data. The format does not constrain the semantic meaning of keys in `meta`.

`tags` is an optional list of string labels associated with the node. If omitted, readers apply the read-time default value `[]`.

`created_at` and `updated_at` record node creation time and most recent modification time as Unix microseconds stored in `uint64` form.

Conformant writers must write both timestamp fields. Readers that encounter a node payload with either timestamp absent must apply the read-time default `0` rather than rejecting the record.

`version` is a mutation counter. If omitted, readers apply the read-time default `1`. The value increments on every node mutation.

A node's identifier is not carried inside the node payload. In AKG, node identity and node contents are separate concerns. The payload describes the node's data; the node key identifies the node.

## Node Identity

The identity of a node is the tuple `(type, id)`, where `type` comes from the node payload and `id` comes from the node primary key `n:{type}:{id}`. Node IDs are unique within a node type's key space, not globally across all node types.

This means the same `id` string may identify distinct nodes when paired with different `type` values. Changing a node's `type` changes node identity; it is not an in-place mutation of the existing node.

## Edge Model

An edge payload in AKG has the following schema:

- `from_node_type: string` — required
- `from_node: string` — required
- `to_node_type: string` — required
- `to_node: string` — required
- `relation: string` — required
- `strength: float` — optional, default `0.5`
- `confidence: float | null` — optional, default `null`
- `meta: map<string, any>` — optional, default `{}`
- `created_at: timestamp` — Unix microseconds, `uint64`
- `updated_at: timestamp` — Unix microseconds, `uint64`
- `version: uint32` — optional, default `1`

`from_node_type` and `to_node_type` identify the types of the source and destination nodes. Because node identity in AKG is the tuple `(type, id)`, an edge must carry the type of each endpoint to fully qualify which nodes it connects.

`from_node` and `to_node` identify the source and destination node IDs of the directed relationship.

`relation` names the relationship. The AKG format does not prescribe a closed vocabulary for this field. Any UTF-8 string is valid as a `relation` value, subject to the key-component constraints in Section 4 (no `:` delimiter, non-empty, at most 64 bytes), because `relation` is part of the edge key.

`strength` expresses the importance, centrality, or salience of the relationship. If omitted, readers apply the read-time default value `0.5`.

`confidence` expresses how certain the writer is that the relationship is valid. If omitted, readers apply the read-time default value `null`.

`meta` is an extensible map for structured edge-local data.

`created_at` and `updated_at` record edge creation time and most recent modification time as Unix microseconds stored in `uint64` form.

Conformant writers must write both timestamp fields. Readers that encounter an edge payload with either timestamp absent must apply the read-time default `0` rather than rejecting the record.

`version` is a mutation counter. If omitted, readers apply the read-time default `1`. The value increments on every edge mutation.

## Edge Identity

Edges do not carry an `id` field. The identity of an edge is the natural key formed by the tuple `(from_node_type, from_node, relation, to_node_type, to_node)`.

Because node identity is `(type, id)`, edge identity must also carry both components of each endpoint's identity. Referencing nodes by bare `id` string alone would be ambiguous when two nodes of different types share the same `id` value.

This identity rule means that AKG permits at most one edge for a given source node identity, relation string, and destination node identity combination. Attributes such as `strength`, `confidence`, `meta`, timestamps, and `version` describe that edge instance but do not participate in its identity.

## Mutability and Versioning

Nodes and edges are mutable records.

For nodes, any change to payload content constitutes a mutation and increments `version`.

For edges, fields including `strength`, `confidence`, and `meta` may change over time without changing edge identity. Such changes are mutations of the existing edge, not creation of a new edge. Every edge mutation increments `version`.

The `version` field is therefore the in-record indicator of change history for both entity types. AKG uses the same versioning model for nodes and edges.

## Defaults and Record Meaning

AKG requires only the fields necessary to make a record meaningful.

For nodes, the required fields are `type` and `title`. All other optional fields have defined defaults:

- `body` defaults to `""`
- `meta` defaults to `{}`
- `tags` defaults to `[]`
- `version` defaults to `1`

For edges, the required fields are `from_node_type`, `from_node`, `to_node_type`, `to_node`, and `relation`. All other optional fields have defined defaults:

- `strength` defaults to `0.5`
- `confidence` defaults to `null`
- `meta` defaults to `{}`
- `version` defaults to `1`

The distinction between `confidence: null` and a numeric confidence value is significant. `null` means no confidence judgment has been recorded. A numeric value such as `0.5` means a judgment was made and that judgment has that value. These states are not equivalent.

## Type and Relation Vocabularies

AKG is format-agnostic with respect to node types and edge relations. The format validates only that `type` and `relation` are strings meeting the key-component constraints in Section 4 (no `:` delimiter, non-empty, at most 64 bytes). It does not require a registry, fixed taxonomy, or predefined ontology.

Implementations may publish default node type taxonomies, default relation vocabularies, or informational registries for interoperability and ergonomics. Such conventions are outside the format contract. A conformant AKG reader or writer must accept custom `type` and `relation` strings without treating them as schema violations.
