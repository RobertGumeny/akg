import { readFileSync, writeFileSync, existsSync, mkdirSync, openSync, writeSync, fsyncSync, closeSync } from 'node:fs';
import { dirname } from 'node:path';
import { randomBytes } from 'node:crypto';
import { NotFoundError, InvalidInputError, MissingRequiredFieldError } from './errors.js';
import type { NodeRef, NodeFields, Node, EdgeFields, Edge } from './types.js';
import {
  decodeContainer, encodeContainer, decodeDataEntries, encodeDataEntries,
  decodeBloom, encodeBloom, DataEntry, equalBytes, compareBytes,
} from './internal/format.js';
import {
  decodeWALRecord, encodeWALRecords, WALRecord,
  WAL_OP_PUT_NODE, WAL_OP_DELETE_NODE, WAL_OP_PUT_EDGE, WAL_OP_DELETE_EDGE, WAL_OP_COMMIT,
} from './internal/wal.js';
import {
  decodeNodePayload, decodeNodePutPayload, encodeNodePayload, encodeNodePutPayload, encodeNodeDeletePayload, decodeNodeDeletePayload,
  decodeEdgePayload, decodeEdgePutPayload, encodeEdgePayload, encodeEdgeDeletePayload, decodeEdgeDeletePayload,
  validateWALPayload,
  CoreNode, CoreEdge, NodePut, NodeDelete, EdgeDelete,
} from './internal/codec.js';
import {
  validateComponent, validateTag, validateNodeID,
  buildNodeKey, buildEdgeKey, buildEdgeIndexKey, buildTagKey, buildTemporalNodeKey, buildTemporalEdgeKey,
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

function newStoreState(): StoreState {
  return { nodes: new Map(), edges: new Map() };
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
  const rec = { id, node };
  state.nodes.set(ident, rec);
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

export class Store {
  private path: string;
  private state: StoreState;
  private pending: PendingRecord[];
  private committedWAL: WALRecord[];
  private nextWALSeq: bigint;
  private closed: boolean;

  private constructor(
    path: string,
    state: StoreState,
    committedWAL: WALRecord[],
    nextWALSeq: bigint,
  ) {
    this.path = path;
    this.state = state;
    this.pending = [];
    this.committedWAL = committedWAL;
    this.nextWALSeq = nextWALSeq;
    this.closed = false;
  }

  static async open(path: string): Promise<Store> {
    if (!existsSync(path)) {
      const dir = dirname(path);
      mkdirSync(dir, { recursive: true });
      const st = new Store(path, newStoreState(), [], 1n);
      await st.writeFile();
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

    const state = hydrateDataEntries(entries);
    const [committedWAL, nextSeq] = inspectAndReplayWAL(state, c.wal);
    return new Store(path, state, committedWAL, nextSeq);
  }

  // ---- write operations (sync, in-memory) ---------------------------------

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

    const rec = putNodeInState(this.state, actualID, n, nowMicros());
    const payload = encodeNodePutPayload({ id: rec.id, node: rec.node });
    this.pending.push({ op: WAL_OP_PUT_NODE, payload });
    return { type: typeName, id: rec.id };
  }

  getNode(typeName: string, id: string): Node | null {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(typeName, id);
    const k = nodeKey({ type: typeName, id });
    const rec = this.state.nodes.get(k);
    if (!rec) return null;
    return nodeFromRecord(rec);
  }

  listNodesByTag(tag: string): Node[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    validateTag(tag);
    const matches: NodeRecord[] = [];
    for (const rec of this.state.nodes.values()) {
      if (rec.node.tags.includes(tag)) matches.push(rec);
    }
    return sortAndMapNodes(matches);
  }

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
      strength: fields.strength ?? 0,
      confidence: fields.confidence !== undefined ? fields.confidence : null,
      meta: fields.meta ? { ...fields.meta } : {},
      createdAt: 0n,
      updatedAt: 0n,
      version: 1,
    };

    const rec = putEdgeInState(this.state, e, nowMicros());
    const payload = encodeEdgePutPayload(rec);
    this.pending.push({ op: WAL_OP_PUT_EDGE, payload });
  }

  outboundEdges(nodeRef: NodeRef, relation?: string): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(nodeRef.type, nodeRef.id);
    if (relation) validateComponent(relation);
    const matches: CoreEdge[] = [];
    for (const e of this.state.edges.values()) {
      if (e.fromType !== nodeRef.type || e.fromNode !== nodeRef.id) continue;
      if (relation && e.relation !== relation) continue;
      matches.push(e);
    }
    return sortAndMapEdgesByKey(matches);
  }

  inboundEdges(nodeRef: NodeRef, relation?: string): Edge[] {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(nodeRef.type, nodeRef.id);
    if (relation) validateComponent(relation);
    const matches: CoreEdge[] = [];
    for (const e of this.state.edges.values()) {
      if (e.toType !== nodeRef.type || e.toNode !== nodeRef.id) continue;
      if (relation && e.relation !== relation) continue;
      matches.push(e);
    }
    return sortAndMapEdgesByIndexKey(matches);
  }

  deleteNode(typeName: string, id: string): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(typeName, id);
    const k = nodeKey({ type: typeName, id });
    if (!this.state.nodes.has(k)) throw new NotFoundError(`node ${typeName}/${id} not found`);
    for (const e of this.state.edges.values()) {
      if ((e.fromType === typeName && e.fromNode === id) || (e.toType === typeName && e.toNode === id)) {
        throw new InvalidInputError('node has live edges; delete edges first');
      }
    }
    this.state.nodes.delete(k);
    const payload = encodeNodeDeletePayload({ type: typeName, id });
    this.pending.push({ op: WAL_OP_DELETE_NODE, payload });
  }

  deleteEdge(fromRef: NodeRef, relation: string, toRef: NodeRef): void {
    if (this.closed) throw new InvalidInputError('store is closed');
    buildNodeKey(fromRef.type, fromRef.id);
    buildNodeKey(toRef.type, toRef.id);
    validateComponent(relation);
    const k = edgeKey({ fromType: fromRef.type, from: fromRef.id, relation, toType: toRef.type, to: toRef.id });
    if (!this.state.edges.has(k)) throw new NotFoundError(`edge not found`);
    this.state.edges.delete(k);
    const payload = encodeEdgeDeletePayload({ fromType: fromRef.type, fromNode: fromRef.id, relation, toType: toRef.type, toNode: toRef.id });
    this.pending.push({ op: WAL_OP_DELETE_EDGE, payload });
  }

  // ---- internal inspection (for testing/conformance) ---------------------

  get hasUncompactedWAL(): boolean {
    return this.committedWAL.length > 0;
  }

  get nextWALSequence(): bigint {
    return this.nextWALSeq;
  }

  // ---- lifecycle (async, I/O) ---------------------------------------------

  async commit(): Promise<void> {
    if (this.closed) throw new InvalidInputError('store is closed');
    if (this.pending.length === 0) return;
    const records: WALRecord[] = [];
    let next = this.nextWALSeq;
    for (const p of this.pending) {
      records.push({ sequence: next, operation: p.op, payload: p.payload });
      next++;
    }
    records.push({ sequence: next, operation: WAL_OP_COMMIT, payload: new Uint8Array(0) });
    this.committedWAL = [...this.committedWAL, ...records];
    this.pending = [];
    this.nextWALSeq = next + 1n;
    await this.writeFile();
  }

  async close(): Promise<void> {
    if (this.closed) return;
    await this.commit();
    this.closed = true;
  }

  // ---- internal -----------------------------------------------------------

  private async writeFile(): Promise<void> {
    const entries = materializeDataEntries(this.state);
    const data = encodeDataEntries(entries);
    const keys = entries.map(e => e.key);
    const bloom = encodeBloom(keys);
    const walPayload = encodeWALRecords(this.committedWAL);
    const file = encodeContainer({ data, bloom, wal: walPayload });
    writeFileAtomic(this.path, file);
  }
}

