---
title: AKG lifecycle example
status: release-candidate docs
---

# AKG lifecycle example

This is a tiny core-format example, not an SDK or product workflow. It uses only
the documented root package API to:

1. create an AKG file;
2. add three nodes and two edges;
3. commit and reopen the file;
4. read records back with exact lookups and whole-state lists;
5. compact and validate the result.

## What this example does not show

This example demonstrates the AKG file lifecycle, not agent context retrieval.
The file header and internal indexes help AKG readers find and validate sections,
keys, nodes, edges, and derived state. They are not an agent-facing table of
contents or ranking system.

An SDK or product can build retrieval behavior above core using AKG nodes, edges,
tags, metadata, and whole-state reads. For example, an SDK might add tag lookup,
inbound/outbound edge traversal, cached indexes, semantic search, recency rules,
or task-specific memory selection. Those policies are intentionally outside this
minimal lifecycle example and outside AKG core.

Run it from a clean checkout:

```sh
go run ./examples/lifecycle
```

By default the program writes a temporary `.akg` file and prints a readable node
summary. To keep the output file, pass a path that does not already exist:

```sh
go run ./examples/lifecycle /tmp/akg-lifecycle.akg
```
