# akg-ts

TypeScript SDK for reading and writing AKG knowledge graph files.

- Package: `akg-ts`
- Implements AKG v1 with a full public API — tag lookup, edge traversal, and everything you need to build on top of AKG.

## Install

```sh
npm install akg-ts
```

## Quick start

```typescript
import { open } from 'akg-ts';

const store = await open('memory.akg');

const alice = store.putNode('person', 'alice', {
  title: 'Alice',
  body: 'A researcher.',
}, ['active', 'researcher']);

const bob = store.putNode('person', 'bob', {
  title: 'Bob',
}, []);

store.putEdge(alice, 'knows', bob, {});
await store.close();
```

## Getting started

Create a project, write a graph, run it.

**1. Create a project directory and initialize npm:**

```sh
mkdir mygraph && cd mygraph
npm init -y
```

**2. Install akg-ts:**

```sh
npm install akg-ts
```

**3. Create `index.ts`:**

```typescript
import { open } from 'akg-ts';

const store = await open('mygraph.akg');
const alice = store.putNode('person', 'alice', { title: 'Alice' }, []);
const bob = store.putNode('person', 'bob', { title: 'Bob' }, []);
store.putEdge(alice, 'knows', bob, {});
await store.commit();

const node = store.getNode('person', 'alice')!;
console.log(`node: ${node.type}/${node.id} — "${node.title}"`);

const edges = store.outboundEdges(alice);
for (const e of edges) {
  console.log(`  -[${e.relation}]-> ${e.to.type}/${e.to.id}`);
}
await store.close();
```

**4. Run it:**

```sh
npx tsx index.ts
```

**Expected output:**

```
node: person/alice — "Alice"
  -[knows]-> person/bob
```

A `mygraph.akg` file is now in your directory. Open it again later — the graph persists.

## Naming rules

AKG enforces naming constraints on type names, relation names, and tags. Node IDs follow a separate, more permissive rule.

| Component | Rule |
|---|---|
| Type names | Lowercase `[a-z0-9_]`; no leading, trailing, or consecutive underscores |
| Relation names | Same as type names |
| Tags | Same as type names |
| Node IDs | Any valid UTF-8 string up to 64 characters; colons (`:`) are not allowed |

Node IDs are deliberately more permissive — they may be user-supplied slugs, hex strings, UUIDs, or anything else that avoids `:`, which is reserved as a key delimiter. Type names, relation names, and tags share the same stricter rule because they are used as structural labels in the graph's key space.

Invalid values are rejected at write time with `InvalidInputError`.

## API

### Opening a store

```typescript
import { open } from 'akg-ts';

const store = await open(path: string): Promise<Store>
```

Opens an existing `.akg` file or creates a new empty one if the path does not exist. Returns an error if the file exists but is malformed.

### Writing nodes

```typescript
const ref: NodeRef = store.putNode(typeName: string, id: string, fields: NodeFields, tags: string[]): NodeRef
```

Writes or replaces the node at `(typeName, id)`. If `id` is empty, a new ID is generated. Returns a `NodeRef` you can pass directly to `putEdge`. Throws synchronously on validation errors.

`NodeFields`:

| Field   | Required | Type                        |
|---------|----------|-----------------------------|
| `title` | yes      | `string`                    |
| `body`  | no       | `string`                    |
| `meta`  | no       | `Record<string, unknown>`   |