// ---- Open factory ----------------------------------------------------------

export async function open(path: string): Promise<Store> {
  return Store.open(path);
}

// ---- Hydration and materialization -----------------------------------------

function hydrateDataEntries(entries: DataEntry[]): StoreState {
  const enc = new TextEncoder();
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
      state.nodes.set(nodeKey({ type: parsed.type, id: parsed.id }), { id: parsed.id, node });
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
      state.edges.set(edgeKey({ fromType: edge.fromType, from: edge.fromNode, relation: edge.relation, toType: edge.toType, to: edge.toNode }), edge);
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
  validateDerivedKeys(state, entries);
  return state;
}

function wrapDataPayloadError(e: unknown): Error {
  if (e instanceof MissingRequiredFieldError) return new MissingRequiredFieldError(`invalid data payload: ${e.message}`);
  return new InvalidInputError(`invalid data payload: ${e instanceof Error ? e.message : String(e)}`);
}

function validateDerivedKeys(state: StoreState, entries: DataEntry[]): void {
  const dec = new TextDecoder('utf-8');
  const expected = materializeDataEntries(state);
  if (expected.length !== entries.length) throw new InvalidInputError('derived index mismatch');
  for (let i = 0; i < expected.length; i++) {
    if (!equalBytes(expected[i].key, entries[i].key)) throw new InvalidInputError('derived index mismatch');
  }
}

export function materializeDataEntries(state: StoreState): DataEntry[] {
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
      add(buildTagKey(tag, rec.id), new Uint8Array(0));
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

function inspectAndReplayWAL(state: StoreState, walPayload: Uint8Array | null): [WALRecord[], bigint] {
  if (!walPayload || walPayload.length === 0) return [[], 1n];

  let next = 1n;
  const all: WALRecord[] = [];
  let lastCommit = -1;
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
    if (r.sequence >= next) next = r.sequence + 1n;
    if (r.operation === WAL_OP_COMMIT) lastCommit = all.length - 1;
    pos += n;
  }

  if (lastCommit < 0) return [[], next];

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
        state.nodes.set(nodeKey({ type: put.node.type, id: put.id }), { id: put.id, node: put.node });
        break;
      }
      case WAL_OP_DELETE_NODE: {
        const d = decodeNodeDeletePayload(r.payload);
        state.nodes.delete(nodeKey({ type: d.type, id: d.id }));
        break;
      }
      case WAL_OP_PUT_EDGE: {
        const e = decodeEdgePutPayload(r.payload);
        state.edges.set(edgeKey({ fromType: e.fromType, from: e.fromNode, relation: e.relation, toType: e.toType, to: e.toNode }), e);
        break;
      }
      case WAL_OP_DELETE_EDGE: {
        const d = decodeEdgeDeletePayload(r.payload);
        state.edges.delete(edgeKey({ fromType: d.fromType, from: d.fromNode, relation: d.relation, toType: d.toType, to: d.toNode }));
        break;
      }
    }
  }

  return [committed, next];
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

// ---- File I/O --------------------------------------------------------------

function writeFileAtomic(path: string, data: Uint8Array): void {
  const fd = openSync(path, 'w');
  try {
    writeSync(fd, data);
    fsyncSync(fd);
  } finally {
    closeSync(fd);
  }
}
