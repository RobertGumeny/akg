import { InvalidInputError, MissingRequiredFieldError } from '../errors.js';
import { encodeMsgpack, decodeMsgpackFull, type MsgpackValue } from './msgpack.js';

export interface CoreNode {
  type: string;
  title: string;
  body: string;
  meta: Record<string, unknown>;
  tags: string[];
  createdAt: bigint;
  updatedAt: bigint;
  version: number;
}

export interface CoreEdge {
  fromType: string;
  fromNode: string;
  toType: string;
  toNode: string;
  relation: string;
  strength: number;
  confidence: number | null;
  meta: Record<string, unknown>;
  createdAt: bigint;
  updatedAt: bigint;
  version: number;
}

export interface NodePut {
  id: string;
  node: CoreNode;
}

export interface NodeDelete {
  type: string;
  id: string;
}

export interface EdgeDelete {
  fromType: string;
  fromNode: string;
  relation: string;
  toType: string;
  toNode: string;
}

function asUint(v: MsgpackValue): bigint | null {
  if (typeof v === 'number' && Number.isInteger(v) && v >= 0) return BigInt(v);
  if (typeof v === 'bigint' && v >= 0n) return v;
  return null;
}

function toMsgpackMeta(meta: Record<string, unknown>): MsgpackValue {
  const m: Record<string, MsgpackValue> = {};
  for (const [k, v] of Object.entries(meta)) {
    m[k] = toMsgpackValue(v);
  }
  return m;
}

function toMsgpackValue(v: unknown): MsgpackValue {
  if (v === null || v === undefined) return null;
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v;
  if (typeof v === 'number') return v;
  if (typeof v === 'bigint') return v;
  if (Array.isArray(v)) return v.map(toMsgpackValue);
  if (typeof v === 'object') {
    const m: Record<string, MsgpackValue> = {};
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) {
      m[k] = toMsgpackValue(val);
    }
    return m;
  }
  throw new InvalidInputError('unsupported meta value type');
}

function fromMsgpackMeta(v: MsgpackValue): Record<string, unknown> {
  if (v === null || v === undefined) return {};
  if (typeof v !== 'object' || Array.isArray(v)) throw new InvalidInputError('invalid meta');
  return v as Record<string, unknown>;
}

export function encodeNodePayload(n: CoreNode): Uint8Array {
  if (!n.type || !n.title) throw new MissingRequiredFieldError();
  const m: Record<string, MsgpackValue> = {
    type: n.type,
    title: n.title,
    created_at: n.createdAt,
    updated_at: n.updatedAt,
  };
  if (n.body) m.body = n.body;
  if (Object.keys(n.meta).length > 0) m.meta = toMsgpackMeta(n.meta);
  if (n.tags.length > 0) m.tags = n.tags;
  if (n.version !== 0 && n.version !== 1) m.version = BigInt(n.version);
  return encodeMsgpack(m);
}

export function encodeNodePutPayload(p: NodePut): Uint8Array {
  if (!p.id) throw new MissingRequiredFieldError();
  if (!p.node.type || !p.node.title) throw new MissingRequiredFieldError();
  const m: Record<string, MsgpackValue> = {
    id: p.id,
    type: p.node.type,
    title: p.node.title,
    created_at: p.node.createdAt,
    updated_at: p.node.updatedAt,
  };
  if (p.node.body) m.body = p.node.body;
  if (Object.keys(p.node.meta).length > 0) m.meta = toMsgpackMeta(p.node.meta);
  if (p.node.tags.length > 0) m.tags = p.node.tags;
  if (p.node.version !== 0 && p.node.version !== 1) m.version = BigInt(p.node.version);
  return encodeMsgpack(m);
}

