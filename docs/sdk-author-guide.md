---
title: AKG SDK author guide
status: release-candidate docs
---

# AKG SDK author guide

AKG core defines the file format. An SDK builds useful behavior on top of it —
ingestion pipelines, retrieval helpers, tagging utilities, traversal patterns,
and any product-specific memory policy your users need. That work belongs in the
SDK, not in the format layer.

This guide is for anyone implementing an AKG SDK in any language.

## What an SDK does

An SDK may provide:

- application-specific create/open wrappers;
- ingestion from notes, chats, files, issues, or product data;
- tag, inbound-edge, outbound-edge, and traversal helpers;
- caches or read indexes derived from current AKG state;
- search, ranking, embeddings, or retrieval pipelines stored outside the core
  format or represented as ordinary nodes and edges;
- product policy for naming, retention, summarization, and permissions.

These are SDK choices. None of them are required for a conformant AKG reader or
writer.

## Suggested architecture

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

AKG core is the durable interchange layer. Your SDK decides how to create nodes
and edges, how to search them, whether to pair them with external retrieval
systems, and how to surface them to users. Another SDK can make entirely
different choices and still produce conformant `.akg` files.

## Conformance testing

The conformance test suite lives in `testdata/conformance/`. It contains `.akg`
fixture files and a `manifest.json` that describes accept/reject expectations for
each one.

Run your implementation against it to verify format compatibility. You do not
need to import the Go package or match its internal structure — the contract is
the spec plus the fixture expectations.

See the [Conformance guide](conformance.md) for setup and usage details.

## Reference implementation

The Go package at `github.com/RobertGumeny/akg-go` is a minimal reference
implementation. Treat it as the behavior to match at the format boundary, not as
a blueprint for your own internal structure.

For the current Go surface, see [Public API](API.md). For lifecycle behavior,
see [Lifecycle guide](lifecycle.md).
