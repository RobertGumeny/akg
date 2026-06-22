import { readFileSync, existsSync, mkdirSync, openSync, writeSync, fsyncSync, closeSync, renameSync, unlinkSync, statSync } from 'node:fs';
import { dirname, basename, join } from 'node:path';
import { randomBytes } from 'node:crypto';
import { NotFoundError, InvalidInputError, MissingRequiredFieldError } from './errors.js';
import type {
  NodeRef, NodeFields, Node, EdgeFields, Edge,
  EdgeFilter, NodeFilter, Snapshot,
  RecencyFilter, EdgeRecencyFilter,
  ReconcileResult, CascadeDeleteResult,
} from './types.js';
import {
  decodeContainer, encodeContainer, decodeDataEntries, encodeDataEntries,
  decodeBloom, encodeBloom, DataEntry, equalBytes, compareBytes, CURRENT_MAJOR,
} from './internal/format.js';
import {
  decodeWALRecord, encodeWALRecords, WALRecord,
  WAL_OP_PUT_NODE, WAL_OP_DELETE_NODE, WAL_OP_PUT_EDGE, WAL_OP_DELETE_EDGE, WAL_OP_COMMIT,
} from './internal/wal.js';
import {
  decodeNodePayload, decodeNodePutPayload, encodeNodePayload, encodeNodePutPayload, encodeNodeDeletePayload, decodeNodeDeletePayload,
  decodeEdgePayload, decodeEdgePutPayload, encodeEdgePayload, encodeEdgeDeletePayload, decodeEdgeDeletePayload,
  validateWALPayload,
  CoreNode, CoreEdge,
} from './internal/codec.js';
import {
  validateComponent, validateTag, validateNodeID,
  buildNodeKey, buildEdgeKey, buildEdgeIndexKey, buildTagKeyForMajor, buildTemporalNodeKey, buildTemporalEdgeKey,
  parseNodeKey, parseEdgeKey, parseEdgeIndexKey, parseTagKey, parseTemporalKey,
} from './internal/keys.js';

const MAX_TAGS = 32;

interface NodeRecord {
  id: string;
  node: CoreNode;
}

interface NodeIdentity {
  type: string;
  id: string;
}

interface EdgeIdentity {
  fromType: string;
  from: string;
  relation: string;
  toType: string;
  to: string;
}

interface StoreState {
  nodes: Map<string, NodeRecord>;
  edges: Map<string, CoreEdge>;
  // Secondary in-memory indexes derived from nodes/edges (PERF-1). They turn
  // listNodesByTag / outboundEdges / inboundEdges from O(total) full scans into
  // O(matches) lookups. Pure derived state, rebuilt at load from the same primary
  // records the persisted derived keys validate — no format change. Every mutation
  // path keeps them consistent.
  tagIndex: Map<string, Set<string>>; // tag -> node keys
  outIndex: Map<string, Set<string>>; // from-node key -> edge keys
  inIndex: Map<string, Set<string>>; // to-node key -> edge keys
}

function nodeKey(ident: NodeIdentity): string {
  return `${ident.type}\0${ident.id}`;
}

function edgeKey(ident: EdgeIdentity): string {
  return `${ident.fromType}\0${ident.from}\0${ident.relation}\0${ident.toType}\0${ident.to}`;
}

function nowMicros(): bigint {
  return BigInt(Date.now()) * 1000n;
}

// Overridable in tests. Set to a function returning a fixed bigint to pin time.
// Usage in tests: import { _setTestNow } from '../src/store.js'; _setTestNow(100n);
export let _testNow: (() => bigint) | null = null;
export function _setTestNow(ts: bigint | null): void {
  _testNow = ts === null ? null : () => ts;
}

function clock(): bigint {
  return _testNow ? _testNow() : nowMicros();
}

function newStoreState(): StoreState {
  return {
    nodes: new Map(),
    edges: new Map(),
    tagIndex: new Map(),
    outIndex: new Map(),
    inIndex: new Map(),
  };
}

// --- secondary-index maintenance (PERF-1) -----------------------------------
// All helpers are idempotent at the set level, so callers may re-add an entry.

function addToSet(index: Map<string, Set<string>>, key: string, value: string): void {
  let set = index.get(key);
  if (!set) {
    set = new Set();
    index.set(key, set);
  }
  set.add(value);
}

function removeFromSet(index: Map<string, Set<string>>, key: string, value: string): void {
  const set = index.get(key);
  if (!set) return;
  set.delete(value);
  if (set.size === 0) index.delete(key);
}

function indexAddTags(state: StoreState, nodeK: string, tags: string[]): void {
  for (const tag of tags) addToSet(state.tagIndex, tag, nodeK);
}

function indexRemoveTags(state: StoreState, nodeK: string, tags: string[]): void {
  for (const tag of tags) removeFromSet(state.tagIndex, tag, nodeK);
}

function indexAddEdge(state: StoreState, e: CoreEdge, edgeK: string): void {
  addToSet(state.outIndex, nodeKey({ type: e.fromType, id: e.fromNode }), edgeK);
  addToSet(state.inIndex, nodeKey({ type: e.toType, id: e.toNode }), edgeK);
}

function indexRemoveEdge(state: StoreState, e: CoreEdge, edgeK: string): void {
  removeFromSet(state.outIndex, nodeKey({ type: e.fromType, id: e.fromNode }), edgeK);
  removeFromSet(state.inIndex, nodeKey({ type: e.toType, id: e.toNode }), edgeK);
}

function putNodeInState(state: StoreState, id: string, n: CoreNode, now: bigint): NodeRecord {
  const ident = nodeKey({ type: n.type, id });
  const existing = state.nodes.get(ident);
  const node: CoreNode = { ...n };
  if (existing) {
    node.createdAt = existing.node.createdAt;
    node.updatedAt = now;
    node.version = existing.node.version + 1;
  } else {
    node.createdAt = now;
    node.updatedAt = now;
    node.version = 1;
  }
  if (!node.meta) node.meta = {};
  if (!node.tags) node.tags = [];
  if (existing) indexRemoveTags(state, ident, existing.node.tags);
  const rec = { id, node };
  state.nodes.set(ident, rec);
  indexAddTags(state, ident, node.tags);
  return rec;
}