export function decodeNodePayload(b: Uint8Array): CoreNode {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid data payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid data payload');
  const m = v as Record<string, MsgpackValue>;
  const type = m.type;
  if (typeof type !== 'string' || !type) throw new MissingRequiredFieldError('node type missing');
  const title = m.title;
  if (typeof title !== 'string' || !title) throw new MissingRequiredFieldError('node title missing');
  const node: CoreNode = {
    type,
    title,
    body: typeof m.body === 'string' ? m.body : '',
    meta: m.meta ? fromMsgpackMeta(m.meta) : {},
    tags: [],
    createdAt: 0n,
    updatedAt: 0n,
    version: 1,
  };
  const ts = asUint(m.created_at);
  if (ts !== null) node.createdAt = ts;
  const tu = asUint(m.updated_at);
  if (tu !== null) node.updatedAt = tu;
  const ver = asUint(m.version);
  if (ver !== null) node.version = Number(ver);
  if (node.version === 0) node.version = 1;
  if (Array.isArray(m.tags)) {
    for (const t of m.tags) {
      if (typeof t !== 'string') throw new InvalidInputError('invalid data payload: tag not a string');
      node.tags.push(t);
    }
  }
  return node;
}

export function decodeNodePutPayload(b: Uint8Array): NodePut {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid wal payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid wal payload');
  const m = v as Record<string, MsgpackValue>;
  const id = m.id;
  if (typeof id !== 'string' || !id) throw new MissingRequiredFieldError('node id missing');
  const node = decodeNodeFromMap(m);
  return { id, node };
}

function decodeNodeFromMap(m: Record<string, MsgpackValue>): CoreNode {
  const type = m.type;
  if (typeof type !== 'string' || !type) throw new MissingRequiredFieldError('node type missing');
  const title = m.title;
  if (typeof title !== 'string' || !title) throw new MissingRequiredFieldError('node title missing');
  const node: CoreNode = {
    type,
    title,
    body: typeof m.body === 'string' ? m.body : '',
    meta: m.meta ? fromMsgpackMeta(m.meta) : {},
    tags: [],
    createdAt: 0n,
    updatedAt: 0n,
    version: 1,
  };
  const ts = asUint(m.created_at);
  if (ts !== null) node.createdAt = ts;
  const tu = asUint(m.updated_at);
  if (tu !== null) node.updatedAt = tu;
  const ver = asUint(m.version);
  if (ver !== null) node.version = Number(ver);
  if (node.version === 0) node.version = 1;
  if (Array.isArray(m.tags)) {
    for (const t of m.tags) {
      if (typeof t !== 'string') throw new InvalidInputError('invalid payload: tag not a string');
      node.tags.push(t);
    }
  }
  return node;
}

export function encodeNodeDeletePayload(d: NodeDelete): Uint8Array {
  if (!d.type || !d.id) throw new MissingRequiredFieldError();
  return encodeMsgpack({ type: d.type, id: d.id });
}

export function decodeNodeDeletePayload(b: Uint8Array): NodeDelete {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid wal payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid wal payload');
  const m = v as Record<string, MsgpackValue>;
  const type = m.type;
  if (typeof type !== 'string' || !type) throw new MissingRequiredFieldError('node type missing');
  const id = m.id;
  if (typeof id !== 'string' || !id) throw new MissingRequiredFieldError('node id missing');
  return { type, id };
}

export function encodeEdgePayload(e: CoreEdge): Uint8Array {
  if (!e.fromType || !e.fromNode || !e.toType || !e.toNode || !e.relation) throw new MissingRequiredFieldError();
  const m: Record<string, MsgpackValue> = {
    from_node_type: e.fromType,
    from_node: e.fromNode,
    to_node_type: e.toType,
    to_node: e.toNode,
    relation: e.relation,
    strength: e.strength,
    created_at: e.createdAt,
    updated_at: e.updatedAt,
  };
  if (e.confidence !== null) m.confidence = e.confidence;
  if (Object.keys(e.meta).length > 0) m.meta = toMsgpackMeta(e.meta);
  if (e.version !== 0 && e.version !== 1) m.version = BigInt(e.version);
  return encodeMsgpack(m);
}

export function decodeEdgePayload(b: Uint8Array): CoreEdge {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid data payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid data payload');
  const m = v as Record<string, MsgpackValue>;
  return decodeEdgeFromMap(m);
}

