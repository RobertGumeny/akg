# akg-go

Go SDK for reading and writing AKG knowledge graph files.

- Module: `github.com/RobertGumeny/akg/sdk/akg-go`
- Implements AKG v1 independently, without importing the Go Reference SDK — this
  keeps the public API free to expose the full surface an application needs
  (tag lookup, edge traversal, etc.) without being constrained by the Reference
  SDK's intentionally minimal scope.

## Install

```sh
go get github.com/RobertGumeny/akg/sdk/akg-go
```

## Quick start

```go
import akg "github.com/RobertGumeny/akg/sdk/akg-go"

store, err := akg.Open("memory.akg")
if err != nil { ... }
defer store.Close()

alice, err := store.PutNode("person", "alice", akg.NodeFields{
    Title: "Alice",
    Body:  "A researcher.",
}, []string{"active"})

bob, err := store.PutNode("person", "bob", akg.NodeFields{
    Title: "Bob",
}, nil)

err = store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
```

## Command-line tool

The SDK ships a single `akg-go` binary in the conventional `akg-go <command> [args]`
shape:

```sh
go build -o akg-go ./cmd/akg-go

# render a .akg file as readable text, grouped by node type
akg-go show memory.akg
akg-go show memory.akg --json   # full snapshot as JSON
akg-go show memory.akg --all    # don't collapse large/per-hand node types

# look up the SDK's own API — shipped as an AKG graph, for an agent coding against akg-go
akg-go docs explain PutNode
akg-go docs search commit
akg-go docs overview
akg-go docs dump --format markdown

# (maintainers) regenerate the embedded docs graph from docs/manifest.json
akg-go gen-docs
```

**akg-go ships with full documentation encoded in a `.akg` file.** Install the SDK,
and then you — or your coding agent — can use `akg-go docs` to implement it in your
project: pull exactly the symbol you need — `explain PutNode`, `search "delete"` —
instead of loading the whole API doc into context. The SDK is both the tool and a
worked example of the knowledge graph it builds.

`show` is the general-purpose reader, the human-facing companion to the reference
CLI's JSON `akg inspect`: it opens any store with `Open`, takes a `Snapshot`, and
prints each node's title and body under its type, so you can read what *any*
application wrote — an agent's memory, the docs graph above — without parsing the
binary format by hand.

## Getting started

Create a project, write a graph, run it.

**1. Create a project directory and initialize a module:**

```sh
mkdir mygraph && cd mygraph
go mod init mygraph
```

**2. Install akg-go:**

```sh
go get github.com/RobertGumeny/akg/sdk/akg-go
```

**3. Create `main.go`:**

```go
package main

import (
	"fmt"
	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

func main() {
	store, _ := akg.Open("mygraph.akg")
	defer store.Close()

	alice, _ := store.PutNode("person", "alice", akg.NodeFields{Title: "Alice"}, nil)
	bob, _ := store.PutNode("person", "bob", akg.NodeFields{Title: "Bob"}, nil)
	store.PutEdge(alice, "knows", bob, akg.EdgeFields{})
	store.Commit()

	node, _ := store.GetNode("person", "alice")
	fmt.Printf("node: %s/%s — %q\n", node.Type, node.ID, node.Title)

	edges, _ := store.OutboundEdges(alice, "")
	for _, e := range edges {
		fmt.Printf("  -[%s]-> %s/%s\n", e.Relation, e.To.Type, e.To.ID)
	}
}
```

**4. Run it:**

```sh
go run .
```

**Expected output:**

```
node: person/alice — "Alice"
  -[knows]-> person/bob
```

A `mygraph.akg` file is now in your directory. Open it again later — the graph persists.

## Naming rules

AKG enforces naming constraints on type names, relation names, and tags. Node IDs
follow a separate, more permissive rule.

| Component | Rule |
|---|---|
| Type names | Lowercase `[a-z0-9_]`; no leading, trailing, or consecutive underscores |
| Relation names | Same as type names |
| Tags | Same as type names |
| Node IDs | Any valid UTF-8 string up to 64 characters; colons (`:`) are not allowed |

Node IDs are deliberately more permissive — they may be user-supplied slugs, hex
strings, UUIDs, or anything else that avoids `:`, which is reserved as a key
delimiter. Type names, relation names, and tags share the same stricter rule because
they are used as structural labels in the graph's key space.

Invalid values are rejected at write time with `ErrInvalidInput`.

## API

### Opening a store

```go
store, err := akg.Open(path string) (*Store, error)
```