function putEdgeInState(state: StoreState, e: CoreEdge, now: bigint): CoreEdge {
  const ident = edgeKey({ fromType: e.fromType, from: e.fromNode, relation: e.relation, toType: e.toType, to: e.toNode });
  const existing = state.edges.get(ident);
  const edge: CoreEdge = { ...e };
  if (existing) {
    edge.createdAt = existing.createdAt;
    edge.updatedAt = now;
    edge.version = existing.version + 1;
  } else {
    edge.createdAt = now;
    edge.updatedAt = now;
    edge.version = 1;
  }
  if (!edge.meta) edge.meta = {};
  state.edges.set(ident, edge);
  indexAddEdge(state, edge, ident); // idempotent; identity is stable across replace
  return edge;
}

function validateTagsArray(tags: string[]): void {
  if (tags.length > MAX_TAGS) throw new InvalidInputError('too many tags');
  const seen = new Set<string>();
  for (const tag of tags) {
    if (seen.has(tag)) throw new InvalidInputError('duplicate tags');
    seen.add(tag);
    validateTag(tag);
  }
}

function generateNodeID(): string {
  return randomBytes(8).toString('hex');
}

interface PendingRecord {
  op: number;
  payload: Uint8Array;
}

// Writer-side flush thresholds (spec docs/spec/05-wal.md:116-121, mirrored from
// the Go reference internal/store/file.go:16-17). The first threshold reached
// wins. This is a durability safety valve, not a compaction trigger.
const WAL_ENTRY_FLUSH_THRESHOLD = 1000;
const WAL_BYTE_FLUSH_THRESHOLD = 10 * 1024 * 1024;

// Per-record WAL framing overhead: 13-byte header + 4-byte trailing CRC. Used to
// estimate pending byte growth for the flush policy.
const WAL_RECORD_OVERHEAD = 13 + 4;

/**
 * A read/write handle to a single AKG knowledge graph file. Mutations are held
 * in memory until commit() or close() persists them. Open one via the `open`
 * factory; one active writer per file.
 */
export class Store {
  private path: string;
  private state: StoreState;
  private pending: PendingRecord[];
  private pendingBytes: number;
  private nextWALSeq: bigint;
  private closed: boolean;
  // Byte/entry length of the persisted WAL prefix up to and including the last
  // COMMIT record. commit() appends new records after walAppendBytes instead of
  // rewriting the WAL. compact() resets both to 0.
  private walAppendBytes: number;
  private walAppendEntries: number;
  // Totals for the uncompacted WAL after the most recent commit. Drive the
  // flush policy and the hasUncompactedWAL accessor.
  private uncompactedWALEntries: number;
  private uncompactedWALBytes: number;

  private constructor(
    path: string,
    state: StoreState,
    nextWALSeq: bigint,
    walAppendBytes: number,
    walAppendEntries: number,
    uncompactedWALEntries: number,
    uncompactedWALBytes: number,
  ) {
    this.path = path;
    this.state = state;
    this.pending = [];
    this.pendingBytes = 0;
    this.nextWALSeq = nextWALSeq;
    this.closed = false;
    this.walAppendBytes = walAppendBytes;
    this.walAppendEntries = walAppendEntries;
    this.uncompactedWALEntries = uncompactedWALEntries;
    this.uncompactedWALBytes = uncompactedWALBytes;
  }

  static async open(path: string): Promise<Store> {
    if (!existsSync(path)) {
      const dir = dirname(path);
      mkdirSync(dir, { recursive: true });
      const st = new Store(path, newStoreState(), 1n, 0, 0, 0, 0);
      st.writeSnapshot();
      return st;
    }
    const file = readFileSync(path);
    const bytes = new Uint8Array(file.buffer, file.byteOffset, file.byteLength);
    return Store.fromBytes(bytes, path);
  }

  static fromBytes(file: Uint8Array, path: string): Store {
    const c = decodeContainer(file);
    const entries = decodeDataEntries(c.data);

    if (c.bloom !== null) {
      decodeBloom(c.bloom);
      const keys = entries.map(e => e.key);
      const expected = encodeBloom(keys);
      if (!equalBytes(c.bloom, expected)) {
        throw new InvalidInputError('invalid bloom section');
      }
    }

    const state = hydrateDataEntries(entries, c.major);
    const info = inspectAndReplayWAL(state, c.wal);
    const walBytes = c.wal ? c.wal.length : 0;
    return new Store(path, state, info.next, info.appendBytes, info.appendEntries, info.entries, walBytes);
  }

  // ---- write operations (sync, in-memory) ---------------------------------

  /**
   * Writes or replaces the node at (typeName, id). `fields.title` is required;
   * an empty `id` generates a new id. Returns a NodeRef usable with putEdge.
   * Throws synchronously on validation errors. Held in memory until commit/close.
   */
  putNode(typeName: string, id: string, fields: NodeFields, tags: string[]): NodeRef {
    if (this.closed) throw new InvalidInputError('store is closed');
    validateComponent(typeName);
    if (!fields.title) throw new MissingRequiredFieldError('title is required');
    const actualID = id || generateNodeID();
    validateNodeID(actualID);
    validateTagsArray(tags);

    const n: CoreNode = {
      type: typeName,
      title: fields.title,
      body: fields.body ?? '',
      meta: fields.meta ? { ...fields.meta } : {},
      tags: [...tags],
      createdAt: 0n,
      updatedAt: 0n,
      version: 1,
    };

    const rec = putNodeInState(this.state, actualID, n, clock());
    const payload = encodeNodePutPayload({ id: rec.id, node: rec.node });
    this.stagePending(WAL_OP_PUT_NODE, payload);
    return { type: typeName, id: rec.id };
  }

