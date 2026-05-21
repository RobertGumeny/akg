---
title: AKG SDK author guide
status: release-candidate docs
---

# AKG SDK author guide

AKG core is the format layer. SDKs and products should build user-facing memory
behavior above it instead of adding that behavior to the file format.

## A good SDK boundary

An SDK may provide:

- application-specific create/open wrappers;
- ingestion from notes, chats, files, issues, or product data;
- tag, inbound-edge, outbound-edge, and traversal helpers;
- caches or read indexes derived from current AKG state;
- search, ranking, embeddings, or retrieval pipelines stored outside the core
  format or represented as ordinary nodes and edges;
- product policy for naming, retention, summarization, and permissions.

Those are SDK choices. They should not be required for a conformant AKG reader.

## What not to put in the file format

Do not make the core format depend on:

- memory-file ingestion rules;
- a specific agent workflow;
- vector index internals;
- query language execution;
- merge/conflict-resolution policy;
- background compaction or daemon behavior;
- multi-writer coordination.

If a feature can be rebuilt from nodes, edges, tags, metadata, or external
indexes, keep it above core.

## Example architecture

One reasonable product architecture is:

```text
Product or agent workflow
        ↓
SDK ingestion and retrieval policy
        ↓
SDK indexes/caches/search, if needed
        ↓
AKG core read/write/validate/compact
        ↓
.akg file
```

In that shape, AKG core is the durable interchange layer. The SDK can choose how
to create nodes and edges, how to search them, and how to present them to users.
Another SDK can make different choices while still producing conformant `.akg`
files.

## Reference implementation relationship

The Go package is a canonical minimal reference implementation and conformance
oracle. It is safe to use directly, but it is not the required architecture for
every downstream SDK. Treat it as the behavior to match at the format boundary,
not as a mandate to copy internal package layout.

For the current Go surface, see [Public API](API.md). For lifecycle behavior, see
[Lifecycle guide](lifecycle.md).
