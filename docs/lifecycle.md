---
title: AKG lifecycle guide
status: release-candidate docs
---

# AKG lifecycle guide

The AKG file lifecycle is deliberately small:

1. create a file;
2. add or delete nodes and edges;
3. commit mutations;
4. close and reopen through ordinary validation;
5. read current state;
6. compact when you want to rewrite the file to live state only;
7. validate the result.

The runnable walkthrough lives in [`../examples/lifecycle`](../examples/lifecycle).
Run it with:

```sh
go run ./examples/lifecycle
```

## Minimal Go shape

```go
store, err := akg.Create("example.akg")
// handle err

_, err = store.PutNode("node-1", akg.Node{
    Type:      "note",
    Title:     "First AKG node",
    CreatedAt: 1,
    UpdatedAt: 1,
    Version:   1,
})
// handle err

err = store.Commit()
// handle err
err = store.Close()
// handle err

reopened, err := akg.Open("example.akg")
// handle err
node, ok := reopened.GetNode("note", "node-1")
_ = node
_ = ok

err = reopened.Compact()
// handle err
err = akg.Validate("example.akg")
// handle err
```

## What reads mean

The v1 public API returns current live state only:

- `GetNode` and `GetEdge` read exact records by identity.
- `ListNodes` and `ListEdges` list current live records.

It does not expose WAL records, deleted records, historical versions, internal
sections, derived index mutation, traversal, or query planning. SDKs can build
richer read helpers above these primitives.

## Compaction

Compaction is explicit. It rewrites the file so current live nodes and edges are
stored as compact Data/Bloom state with an empty WAL. It is not a background
service and it is not automatic multi-writer coordination.

## Validation

`akg.Validate(path)` checks that a file opens under ordinary strict semantics.
For cross-implementation validation behavior, see the
[conformance guide](conformance.md).