  /** Returns the node at (typeName, id), or null (not an error) if it does not exist. */
  getNode(typeName: string, id: string): Node | null {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(typeName, id);
    const k = nodeKey({ type: typeName, id });
    const rec = this.state.nodes.get(k);
    if (!rec) return null;
    return nodeFromRecord(rec);
  }

  /** Returns all nodes carrying the given tag, sorted by key. */
  listNodesByTag(tag: string): Node[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    validateTag(tag);
    // Index lookup: O(nodes carrying tag), not a full O(total nodes) scan.
    const matches: NodeRecord[] = [];
    for (const nk of this.state.tagIndex.get(tag) ?? []) {
      const rec = this.state.nodes.get(nk);
      if (rec) matches.push(rec);
    }
    return sortAndMapNodes(matches);
  }

  /** Returns all nodes, optionally filtered to typeName (omitted = all types). Unknown type returns empty. Sorted by key. */
  listNodes(typeName?: string): Node[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (typeName) validateComponent(typeName);
    const matches: NodeRecord[] = [];
    for (const rec of this.state.nodes.values()) {
      if (typeName && rec.node.type !== typeName) continue;
      matches.push(rec);
    }
    return sortAndMapNodes(matches);
  }

  /**
   * Writes or replaces the edge at (fromRef, relation, toRef). Both endpoints
   * must already exist, or NotFoundError is thrown. Held in memory until commit/close.
   */
  putEdge(fromRef: NodeRef, relation: string, toRef: NodeRef, fields: EdgeFields): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (!this.state.nodes.has(nodeKey({ type: fromRef.type, id: fromRef.id }))) {
      throw new NotFoundError(`node ${fromRef.type}/${fromRef.id} not found`);
    }
    if (!this.state.nodes.has(nodeKey({ type: toRef.type, id: toRef.id }))) {
      throw new NotFoundError(`node ${toRef.type}/${toRef.id} not found`);
    }
    validateComponent(relation);

    const e: CoreEdge = {
      fromType: fromRef.type,
      fromNode: fromRef.id,
      toType: toRef.type,
      toNode: toRef.id,
      relation,
      strength: fields.strength ?? 0.5,
      confidence: fields.confidence !== undefined ? fields.confidence : null,
      meta: fields.meta ? { ...fields.meta } : {},
      createdAt: 0n,
      updatedAt: 0n,
      version: 1,
    };