See [Naming rules](#naming-rules) for the constraints on `typeName` and tags.

### Writing edges

```typescript
store.putEdge(fromRef: NodeRef, relation: string, toRef: NodeRef, fields: EdgeFields): void
```

Writes or replaces the edge at `(fromRef, relation, toRef)`. Both referenced nodes must already exist. See [Naming rules](#naming-rules) for the constraints on `relation`.

`EdgeFields`:

| Field        | Required | Type                      | Default |
|--------------|----------|---------------------------|---------|
| `strength`   | no       | `number`                  | `0.5`   |
| `confidence` | no       | `number \| null`          | `null`  |
| `meta`       | no       | `Record<string, unknown>` | `{}`    |

**`strength`** is a caller-defined weight for the edge — how strongly the relationship holds. The SDK stores and returns it as-is; no semantic is imposed. Use it for ranking, sorting, or filtering (e.g. `0.0`–`1.0` for weak-to-strong, or an integer priority). If omitted (`undefined` in `EdgeFields`), the AKG v1 spec default of `0.5` is applied.

**`confidence`** represents how certain you are that the edge is correct — for example when it was inferred rather than asserted. `null` means no confidence value was recorded (i.e. the edge was asserted directly). When set, the convention is `0.0`–`1.0`. The SDK does not enforce a range. Default `null`.

### Reading

```typescript
// Returns null (not an error) if the node does not exist.
const node: Node | null = store.getNode(typeName: string, id: string)

// Returns all nodes carrying the given tag, sorted by key.
const nodes: Node[] = store.listNodesByTag(tag: string)

// Returns all nodes, optionally filtered to typeName. Pass no argument to return all types.
// An unknown type returns an empty array. Results are sorted by key.
const nodes: Node[] = store.listNodes(typeName?: string)

// Pass no relation to return all edges regardless of relation.
const edges: Edge[] = store.outboundEdges(nodeRef: NodeRef, relation?: string)
const edges: Edge[] = store.inboundEdges(nodeRef: NodeRef, relation?: string)
```

To filter by both type and tag, call `listNodesFiltered` (see [Filtering and inspection helpers](#filtering-and-inspection-helpers)), or call `listNodes(typeName)` and filter the result.

### Metadata fields

`Node` and `Edge` carry three read-only fields set by the SDK:

| Field       | Type     | Description                                       |
|-------------|----------|---------------------------------------------------|
| `createdAt` | `number` | Unix timestamp in **microseconds** when first written |
| `updatedAt` | `number` | Unix timestamp in **microseconds** of last `putNode`/`putEdge` |
| `version`   | `number` | Starts at `1`, increments by `1` on each overwrite |

These are set automatically and cannot be supplied by the caller.

### Committing and closing

```typescript
await store.commit(): Promise<void>  // durably writes all pending mutations
await store.close(): Promise<void>   // commits outstanding mutations and closes the store
```

Mutations (`putNode`, `putEdge`, `deleteNode`, `deleteEdge`) are held in memory until `commit` or `close` is called. They are not visible to other processes and will be lost if the process exits without committing.

Call `commit` periodically in long-running processes where losing a batch of work would be costly. Call `close` when you're done with the store — it commits any outstanding mutations and releases the file handle. `close` is safe to call on a store with no pending mutations.

## Deleting nodes and edges

```typescript
store.deleteNode(typeName: string, id: string): void
store.deleteEdge(fromRef: NodeRef, relation: string, toRef: NodeRef): void
```

Both throw `NotFoundError` if the target does not exist.

**You must delete all edges before deleting a node.** Attempting to delete a node that still has live edges — inbound or outbound — throws `InvalidInputError`. The graph does not cascade-delete edges automatically; this is intentional so that deletions are explicit and auditable.

```typescript
// correct order: edges first, then the node
store.deleteEdge(alice, 'knows', bob);
store.deleteNode('person', 'bob');

// wrong order — throws InvalidInputError
store.deleteNode('person', 'bob'); // bob still has a 'knows' edge
```

## Error handling

Three error classes are exported for callers that need to branch on error type:

| Class | Thrown when |
|---|---|
| `NotFoundError` | A `deleteNode` or `deleteEdge` call targets a node or edge that does not exist. |
| `InvalidInputError` | A caller passes an argument that violates a format or semantic constraint — invalid type name, missing required field, or attempting to delete a node that still has live edges. |
| `MissingRequiredFieldError` | A required field is absent. Returned in two situations: (1) a `putNode` call omits `title`; (2) a decoded record in a file is structurally valid but missing a required field — callers see this when opening a malformed file written by a buggy writer. |

Use `instanceof` to test:

```typescript
import { NotFoundError } from 'akg-ts';

try {
  store.deleteNode('person', 'alice');
} catch (err) {
  if (err instanceof NotFoundError) {
    // node does not exist
  }
}
```

Note: `getNode` returns `null` rather than throwing `NotFoundError` for missing nodes.

## NodeRef

`putNode` returns a `NodeRef`:

```typescript
interface NodeRef {
  type: string;
  id: string;
}
```

This shape is part of the public SDK contract and is identical across the Go and TypeScript SDKs, including field names and JSON keys. `NodeRef` values are safe to serialize and pass between systems.

A `NodeRef` returned by `putNode` can be passed directly to `putEdge`, `outboundEdges`, `inboundEdges`, `deleteNode`, and `deleteEdge` without re-fetching the node. You can also construct one manually from a known type and ID:

```typescript
const ref: NodeRef = { type: 'person', id: 'alice' };
const edges = store.outboundEdges(ref);
```

When an empty string is passed as `id` to `putNode`, the SDK generates a unique ID. The generated ID is available on the returned `NodeRef`:

```typescript
const ref = store.putNode('person', '', { title: 'New person' }, []);
console.log(ref.id); // e.g. "01J2K3..."
```

## Compaction

```typescript
await store.compact(): Promise<void>
```

`compact` rewrites the `.akg` file to contain only live records, discarding all tombstones and prior WAL history. Before compacting, it automatically commits any pending in-memory mutations. If the auto-commit fails, compaction does not run.

After a successful compaction:
- The logical graph content (nodes and edges) is unchanged.
- The file contains no WAL section.
- The open store remains fully usable.

**Compaction is always caller-triggered — it is never automatic.** Callers that do not call `compact` will accumulate WAL entries over time; this is safe but eventually increases file size.

```typescript
store.deleteEdge(alice, 'knows', bob);
await store.compact();
// file now contains only live records; no WAL, no tombstones
const nodes = store.listNodes();
```

## Filtering and inspection helpers

### listNodesFiltered

```typescript
store.listNodesFiltered(filter: NodeFilter): Node[]
```

`NodeFilter` fields:

| Field  | Type                      | Matches |
|--------|---------------------------|---------|
| `type` | `string`                  | Nodes of this type (omitted = all types) |
| `tag`  | `string`                  | Nodes carrying this tag (omitted = all tags) |
| `meta` | `Record<string, unknown>` | Nodes whose metadata contains all key/value pairs |

Non-empty fields combine with AND semantics. Unknown types or tags return empty results rather than errors. Metadata filtering uses JSON-like deep equality: scalars by value, arrays by ordered equality, objects by recursive equality ignoring key order.

```typescript
const nodes = store.listNodesFiltered({
  type: 'decision',
  tag: 'active',
  meta: { status: 'accepted' },
});
```

### getNodes

```typescript
store.getNodes(refs: NodeRef[]): Array<Node | null>
```

Returns one output position per input ref. Preserves input order. Preserves duplicate refs as duplicate output positions. Returns `null` at positions where the referenced node does not exist.

```typescript
const selected = store.getNodes([
  { type: 'decision', id: 'd1' },
  { type: 'task', id: 't1' },
]);
// selected[0] is Node for d1, or null if missing
// selected[1] is Node for t1, or null if missing
```

### listEdges

```typescript
store.listEdges(filter?: EdgeFilter): Edge[]
```

`EdgeFilter` fields:

| Field      | Type                      | Matches |
|------------|---------------------------|---------|
| `relation` | `string`                  | Edges with this relation (omitted = all relations) |
| `meta`     | `Record<string, unknown>` | Edges whose metadata contains all key/value pairs |

```typescript
const allEdges = store.listEdges();
const knowsEdges = store.listEdges({ relation: 'knows' });
const inferred = store.listEdges({ meta: { source: 'inferred' } });
```

### snapshot

```typescript
store.snapshot(): Snapshot
```

Returns all live nodes and all live edges in deterministic order. The `Snapshot` object is JSON-serializable.

```typescript
const snap = store.snapshot();
const encoded = JSON.stringify(snap);
console.log(`${snap.nodes.length} nodes, ${snap.edges.length} edges`);
```

## Recency helpers

Recency helpers return records sorted newest-first by `updatedAt` (Unix microseconds). Tie-breaker for nodes: `createdAt` desc, `type` asc, `id` asc. Tie-breaker for edges: `createdAt` desc, `from.type` asc, `from.id` asc, `relation` asc, `to.type` asc, `to.id` asc.

Time-window bounds are inclusive: `sinceUpdatedAt <= updatedAt <= untilUpdatedAt`. Timestamps are Unix microseconds, matching `Node.updatedAt` and `Edge.updatedAt`.

```typescript
const recentNodes = store.recentNodes({ type: 'task', tag: 'active', limit: 20 });

const taskRef = { type: 'task', id: 't1' };
const recentEdges = store.recentEdges({ from: taskRef, relation: 'depends_on', limit: 20 });
```

`limit` omitted or `0` means unlimited. Positive `limit` caps results after filtering and sorting. Negative `limit` throws `InvalidInputError`.

These helpers are not cursor-pagination APIs; callers that need duplicate-free checkpoint pagination should track a full cursor separately.

## Edge reconciliation

`reconcileOutboundEdges` synchronizes the outbound edges for a source node and relation to exactly the desired target set. Missing desired edges are added; stale edges (same source+relation, not in desired) are removed; edges for other relations or other source nodes are unchanged.

```typescript
const result = store.reconcileOutboundEdges(alice, 'knows', [bob], { strength: 0.8 });
console.log(result.added, result.removed, result.unchanged);
```

## Cascade delete

Normal `deleteNode` rejects nodes with live edges. `deleteNodeCascade` is an explicit opt-in helper that deletes all inbound and outbound edges first, then deletes the node. It is auditable: the returned `CascadeDeleteResult` reports how many edges and nodes were deleted.

```typescript
const result = store.deleteNodeCascade('person', 'alice');
console.log(result.deletedInboundEdges, result.deletedOutboundEdges, result.deletedNode);
```

`deleteNode` behavior is unchanged — it still rejects nodes with live edges. Only callers that explicitly call `deleteNodeCascade` get cascade behavior.

## Concurrency and single-writer semantics

**One active writer per `.akg` file.** Only one process should have a store open for writing at a time.

Mutations (`putNode`, `putEdge`, `deleteNode`, `deleteEdge`) are held in memory until `commit` or `close` is called. They are not visible to other processes until after a successful commit or close.

Opening the same file from two processes simultaneously, or concurrent access without external serialization, produces undefined behavior — there is no lock file or advisory lock. If you need concurrent access, serialize writes at the application layer.

Cross-platform lock-file or advisory locking is an explicit future enhancement.

## Run the example

```sh
npx tsx examples/basic.ts
```
