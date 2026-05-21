# Encoding

This section defines the payload encoding used for AKG node and edge payloads. Unless otherwise stated, node and edge payloads are serialized as MessagePack values.

## MessagePack

AKG uses MessagePack as the serialization format for all node and edge payloads.

Each payload must be encoded as a MessagePack map, not as a positional array. Field names are part of the serialized representation. This makes payloads forward-compatible: a reader that encounters a field it does not recognize may ignore that field without losing the ability to decode the rest of the record.

A conformant writer must encode node and edge payloads as maps. A positional array encoding is not conformant.

## String Encoding

All strings in AKG payloads are UTF-8 encoded. This includes, but is not limited to, node `type`, node `title`, node `body`, edge `relation`, tag values, `meta` map keys, and string values nested within `meta`. A reader must reject a payload containing any MessagePack string value or map key that is not valid UTF-8.

## Timestamps

AKG timestamps are Unix time in microseconds stored as plain unsigned 64-bit integers.

This encoding is used for:

- node `created_at`
- node `updated_at`
- edge `created_at`
- edge `updated_at`
- any header or file-level timestamp field defined by this specification or a future backward-compatible extension

AKG does not use a MessagePack extension type for timestamps.

## Node Payload Encoding

A node payload has the following field set:

- `type: string` — required
- `title: string` — required
- `body: string` — optional, default `""`
- `meta: map<string, any>` — optional, default `{}`
- `tags: string[]` — optional, default `[]`
- `created_at: uint64` — required on write, read-time default `0` if absent
- `updated_at: uint64` — required on write, read-time default `0` if absent
- `version: uint32` — optional, default `1`

A conformant writer must write `type`, `title`, `created_at`, and `updated_at`.

A reader must reject a node payload if either required field `type` or `title` is missing. If an optional field is absent, the reader must apply its default silently.

For `created_at` and `updated_at`, writers are strict and readers are lenient: a conformant writer must emit both fields, but a reader encountering an older or non-conformant payload without one or both fields must apply the read-time default `0` rather than rejecting the record.

## Edge Payload Encoding

An edge payload has the following field set:

- `from_node: string` — required
- `to_node: string` — required
- `relation: string` — required
- `strength: float` — optional, default `0.5`
- `confidence: float | null` — optional, default `null`
- `meta: map<string, any>` — optional, default `{}`
- `created_at: uint64` — required on write, read-time default `0` if absent
- `updated_at: uint64` — required on write, read-time default `0` if absent
- `version: uint32` — optional, default `1`

A conformant writer must write `from_node`, `to_node`, `relation`, `created_at`, and `updated_at`.

A reader must reject an edge payload if any of the required fields `from_node`, `to_node`, or `relation` is missing. If an optional field is absent, the reader must apply its default silently.

As with nodes, `created_at` and `updated_at` are mandatory on write and have read-time default `0` when absent.

## Default Application

Readers must apply the following defaults when optional fields are missing.

For nodes:

- `body` → `""`
- `meta` → `{}`
- `tags` → `[]`
- `version` → `1`

For edges:

- `strength` → `0.5`
- `confidence` → `null`
- `meta` → `{}`
- `version` → `1`

Applying these defaults is part of normal decoding behavior and must not be treated as an error condition.

## `strength` and `confidence`

Edge payloads separate relationship importance from relationship certainty.

`strength` is a floating-point value describing how important, central, or salient the relationship is. `confidence` is either a floating-point value or `null`, and describes how certain the writer is that the relationship is valid.

`confidence: null` and `confidence: 0.5` are not equivalent. `null` means no confidence judgment has been recorded. A numeric value means a judgment was made and that value is the judgment.

## `meta` Value Types

The `meta` field of both nodes and edges may contain any value representable by standard MessagePack. This includes:

- strings
- integers
- floating-point values
- booleans
- arrays
- maps
- nil

Nested arrays and nested maps are valid. Arbitrary nesting is permitted by the format.

The format does not define application meaning for `meta` keys or values.

## Field Widths and MessagePack Integer Encoding

AKG uses fixed-width binary fields where the file container requires them, such as the header and section table structures defined in Section 2. Within MessagePack payloads, however, integer values use MessagePack's native encoding rules rather than a payload-level fixed-width layout.

Accordingly, a timestamp conceptually defined as `uint64` is encoded in the header or section table using the fixed-width binary structure of that container, but in a node or edge payload it is encoded as a MessagePack integer value.