    const rec = putEdgeInState(this.state, e, clock());
    const payload = encodeEdgePutPayload(rec);
    this.stagePending(WAL_OP_PUT_EDGE, payload);
  }

  /** Returns edges originating at nodeRef, optionally filtered to a relation (omitted = all). */
  outboundEdges(nodeRef: NodeRef, relation?: string): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(nodeRef.type, nodeRef.id);
    if (relation) validateComponent(relation);
    // Index lookup: O(out-degree of nodeRef), not a full O(total edges) scan.
    const matches: CoreEdge[] = [];
    for (const ek of this.state.outIndex.get(nodeKey({ type: nodeRef.type, id: nodeRef.id })) ?? []) {
      const e = this.state.edges.get(ek);
      if (!e) continue;
      if (relation && e.relation !== relation) continue;
      matches.push(e);
    }
    return sortAndMapEdgesByKey(matches);
  }

  /** Returns edges pointing at nodeRef, optionally filtered to a relation (omitted = all). */
  inboundEdges(nodeRef: NodeRef, relation?: string): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(nodeRef.type, nodeRef.id);
    if (relation) validateComponent(relation);
    // Index lookup: O(in-degree of nodeRef), not a full O(total edges) scan.
    const matches: CoreEdge[] = [];
    for (const ek of this.state.inIndex.get(nodeKey({ type: nodeRef.type, id: nodeRef.id })) ?? []) {
      const e = this.state.edges.get(ek);
      if (!e) continue;
      if (relation && e.relation !== relation) continue;
      matches.push(e);
    }
    return sortAndMapEdgesByIndexKey(matches);
  }

  /**
   * Deletes the node at (typeName, id). Throws NotFoundError if it does not exist,
   * or InvalidInputError if it still has live edges — delete those edges first.
   */
  deleteNode(typeName: string, id: string): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(typeName, id);
    const k = nodeKey({ type: typeName, id });
    const rec = this.state.nodes.get(k);
    if (!rec) throw new NotFoundError(`node ${typeName}/${id} not found`);
    // O(1) incident-edge check via the indexes (replaces a full edge scan).
    if ((this.state.outIndex.get(k)?.size ?? 0) > 0 || (this.state.inIndex.get(k)?.size ?? 0) > 0) {
      throw new InvalidInputError('node has live edges; delete edges first');
    }
    this.state.nodes.delete(k);
    indexRemoveTags(this.state, k, rec.node.tags);
    const payload = encodeNodeDeletePayload({ type: typeName, id });
    this.stagePending(WAL_OP_DELETE_NODE, payload);
  }

  /** Deletes the edge at (fromRef, relation, toRef). Throws NotFoundError if it does not exist. */
  deleteEdge(fromRef: NodeRef, relation: string, toRef: NodeRef): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(fromRef.type, fromRef.id);
    buildNodeKey(toRef.type, toRef.id);
    validateComponent(relation);
    const k = edgeKey({ fromType: fromRef.type, from: fromRef.id, relation, toType: toRef.type, to: toRef.id });
    const edge = this.state.edges.get(k);
    if (!edge) throw new NotFoundError(`edge not found`);
    this.state.edges.delete(k);
    indexRemoveEdge(this.state, edge, k);
    const payload = encodeEdgeDeletePayload({ fromType: fromRef.type, fromNode: fromRef.id, relation, toType: toRef.type, toNode: toRef.id });
    this.stagePending(WAL_OP_DELETE_EDGE, payload);
  }

  // ---- global edge listing and snapshots ---------------------------------

  /** Returns all edges, optionally narrowed by an EdgeFilter (relation and/or meta). Sorted by key. */
  listEdges(filter?: EdgeFilter): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (filter?.relation) validateComponent(filter.relation);
    const matches: CoreEdge[] = [];
    for (const e of this.state.edges.values()) {
      if (filter?.relation && e.relation !== filter.relation) continue;
      if (filter?.meta && !metaMatches(e.meta, filter.meta)) continue;
      matches.push(e);
    }
    return sortAndMapEdgesByKey(matches);
  }

  /** Returns all live nodes and edges in deterministic order as a JSON-serializable Snapshot. */
  snapshot(): Snapshot {
    if (this.closed) throw new InvalidInputError('store is closed');
    return {
      nodes: this.listNodes(),
      edges: this.listEdges(),
    };
  }

  // ---- node filtering and batch inspection --------------------------------

  /**
   * Returns nodes matching the NodeFilter (type, tag, and/or meta) with AND
   * semantics. Unknown types or tags return empty results, not errors.
   */
  listNodesFiltered(filter: NodeFilter): Node[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (filter.type) validateComponent(filter.type);
    if (filter.tag) validateTag(filter.tag);
    const matches: NodeRecord[] = [];
    for (const rec of this.state.nodes.values()) {
      if (filter.type && rec.node.type !== filter.type) continue;
      if (filter.tag && !rec.node.tags.includes(filter.tag)) continue;
      if (filter.meta && !metaMatches(rec.node.meta, filter.meta)) continue;
      matches.push(rec);
    }
    return sortAndMapNodes(matches);
  }

  /**
   * Batch lookup: returns one output position per input ref, in input order,
   * preserving duplicates. A position is null where the referenced node is missing.
   */
  getNodes(refs: NodeRef[]): Array<Node | null> {
    if (this.closed) throw new InvalidInputError('store is closed');
    return refs.map(ref => {
      buildNodeKey(ref.type, ref.id);
      const k = nodeKey({ type: ref.type, id: ref.id });
      const rec = this.state.nodes.get(k);
      return rec ? nodeFromRecord(rec) : null;
    });
  }

  // ---- recency helpers ----------------------------------------------------

  /**
   * Returns nodes newest-first by updatedAt (tie-broken by createdAt desc, type
   * asc, id asc), filtered by the RecencyFilter. limit 0/omitted = unlimited;
   * negative throws InvalidInputError.
   */
  recentNodes(filter?: RecencyFilter): Node[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    const limit = filter?.limit ?? 0;
    if (limit < 0) throw new InvalidInputError('limit must be non-negative');
    if (filter?.type) validateComponent(filter.type);
    if (filter?.tag) validateTag(filter.tag);

    const matches: NodeRecord[] = [];
    for (const rec of this.state.nodes.values()) {
      if (filter?.type && rec.node.type !== filter.type) continue;
      if (filter?.tag && !rec.node.tags.includes(filter.tag)) continue;
      const ua = Number(rec.node.updatedAt);
      if (filter?.sinceUpdatedAt && ua < filter.sinceUpdatedAt) continue;
      if (filter?.untilUpdatedAt && ua > filter.untilUpdatedAt) continue;
      matches.push(rec);
    }

    const enc = new TextEncoder();
    matches.sort((a, b) => {
      const ua = Number(a.node.updatedAt), ub = Number(b.node.updatedAt);
      if (ua !== ub) return ub - ua;
      const ca = Number(a.node.createdAt), cb = Number(b.node.createdAt);
      if (ca !== cb) return cb - ca;
      if (a.node.type !== b.node.type) return a.node.type < b.node.type ? -1 : 1;
      const ak = enc.encode(buildNodeKey(a.node.type, a.id));
      const bk = enc.encode(buildNodeKey(b.node.type, b.id));
      return compareBytes(ak, bk);
    });

    const result = limit > 0 ? matches.slice(0, limit) : matches;
    return result.map(nodeFromRecord);
  }

  /**
   * Returns edges newest-first by updatedAt (tie-broken by createdAt desc then
   * endpoint/relation order), filtered by the EdgeRecencyFilter. limit 0/omitted
   * = unlimited; negative throws InvalidInputError.
   */
  recentEdges(filter?: EdgeRecencyFilter): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    const limit = filter?.limit ?? 0;
    if (limit < 0) throw new InvalidInputError('limit must be non-negative');
    if (filter?.relation) validateComponent(filter.relation);
    if (filter?.from) buildNodeKey(filter.from.type, filter.from.id);
    if (filter?.to) buildNodeKey(filter.to.type, filter.to.id);

    const matches: CoreEdge[] = [];
    for (const e of this.state.edges.values()) {
      if (filter?.relation && e.relation !== filter.relation) continue;
      if (filter?.from && (e.fromType !== filter.from.type || e.fromNode !== filter.from.id)) continue;
      if (filter?.to && (e.toType !== filter.to.type || e.toNode !== filter.to.id)) continue;
      const ua = Number(e.updatedAt);
      if (filter?.sinceUpdatedAt && ua < filter.sinceUpdatedAt) continue;
      if (filter?.untilUpdatedAt && ua > filter.untilUpdatedAt) continue;
      matches.push(e);
    }

    matches.sort((a, b) => {
      const ua = Number(a.updatedAt), ub = Number(b.updatedAt);
      if (ua !== ub) return ub - ua;
      const ca = Number(a.createdAt), cb = Number(b.createdAt);
      if (ca !== cb) return cb - ca;
      if (a.fromType !== b.fromType) return a.fromType < b.fromType ? -1 : 1;
      if (a.fromNode !== b.fromNode) return a.fromNode < b.fromNode ? -1 : 1;
      if (a.relation !== b.relation) return a.relation < b.relation ? -1 : 1;
      if (a.toType !== b.toType) return a.toType < b.toType ? -1 : 1;
      return a.toNode < b.toNode ? -1 : 1;
    });

    const result = limit > 0 ? matches.slice(0, limit) : matches;
    return result.map(edgeFromCoreEdge);
  }

  // ---- edge reconciliation ------------------------------------------------

  /**
   * Synchronizes the outbound edges from `source` for `relation` to exactly the
   * `desired` target set: missing edges are added, stale ones removed, edges for
   * other relations or sources are untouched. Returns the add/remove/unchanged counts.
   */
  reconcileOutboundEdges(source: NodeRef, relation: string, desired: NodeRef[], fields: EdgeFields): ReconcileResult {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(source.type, source.id);
    if (!this.state.nodes.has(nodeKey({ type: source.type, id: source.id }))) {
      throw new NotFoundError(`node ${source.type}/${source.id} not found`);
    }
    validateComponent(relation);
    for (const d of desired) buildNodeKey(d.type, d.id);

    const desiredSet = new Set(desired.map(d => nodeKey({ type: d.type, id: d.id })));

    const existing = new Map<string, NodeRef>();
    for (const [, e] of this.state.edges) {
      if (e.fromType === source.type && e.fromNode === source.id && e.relation === relation) {
        existing.set(nodeKey({ type: e.toType, id: e.toNode }), { type: e.toType, id: e.toNode });
      }
    }

    let added = 0, removed = 0, unchanged = 0;

    for (const [nk, ref] of existing) {
      if (!desiredSet.has(nk)) {
        const ek = edgeKey({ fromType: source.type, from: source.id, relation, toType: ref.type, to: ref.id });
        this.state.edges.delete(ek);
        const payload = encodeEdgeDeletePayload({ fromType: source.type, fromNode: source.id, relation, toType: ref.type, toNode: ref.id });
        this.stagePending(WAL_OP_DELETE_EDGE, payload);
        removed++;
      }
    }

    for (const d of desired) {
      const nk = nodeKey({ type: d.type, id: d.id });
      if (existing.has(nk)) {
        unchanged++;
      } else {
        if (!this.state.nodes.has(nk)) throw new NotFoundError(`node ${d.type}/${d.id} not found`);
        const e: CoreEdge = {
          fromType: source.type, fromNode: source.id,
          toType: d.type, toNode: d.id,
          relation,
          strength: fields.strength ?? 0.5,
          confidence: fields.confidence !== undefined ? fields.confidence : null,
          meta: fields.meta ? { ...fields.meta } : {},
          createdAt: 0n, updatedAt: 0n, version: 1,
        };
        const rec = putEdgeInState(this.state, e, clock());
        const payload = encodeEdgePutPayload(rec);
        this.stagePending(WAL_OP_PUT_EDGE, payload);
        added++;
      }
    }

    return { added, removed, unchanged };
  }

  // ---- cascade delete -----------------------------------------------------

  /**
   * Deletes all inbound and outbound edges of the node at (typeName, id), then
   * deletes the node. Throws NotFoundError if the node does not exist. Returns
   * the counts of edges removed and whether the node was deleted.
   */
  deleteNodeCascade(typeName: string, id: string): CascadeDeleteResult {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(typeName, id);
    const nk = nodeKey({ type: typeName, id });
    const rec = this.state.nodes.get(nk);
    if (!rec) throw new NotFoundError(`node ${typeName}/${id} not found`);

    let deletedInboundEdges = 0, deletedOutboundEdges = 0;

    // Collect incident edge keys from the indexes — O(degree), not a full edge
    // scan — deduping the self-loop case (present in both index sets).
    const seen = new Set<string>();
    const incident: string[] = [];
    for (const ek of this.state.outIndex.get(nk) ?? []) {
      if (!seen.has(ek)) { seen.add(ek); incident.push(ek); }
    }
    for (const ek of this.state.inIndex.get(nk) ?? []) {
      if (!seen.has(ek)) { seen.add(ek); incident.push(ek); }
    }

    for (const ek of incident) {
      const e = this.state.edges.get(ek)!;
      this.state.edges.delete(ek);
      indexRemoveEdge(this.state, e, ek);
      const payload = encodeEdgeDeletePayload({ fromType: e.fromType, fromNode: e.fromNode, relation: e.relation, toType: e.toType, toNode: e.toNode });
      this.stagePending(WAL_OP_DELETE_EDGE, payload);
      if (e.fromType === typeName && e.fromNode === id) deletedOutboundEdges++; else deletedInboundEdges++;
    }

    this.state.nodes.delete(nk);
    indexRemoveTags(this.state, nk, rec.node.tags);
    const payload = encodeNodeDeletePayload({ type: typeName, id });
    this.stagePending(WAL_OP_DELETE_NODE, payload);

    return { deletedInboundEdges, deletedOutboundEdges, deletedNode: true };
  }

  // ---- internal inspection (for testing/conformance) ---------------------

  get hasUncompactedWAL(): boolean {
    return this.walAppendEntries > 0;
  }

  get nextWALSequence(): bigint {
    return this.nextWALSeq;
  }

  /** Number of WAL entries accumulated since the last compaction. */
  get uncompactedWALEntryCount(): number {
    return this.uncompactedWALEntries;
  }

  /** Byte size of the WAL accumulated since the last compaction. */
  get uncompactedWALByteCount(): number {
    return this.uncompactedWALBytes;
  }

  // ---- lifecycle (async, I/O) ---------------------------------------------

  /**
   * Compacts the file: auto-commits any pending mutations, then rewrites it to
   * contain only live records with an empty WAL, discarding tombstones and prior
   * WAL history. If the auto-commit fails, compaction does not run. The store
   * stays usable. Always caller-triggered — never automatic.
   */
  // Compact commits any pending mutations and rewrites the file to contain only
  // live records, discarding all tombstones and prior WAL history. If the
  // auto-commit fails, compaction does not run. After compaction the store
  // remains fully usable. Compaction is never triggered automatically.
  async compact(): Promise<void> {
    if (this.closed) throw new InvalidInputError('store is closed');
    this.commitSync();
    this.writeSnapshot();
  }

  /** Durably persists all pending in-memory mutations by appending them (plus a COMMIT marker) to the file's WAL. */
  async commit(): Promise<void> {
    this.commitSync();
  }

  /** Commits any outstanding mutations and closes the store. Safe to call when nothing is pending; subsequent operations throw. */
  async close(): Promise<void> {
    if (this.closed) return;
    this.commitSync();
    this.closed = true;
  }

  // ---- internal -----------------------------------------------------------

  // stagePending buffers a mutation record and, if the buffered or uncompacted
  // WAL has crossed the spec flush thresholds, commits it automatically. The
  // auto-flush is a durability safety valve (docs/spec/05-wal.md:112-123); it is
  // never a compaction trigger.
  private stagePending(op: number, payload: Uint8Array): void {
    this.pending.push({ op, payload });
    // Each persisted record carries the WAL record framing overhead too.
    this.pendingBytes += payload.length + WAL_RECORD_OVERHEAD;
    if (this.shouldAutoFlush()) {
      this.commitSync();
    }
  }

  private shouldAutoFlush(): boolean {
    const entries = this.uncompactedWALEntries + this.pending.length;
    const bytes = this.uncompactedWALBytes + this.pendingBytes;
    return entries >= WAL_ENTRY_FLUSH_THRESHOLD || bytes >= WAL_BYTE_FLUSH_THRESHOLD;
  }

  // commitSync persists pending mutations by appending only the new WAL records
  // (plus a COMMIT record) onto the existing persisted WAL prefix, reusing the
  // file's Data and Bloom bytes unchanged. Mirrors the Go reference
  // internal/store/file.go:148-193. compact() is the only path that rebuilds
  // Data/Bloom and resets the WAL to empty.
  private commitSync(): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (this.pending.length === 0) return;

    const file = readFileSync(this.path);
    const bytes = new Uint8Array(file.buffer, file.byteOffset, file.byteLength);
    const c = decodeContainer(bytes);
    const existingWAL = c.wal ?? new Uint8Array(0);
    if (this.walAppendBytes > existingWAL.length) {
      throw new InvalidInputError('invalid wal replay: append offset past persisted WAL');
    }
    const walPrefix = existingWAL.slice(0, this.walAppendBytes);

    const records: WALRecord[] = [];
    let next = this.nextWALSeq;
    for (const p of this.pending) {
      records.push({ sequence: next, operation: p.op, payload: p.payload });
      next++;
    }
    records.push({ sequence: next, operation: WAL_OP_COMMIT, payload: new Uint8Array(0) });
    const encoded = encodeWALRecords(records);

    const newWAL = new Uint8Array(walPrefix.length + encoded.length);
    newWAL.set(walPrefix, 0);
    newWAL.set(encoded, walPrefix.length);

    const newFile = encodeContainer({ data: c.data, bloom: c.bloom, wal: newWAL });
    writeFileAtomic(this.path, newFile);

    this.pending = [];
    this.pendingBytes = 0;
    this.nextWALSeq = next + 1n;
    this.uncompactedWALEntries = this.walAppendEntries + records.length;
    this.uncompactedWALBytes = newWAL.length;
    this.walAppendEntries = this.uncompactedWALEntries;
    this.walAppendBytes = this.uncompactedWALBytes;
  }

  // writeSnapshot rebuilds Data/Bloom from current live state and writes a file
  // with an empty WAL section. Used for fresh-file creation and compaction.
  private writeSnapshot(): void {
    const entries = materializeDataEntries(this.state);
    const data = encodeDataEntries(entries);
    const keys = entries.map(e => e.key);
    const bloom = encodeBloom(keys);
    const file = encodeContainer({ data, bloom, wal: new Uint8Array(0) });
    writeFileAtomic(this.path, file);
    this.nextWALSeq = 1n;
    this.walAppendBytes = 0;
    this.walAppendEntries = 0;
    this.uncompactedWALEntries = 0;
    this.uncompactedWALBytes = 0;
  }
}