Opens an existing `.akg` file or creates a new empty one if the path does not
exist. Returns an error if the file exists but is malformed.

### Writing nodes

```go
ref, err := store.PutNode(typeName, id string, fields NodeFields, tags []string) (NodeRef, error)
```

Writes or replaces the node at `(typeName, id)`. If `id` is empty, a new ID is
generated. Returns a `NodeRef` you can pass directly to `PutEdge`.

`NodeFields`:

| Field   | Required | Type            |
|---------|----------|-----------------|
| `Title` | yes      | `string`        |
| `Body`  | no       | `string`        |
| `Meta`  | no       | `map[string]any`|

See [Naming rules](#naming-rules) for the constraints on `typeName` and tags.

### Writing edges

```go
err := store.PutEdge(fromRef NodeRef, relation string, toRef NodeRef, fields EdgeFields) error
```

Writes or replaces the edge at `(fromRef, relation, toRef)`. Both referenced
nodes must already exist. See [Naming rules](#naming-rules) for the constraints
on `relation`.

`EdgeFields`:

| Field        | Required | Type            | Default |
|--------------|----------|-----------------|---------|
| `Strength`   | no       | `*float64`      | `0.5`   |
| `Confidence` | no       | `*float64`      | nil     |
| `Meta`       | no       | `map[string]any`| nil     |

**`Strength`** is a caller-defined weight for the edge — how strongly the relationship holds. Use it for ranking, sorting, or filtering (e.g. `0.0`–`1.0` for weak-to-strong). Both fields use `*float64` so that `nil` ("omitted") is distinguishable from an explicit value including `0.0`. Use `akg.StrengthOf(v)` to supply a value without a temp variable:

```go
// omitted → spec default 0.5
store.PutEdge(alice, "knows", bob, akg.EdgeFields{})

// explicit value
store.PutEdge(alice, "knows", bob, akg.EdgeFields{Strength: akg.StrengthOf(0.75)})

// explicitly zero
store.PutEdge(alice, "knows", bob, akg.EdgeFields{Strength: akg.StrengthOf(0.0)})
```

**`Confidence`** represents how certain you are that the edge is correct — for example when it was inferred rather than asserted. `nil` means no confidence judgment was recorded. When set, the convention is `0.0`–`1.0`. The SDK does not enforce a range.

### Reading

```go
// Returns (nil, nil) if the node does not exist — not an error.
node, err := store.GetNode(typeName, id string) (*Node, error)

// Returns all nodes carrying the given tag, sorted by key.
nodes, err := store.ListNodesByTag(tag string) ([]Node, error)

// Returns all nodes, optionally filtered to typeName. Pass "" to return all types.
// An unknown type returns an empty slice and nil error. Results are sorted by key.
nodes, err := store.ListNodes(typeName string) ([]Node, error)

// Pass an empty relation to return all edges regardless of relation.
edges, err := store.OutboundEdges(nodeRef NodeRef, relation string) ([]Edge, error)
edges, err := store.InboundEdges(nodeRef NodeRef, relation string) ([]Edge, error)
```

To filter by both type and tag, call `ListNodes` and filter the result, or use `ListNodesFiltered` (see [Filtering and inspection helpers](#filtering-and-inspection-helpers)).

### Metadata fields

`Node` and `Edge` carry three read-only fields set by the SDK:

| Field       | Type     | Description                                           |
|-------------|----------|-------------------------------------------------------|
| `CreatedAt` | `uint64` | Unix timestamp in **microseconds** when first written |
| `UpdatedAt` | `uint64` | Unix timestamp in **microseconds** of last `PutNode`/`PutEdge` |
| `Version`   | `uint32` | Starts at `1`, increments by `1` on each overwrite    |

These are set automatically and cannot be supplied by the caller.

### Committing and closing

```go
err := store.Commit() // durably writes all pending mutations
err := store.Close()  // commits outstanding mutations and closes the store
```

Mutations (`PutNode`, `PutEdge`, `DeleteNode`, `DeleteEdge`) are held in memory until `Commit` or `Close` is called. They are not visible to other processes and will be lost if the process exits without committing.

Call `Commit` periodically in long-running processes where losing a batch of work would be costly. Call `Close` when you're done with the store — it commits any outstanding mutations and releases the file handle. `Close` is safe to call on a store with no pending mutations.

## Deleting nodes and edges

```go
err := store.DeleteNode(typeName, id string) error
err := store.DeleteEdge(fromRef NodeRef, relation string, toRef NodeRef) error
```

Both return `ErrNotFound` if the target does not exist.

**You must delete all edges before deleting a node.** Attempting to delete a node that still has live edges — inbound or outbound — returns `ErrInvalidInput`. The graph does not cascade-delete edges automatically; this is intentional so that deletions are explicit and auditable.

```go
// correct order: edges first, then the node
store.DeleteEdge(alice, "knows", bob)
store.DeleteNode("person", "bob")

// wrong order — returns ErrInvalidInput
store.DeleteNode("person", "bob") // bob still has a "knows" edge
```

## Error handling

Three sentinel errors are exported for callers that need to branch on error type:

| Sentinel | Returned when |
|---|---|
| `akg.ErrNotFound` | A `DeleteNode` or `DeleteEdge` call targets a node or edge that does not exist. |
| `akg.ErrInvalidInput` | A caller passes an argument that violates a format or semantic constraint — invalid type name, missing required field, or attempting to delete a node that still has live edges. |
| `akg.ErrMissingRequiredField` | A required field is absent. Returned in two situations: (1) a `PutNode` call omits `Title`, or a `PutEdge` call omits a required identity field; (2) a decoded record in a file is structurally valid but missing a required field — callers see this when opening a malformed file written by a buggy writer. |

`GetNode` is a special case: a missing node returns `(nil, nil)`, not `ErrNotFound`. Check the pointer, not the error:

```go
node, err := store.GetNode("person", "alice")
if err != nil {
    // I/O or decode error
}
if node == nil {
    // node does not exist
}
```

Use `errors.Is` for the delete sentinels:

```go
err := store.DeleteNode("person", "alice")
if errors.Is(err, akg.ErrNotFound) {
    // node did not exist
}
```

## NodeRef

`PutNode` returns a `NodeRef`:

```go
type NodeRef struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}
```

This shape is part of the public SDK contract and is identical across the Go and
TypeScript SDKs, including field names and JSON keys. `NodeRef` values are safe
to serialize and pass between systems.

A `NodeRef` returned by `PutNode` can be passed directly to `PutEdge`,
`OutboundEdges`, `InboundEdges`, `DeleteNode`, and `DeleteEdge` without
re-fetching the node. You can also construct one manually from a known type
and ID:

```go
ref := akg.NodeRef{Type: "person", ID: "alice"}
edges, err := store.OutboundEdges(ref, "")
```

When an empty string is passed as `id` to `PutNode`, the SDK generates a unique
ID. The generated ID is available on the returned `NodeRef`:

```go
ref, err := store.PutNode("person", "", akg.NodeFields{Title: "New person"}, nil)
fmt.Println(ref.ID) // e.g. "01J2K3..."
```

## Compaction

```go
err := store.Compact() error
```

`Compact` rewrites the `.akg` file to contain only live records, discarding all tombstones and prior WAL history. Before compacting, it automatically commits any pending in-memory mutations. If the auto-commit fails, compaction does not run.

After a successful compaction:
- The logical graph content (nodes and edges) is unchanged.
- The file contains no WAL section.
- The open store remains fully usable.

**Compaction is always caller-triggered — it is never automatic.** Callers that do not call `Compact` will accumulate WAL entries over time; this is safe but eventually increases file size.

```go
store.DeleteEdge(alice, "knows", bob)
if err := store.Compact(); err != nil { ... }
// file now contains only live records; no WAL, no tombstones
nodes, _ := store.ListNodes("")
```

## Filtering and inspection helpers

### ListNodesFiltered

```go
nodes, err := store.ListNodesFiltered(filter akg.NodeFilter) ([]Node, error)
```

`NodeFilter` fields:

| Field  | Type            | Matches |
|--------|-----------------|---------|
| `Type` | `string`        | Nodes of this type (empty = all types) |
| `Tag`  | `string`        | Nodes carrying this tag (empty = all tags) |
| `Meta` | `map[string]any`| Nodes whose metadata contains all key/value pairs |

Non-empty fields combine with AND semantics. Unknown types or tags return empty results rather than errors. Metadata filtering uses JSON-like deep equality: scalars by value, arrays by ordered equality, objects by recursive equality ignoring key order.

```go
nodes, _ := store.ListNodesFiltered(akg.NodeFilter{
    Type: "decision",
    Tag:  "active",
    Meta: map[string]any{"status": "accepted"},
})
```

### GetNodes

```go
selected, err := store.GetNodes(refs []NodeRef) ([]*Node, error)
```

Returns one output position per input ref. Preserves input order. Preserves duplicate refs as duplicate output positions. Returns `nil` at positions where the referenced node does not exist.

```go
selected, _ := store.GetNodes([]akg.NodeRef{
    {Type: "decision", ID: "d1"},
    {Type: "task", ID: "t1"},
})
// selected[0] is *Node for d1, or nil if missing
// selected[1] is *Node for t1, or nil if missing
```

### ListEdges

```go
edges, err := store.ListEdges(filter akg.EdgeFilter) ([]Edge, error)
```

`EdgeFilter` fields:

| Field      | Type            | Matches |
|------------|-----------------|---------|
| `Relation` | `string`        | Edges with this relation (empty = all relations) |
| `Meta`     | `map[string]any`| Edges whose metadata contains all key/value pairs |

```go
allEdges, _ := store.ListEdges(akg.EdgeFilter{})
knowsEdges, _ := store.ListEdges(akg.EdgeFilter{Relation: "knows"})
inferred, _ := store.ListEdges(akg.EdgeFilter{Meta: map[string]any{"source": "inferred"}})
```

### Snapshot

```go
snap, err := store.Snapshot() (Snapshot, error)
```

Returns all live nodes and all live edges in deterministic order. The `Snapshot` struct is JSON-serializable using standard library tooling.

```go
snap, _ := store.Snapshot()
encoded, _ := json.Marshal(snap)
fmt.Printf("%d nodes, %d edges\n", len(snap.Nodes), len(snap.Edges))
```

## Recency helpers

Recency helpers return records sorted newest-first by `updatedAt` (Unix microseconds). Tie-breaker for nodes: `createdAt` desc, `type` asc, `id` asc. Tie-breaker for edges: `createdAt` desc, `from.type` asc, `from.id` asc, `relation` asc, `to.type` asc, `to.id` asc.

Time-window bounds are inclusive: `sinceUpdatedAt <= updatedAt <= untilUpdatedAt`. Timestamps are Unix microseconds, matching `Node.UpdatedAt` and `Edge.UpdatedAt`.

```go
recentNodes, _ := store.RecentNodes(akg.RecencyFilter{
    Type:  "task",
    Tag:   "active",
    Limit: 20,
})

taskRef := akg.NodeRef{Type: "task", ID: "t1"}
recentEdges, _ := store.RecentEdges(akg.EdgeRecencyFilter{
    From:     &taskRef,
    Relation: "depends_on",
    Limit:    20,
})
```

`Limit 0` means unlimited. Positive `Limit` caps results after filtering and sorting. Negative `Limit` returns `ErrInvalidInput`.

These helpers are not cursor-pagination APIs; callers that need duplicate-free checkpoint pagination should track a full cursor separately.

## Edge reconciliation

`ReconcileOutboundEdges` synchronizes the outbound edges for a source node and relation to exactly the desired target set. Missing desired edges are added; stale edges (same source+relation, not in desired) are removed; edges for other relations or other source nodes are unchanged.

```go
result, _ := store.ReconcileOutboundEdges(alice, "knows", []akg.NodeRef{bob}, akg.EdgeFields{Strength: 0.8})
fmt.Println(result.Added, result.Removed, result.Unchanged)
```

## Cascade delete

Normal `DeleteNode` rejects nodes with live edges. `DeleteNodeCascade` is an explicit opt-in helper that deletes all inbound and outbound edges first, then deletes the node. It is auditable: the returned `CascadeDeleteResult` reports how many edges and nodes were deleted.

```go
result, _ := store.DeleteNodeCascade("person", "alice")
fmt.Println(result.DeletedInboundEdges, result.DeletedOutboundEdges, result.DeletedNode)
```

`deleteNode` behavior is unchanged — it still rejects nodes with live edges. Only callers that explicitly call `DeleteNodeCascade` get cascade behavior.

## Concurrency and single-writer semantics

**One active writer per `.akg` file.** Only one process should have a store open for writing at a time.

Mutations (`PutNode`, `PutEdge`, `DeleteNode`, `DeleteEdge`) are held in memory until `Commit` or `Close` is called. They are not visible to other processes until after a successful commit or close.

Opening the same file from two processes simultaneously, or from two goroutines without external serialization, produces undefined behavior — there is no lock file or advisory lock. If you need concurrent access, serialize writes at the application layer.

Cross-platform lock-file or advisory locking is an explicit future enhancement.

## Run the example

```sh
go run ./examples/basic
```

See [`examples/basic/README.md`](examples/basic/README.md) for expected output.