export function decodeEdgePutPayload(b: Uint8Array): CoreEdge {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid wal payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid wal payload');
  const m = v as Record<string, MsgpackValue>;
  return decodeEdgeFromMap(m);
}

function decodeEdgeFromMap(m: Record<string, MsgpackValue>): CoreEdge {
  const fromType = m.from_node_type;
  if (typeof fromType !== 'string' || !fromType) throw new MissingRequiredFieldError('from_node_type missing');
  const fromNode = m.from_node;
  if (typeof fromNode !== 'string' || !fromNode) throw new MissingRequiredFieldError('from_node missing');
  const toType = m.to_node_type;
  if (typeof toType !== 'string' || !toType) throw new MissingRequiredFieldError('to_node_type missing');
  const toNode = m.to_node;
  if (typeof toNode !== 'string' || !toNode) throw new MissingRequiredFieldError('to_node missing');
  const relation = m.relation;
  if (typeof relation !== 'string' || !relation) throw new MissingRequiredFieldError('relation missing');

  const edge: CoreEdge = {
    fromType,
    fromNode,
    toType,
    toNode,
    relation,
    strength: 0.5,
    confidence: null,
    meta: {},
    createdAt: 0n,
    updatedAt: 0n,
    version: 1,
  };
  if (typeof m.strength === 'number') edge.strength = m.strength;
  if ('confidence' in m) {
    if (m.confidence === null) {
      edge.confidence = null;
    } else if (typeof m.confidence === 'number') {
      edge.confidence = m.confidence;
    } else {
      throw new InvalidInputError('invalid payload: confidence not a number');
    }
  }
  const ts = asUint(m.created_at);
  if (ts !== null) edge.createdAt = ts;
  const tu = asUint(m.updated_at);
  if (tu !== null) edge.updatedAt = tu;
  const ver = asUint(m.version);
  if (ver !== null) edge.version = Number(ver);
  if (edge.version === 0) edge.version = 1;
  if (m.meta) edge.meta = fromMsgpackMeta(m.meta);
  return edge;
}

export function encodeEdgeDeletePayload(d: EdgeDelete): Uint8Array {
  if (!d.fromType || !d.fromNode || !d.relation || !d.toType || !d.toNode) throw new MissingRequiredFieldError();
  return encodeMsgpack({
    from_node_type: d.fromType,
    from_node: d.fromNode,
    relation: d.relation,
    to_node_type: d.toType,
    to_node: d.toNode,
  });
}

export function decodeEdgeDeletePayload(b: Uint8Array): EdgeDelete {
  let v: MsgpackValue;
  try {
    v = decodeMsgpackFull(b);
  } catch {
    throw new InvalidInputError('invalid wal payload');
  }
  if (typeof v !== 'object' || Array.isArray(v) || v === null) throw new InvalidInputError('invalid wal payload');
  const m = v as Record<string, MsgpackValue>;
  const fromType = m.from_node_type;
  if (typeof fromType !== 'string' || !fromType) throw new MissingRequiredFieldError('from_node_type missing');
  const fromNode = m.from_node;
  if (typeof fromNode !== 'string' || !fromNode) throw new MissingRequiredFieldError('from_node missing');
  const relation = m.relation;
  if (typeof relation !== 'string' || !relation) throw new MissingRequiredFieldError('relation missing');
  const toType = m.to_node_type;
  if (typeof toType !== 'string' || !toType) throw new MissingRequiredFieldError('to_node_type missing');
  const toNode = m.to_node;
  if (typeof toNode !== 'string' || !toNode) throw new MissingRequiredFieldError('to_node missing');
  return { fromType, fromNode, relation, toType, toNode };
}

export function validateWALPayload(op: number, payload: Uint8Array): void {
  switch (op) {
    case 0x01: decodeNodePutPayload(payload); break;
    case 0x02: decodeNodeDeletePayload(payload); break;
    case 0x03: decodeEdgePutPayload(payload); break;
    case 0x04: decodeEdgeDeletePayload(payload); break;
    case 0x05: if (payload.length !== 0) throw new InvalidInputError('invalid wal record'); break;
    default: throw new InvalidInputError('unknown wal operation');
  }
}