// ---- Open factory ----------------------------------------------------------

/** Opens an existing .akg file or creates a new empty one if the path does not exist. Rejects if the file exists but is malformed. */
export async function open(path: string): Promise<Store> {
  return Store.open(path);
}

// ---- Hydration and materialization -----------------------------------------

// hydrateDataEntries reconstructs live state from decoded Data entries. major is
// the file's binary major: it selects which tag-key shape (v1 3-part / v2 4-part)
// the derived-key validation re-derives, so a major-1 file validates against v1.
function hydrateDataEntries(entries: DataEntry[], major: number): StoreState {
  const dec = new TextDecoder('utf-8');

  const state = newStoreState();
  for (const entry of entries) {
    const key = dec.decode(entry.key);
    if (key.startsWith('n:')) {
      const parsed = parseNodeKey(key);
      let node: CoreNode;
      try {
        node = decodeNodePayload(entry.value);
      } catch (e) {
        throw wrapDataPayloadError(e);
      }
      if (node.type !== parsed.type) throw new InvalidInputError('node type mismatch in data key');
      const nk = nodeKey({ type: parsed.type, id: parsed.id });
      state.nodes.set(nk, { id: parsed.id, node });
      indexAddTags(state, nk, node.tags ?? []);
    } else if (key.startsWith('e:')) {
      const parsed = parseEdgeKey(key);
      let edge: CoreEdge;
      try {
        edge = decodeEdgePayload(entry.value);
      } catch (e) {
        throw wrapDataPayloadError(e);
      }
      if (edge.fromType !== parsed.fromType || edge.fromNode !== parsed.fromID || edge.relation !== parsed.relation || edge.toType !== parsed.toType || edge.toNode !== parsed.toID) {
        throw new InvalidInputError('edge identity mismatch in data key');
      }
      const ek = edgeKey({ fromType: edge.fromType, from: edge.fromNode, relation: edge.relation, toType: edge.toType, to: edge.toNode });
      state.edges.set(ek, edge);
      indexAddEdge(state, edge, ek);
    } else if (key.startsWith('t:')) {
      if (entry.value.length !== 0) throw new InvalidInputError('tag key must have empty value');
      parseTagKey(key);
    } else if (key.startsWith('ts:')) {
      if (entry.value.length !== 0) throw new InvalidInputError('temporal key must have empty value');
      parseTemporalKey(key);
    } else if (key.startsWith('ei:')) {
      if (entry.value.length !== 0) throw new InvalidInputError('edge index key must have empty value');
      parseEdgeIndexKey(key);
    } else {
      throw new InvalidInputError(`unknown key prefix: ${key}`);
    }
  }
  validateDerivedKeys(state, entries, major);
  return state;
}

function wrapDataPayloadError(e: unknown): Error {
  if (e instanceof MissingRequiredFieldError) return new MissingRequiredFieldError(`invalid data payload: ${e.message}`);
  return new InvalidInputError(`invalid data payload: ${e instanceof Error ? e.message : String(e)}`);
}

function validateDerivedKeys(state: StoreState, entries: DataEntry[], major: number): void {
  const expected = materializeDataEntriesForMajor(state, major);
  if (expected.length !== entries.length) throw new InvalidInputError('derived index mismatch');
  for (let i = 0; i < expected.length; i++) {
    if (!equalBytes(expected[i].key, entries[i].key)) throw new InvalidInputError('derived index mismatch');
  }
}

// materializeDataEntries derives the live Data key set at the current binary
// major (writers always write major 2).
export function materializeDataEntries(state: StoreState): DataEntry[] {
  return materializeDataEntriesForMajor(state, CURRENT_MAJOR);
}

// materializeDataEntriesForMajor derives the live Data key set as it must appear
// in a file of the given binary major. major selects the tag-key shape via
// buildTagKeyForMajor; everything else is major-independent.
function materializeDataEntriesForMajor(state: StoreState, major: number): DataEntry[] {
  const enc = new TextEncoder();
  const entries: DataEntry[] = [];
  const seen = new Set<string>();

  const add = (key: string, value: Uint8Array) => {
    if (seen.has(key)) throw new InvalidInputError(`duplicate data key: ${key}`);
    seen.add(key);
    entries.push({ key: enc.encode(key), value });
  };

  for (const rec of state.nodes.values()) {
    const key = buildNodeKey(rec.node.type, rec.id);
    const value = encodeNodePayload(rec.node);
    add(key, value);

    for (const tag of rec.node.tags) {
      add(buildTagKeyForMajor(major, tag, rec.node.type, rec.id), new Uint8Array(0));
    }
    add(buildTemporalNodeKey(rec.node.updatedAt, rec.node.type, rec.id), new Uint8Array(0));
  }

  for (const edge of state.edges.values()) {
    const key = buildEdgeKey(edge.fromType, edge.fromNode, edge.relation, edge.toType, edge.toNode);
    const value = encodeEdgePayload(edge);
    add(key, value);
    add(buildEdgeIndexKey(edge.toType, edge.toNode, edge.relation, edge.fromType, edge.fromNode), new Uint8Array(0));
    add(buildTemporalEdgeKey(edge.updatedAt, edge.fromType, edge.fromNode, edge.relation, edge.toType, edge.toNode), new Uint8Array(0));
  }

  entries.sort((a, b) => compareBytes(a.key, b.key));
  return entries;
}

interface WALInfo {
  next: bigint;        // next WAL sequence number to assign
  entries: number;     // total decoded WAL records (committed + trailing)
  appendBytes: number; // byte length of the WAL prefix up to & incl. last COMMIT
  appendEntries: number; // record count of that prefix
}

function inspectAndReplayWAL(state: StoreState, walPayload: Uint8Array | null): WALInfo {
  if (!walPayload || walPayload.length === 0) {
    return { next: 1n, entries: 0, appendBytes: 0, appendEntries: 0 };
  }

  let next = 1n;
  let entries = 0;
  const all: WALRecord[] = [];
  let lastCommit = -1;
  let lastCommitEnd = 0;
  let pos = 0;

  while (pos < walPayload.length) {
    let r: WALRecord;
    let n: number;
    try {
      [r, n] = decodeWALRecord(walPayload.slice(pos));
    } catch (e) {
      if (lastCommit >= 0) break;
      throw e;
    }
    all.push(r);
    entries++;
    if (r.sequence >= next) next = r.sequence + 1n;
    if (r.operation === WAL_OP_COMMIT) {
      lastCommit = all.length - 1;
      lastCommitEnd = pos + n;
    }
    pos += n;
  }

  if (lastCommit < 0) return { next, entries, appendBytes: 0, appendEntries: 0 };

  const committed = all.slice(0, lastCommit + 1);
  let prev = 0n;
  for (let i = 0; i < committed.length; i++) {
    const r = committed[i];
    if (i > 0 && r.sequence <= prev) throw new InvalidInputError('invalid wal record: non-increasing sequence numbers');
    prev = r.sequence;

    try {
      validateWALPayload(r.operation, r.payload);
    } catch (e) {
      if (e instanceof MissingRequiredFieldError) {
        throw new MissingRequiredFieldError(`invalid wal payload: ${e.message}`);
      }
      throw new InvalidInputError(`invalid wal payload: ${e instanceof Error ? e.message : String(e)}`);
    }

    switch (r.operation) {
      case WAL_OP_PUT_NODE: {
        const put = decodeNodePutPayload(r.payload);
        const nk = nodeKey({ type: put.node.type, id: put.id });
        // A later PUT_NODE can replace an earlier one with different tags.
        const prev = state.nodes.get(nk);
        if (prev) indexRemoveTags(state, nk, prev.node.tags ?? []);
        state.nodes.set(nk, { id: put.id, node: put.node });
        indexAddTags(state, nk, put.node.tags ?? []);
        break;
      }
      case WAL_OP_DELETE_NODE: {
        const d = decodeNodeDeletePayload(r.payload);
        const nk = nodeKey({ type: d.type, id: d.id });
        const prev = state.nodes.get(nk);
        if (prev) indexRemoveTags(state, nk, prev.node.tags ?? []);
        state.nodes.delete(nk);
        break;
      }
      case WAL_OP_PUT_EDGE: {
        const e = decodeEdgePutPayload(r.payload);
        const ek = edgeKey({ fromType: e.fromType, from: e.fromNode, relation: e.relation, toType: e.toType, to: e.toNode });
        state.edges.set(ek, e);
        indexAddEdge(state, e, ek);
        break;
      }
      case WAL_OP_DELETE_EDGE: {
        const d = decodeEdgeDeletePayload(r.payload);
        const ek = edgeKey({ fromType: d.fromType, from: d.fromNode, relation: d.relation, toType: d.toType, to: d.toNode });
        const prev = state.edges.get(ek);
        state.edges.delete(ek);
        if (prev) indexRemoveEdge(state, prev, ek);
        break;
      }
    }
  }

  return { next, entries, appendBytes: lastCommitEnd, appendEntries: lastCommit + 1 };
}

// ---- Node/Edge conversions -------------------------------------------------

function nodeFromRecord(rec: NodeRecord): Node {
  return {
    type: rec.node.type,
    id: rec.id,
    title: rec.node.title,
    body: rec.node.body,
    meta: { ...rec.node.meta },
    tags: [...rec.node.tags],
    createdAt: Number(rec.node.createdAt),
    updatedAt: Number(rec.node.updatedAt),
    version: rec.node.version,
  };
}

function edgeFromCoreEdge(e: CoreEdge): Edge {
  return {
    from: { type: e.fromType, id: e.fromNode },
    relation: e.relation,
    to: { type: e.toType, id: e.toNode },
    strength: e.strength,
    confidence: e.confidence,
    meta: { ...e.meta },
    createdAt: Number(e.createdAt),
    updatedAt: Number(e.updatedAt),
    version: e.version,
  };
}

function encodeEdgePutPayload(e: CoreEdge): Uint8Array {
  return encodeEdgePayload(e);
}

function sortAndMapNodes(recs: NodeRecord[]): Node[] {
  const enc = new TextEncoder();
  recs.sort((a, b) => {
    const ak = enc.encode(buildNodeKey(a.node.type, a.id));
    const bk = enc.encode(buildNodeKey(b.node.type, b.id));
    return compareBytes(ak, bk);
  });
  return recs.map(nodeFromRecord);
}

function sortAndMapEdgesByKey(edges: CoreEdge[]): Edge[] {
  const enc = new TextEncoder();
  edges.sort((a, b) => {
    const ak = enc.encode(buildEdgeKey(a.fromType, a.fromNode, a.relation, a.toType, a.toNode));
    const bk = enc.encode(buildEdgeKey(b.fromType, b.fromNode, b.relation, b.toType, b.toNode));
    return compareBytes(ak, bk);
  });
  return edges.map(edgeFromCoreEdge);
}

function sortAndMapEdgesByIndexKey(edges: CoreEdge[]): Edge[] {
  const enc = new TextEncoder();
  edges.sort((a, b) => {
    const ak = enc.encode(buildEdgeIndexKey(a.toType, a.toNode, a.relation, a.fromType, a.fromNode));
    const bk = enc.encode(buildEdgeIndexKey(b.toType, b.toNode, b.relation, b.fromType, b.fromNode));
    return compareBytes(ak, bk);
  });
  return edges.map(edgeFromCoreEdge);
}

// ---- Metadata deep equality ------------------------------------------------

function metaMatches(meta: Record<string, unknown>, filter: Record<string, unknown>): boolean {
  for (const [k, fv] of Object.entries(filter)) {
    if (!(k in meta)) return false;
    if (!deepEqual(meta[k], fv)) return false;
  }
  return true;
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) {
    // numeric cross-type: compare via JSON
    return JSON.stringify(a) === JSON.stringify(b);
  }
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!deepEqual(a[i], b[i])) return false;
    }
    return true;
  }
  if (typeof a === 'object' && typeof b === 'object' && !Array.isArray(a) && !Array.isArray(b)) {
    const ao = a as Record<string, unknown>;
    const bo = b as Record<string, unknown>;
    const ak = Object.keys(ao);
    const bk = Object.keys(bo);
    if (ak.length !== bk.length) return false;
    for (const k of ak) {
      if (!(k in bo)) return false;
      if (!deepEqual(ao[k], bo[k])) return false;
    }
    return true;
  }
  return JSON.stringify(a) === JSON.stringify(b);
}

// ---- File I/O --------------------------------------------------------------

// writeFileAtomic durably replaces path with data using the crash-atomic
// sequence from the Go reference (internal/store/file.go:446): write a
// same-directory temp file, fsync it, rename it over the target, then fsync the
// directory. A crash at any point before the rename leaves the prior committed
// file fully intact; the rename itself is atomic. On any error before the
// rename, the temp file is removed.
function writeFileAtomic(path: string, data: Uint8Array): void {
  const dir = dirname(path);
  const base = basename(path);

  // Preserve the existing file's permission bits if it is already present.
  let mode = 0o666;
  try {
    mode = statSync(path).mode & 0o777;
  } catch {
    // Target does not exist yet; fall back to the default mode.
  }

  const tmpPath = join(dir, `.${base}.commit-${randomBytes(8).toString('hex')}`);
  // 'wx' fails if the temp path somehow already exists, guaranteeing we never
  // clobber an unrelated file with our half-written bytes.
  const fd = openSync(tmpPath, 'wx', mode);
  try {
    writeSync(fd, data);
    fsyncSync(fd);
    closeSync(fd);
  } catch (e) {
    try { closeSync(fd); } catch { /* fd may already be closed */ }
    try { unlinkSync(tmpPath); } catch { /* best-effort cleanup */ }
    throw e;
  }

  try {
    renameSync(tmpPath, path);
  } catch (e) {
    try { unlinkSync(tmpPath); } catch { /* best-effort cleanup */ }
    throw e;
  }

  // fsync the directory so the rename itself is durable.
  fsyncDir(dir);
}

function fsyncDir(dir: string): void {
  let dfd: number;
  try {
    dfd = openSync(dir, 'r');
  } catch {
    // Some platforms disallow opening a directory for fsync; best-effort only.
    return;
  }
  try {
    fsyncSync(dfd);
  } catch {
    // Directory fsync is a durability hint; ignore platform refusals.
  } finally {
    closeSync(dfd);
  }
}
