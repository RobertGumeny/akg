import { InvalidInputError } from '../errors.js';

export const MAX_NODE_ID_LEN = 64;

export function validateComponent(value: string): void {
  if (!value) throw new InvalidInputError('empty component');
  let prevUnderscore = false;
  for (let i = 0; i < value.length; i++) {
    const c = value[i];
    if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
      prevUnderscore = false;
    } else if (c === '_') {
      if (i === 0 || prevUnderscore) throw new InvalidInputError(`invalid component: ${value}`);
      prevUnderscore = true;
    } else {
      throw new InvalidInputError(`invalid component: ${value}`);
    }
  }
  if (prevUnderscore) throw new InvalidInputError(`invalid component: ${value}`);
}

export function validateTag(tag: string): void {
  validateComponent(tag);
}

export function validateNodeID(id: string): void {
  if (!id) throw new InvalidInputError('empty node ID');
  if (id.includes(':')) throw new InvalidInputError('node ID must not contain colons');
  const encoder = new TextEncoder();
  const bytes = encoder.encode(id);
  const decoder = new TextDecoder('utf-8', { fatal: true });
  try {
    decoder.decode(bytes);
  } catch {
    throw new InvalidInputError('node ID must be valid UTF-8');
  }
  if ([...id].length > MAX_NODE_ID_LEN) {
    throw new InvalidInputError(`node ID exceeds ${MAX_NODE_ID_LEN} characters`);
  }
}

export function buildNodeKey(type: string, id: string): string {
  validateComponent(type);
  validateNodeID(id);
  return `n:${type}:${id}`;
}

export function buildEdgeKey(fromType: string, fromID: string, relation: string, toType: string, toID: string): string {
  validateComponent(fromType);
  validateNodeID(fromID);
  validateComponent(relation);
  validateComponent(toType);
  validateNodeID(toID);
  return `e:${fromType}:${fromID}:${relation}:${toType}:${toID}`;
}

export function buildEdgeIndexKey(toType: string, toID: string, relation: string, fromType: string, fromID: string): string {
  validateComponent(toType);
  validateNodeID(toID);
  validateComponent(relation);
  validateComponent(fromType);
  validateNodeID(fromID);
  return `ei:${toType}:${toID}:${relation}:${fromType}:${fromID}`;
}

export function buildTagKey(tag: string, id: string): string {
  validateTag(tag);
  validateNodeID(id);
  return `t:${tag}:${id}`;
}

export function buildTemporalNodeKey(ts: bigint, type: string, id: string): string {
  const nodeKey = buildNodeKey(type, id);
  return `ts:${ts}:${nodeKey}`;
}

export function buildTemporalEdgeKey(ts: bigint, fromType: string, fromID: string, relation: string, toType: string, toID: string): string {
  const edgeKey = buildEdgeKey(fromType, fromID, relation, toType, toID);
  return `ts:${ts}:${edgeKey}`;
}

export interface ParsedNodeKey {
  type: string;
  id: string;
}

export interface ParsedEdgeKey {
  fromType: string;
  fromID: string;
  relation: string;
  toType: string;
  toID: string;
}

export interface ParsedEdgeIndexKey {
  toType: string;
  toID: string;
  relation: string;
  fromType: string;
  fromID: string;
}

function splitKey(key: string, want: number): string[] | null {
  const parts = key.split(':');
  if (parts.length !== want) return null;
  for (const p of parts) if (!p) return null;
  return parts;
}

export function parseNodeKey(key: string): ParsedNodeKey {
  const parts = splitKey(key, 3);
  if (!parts || parts[0] !== 'n') throw new InvalidInputError(`malformed node key: ${key}`);
  try {
    validateComponent(parts[1]);
    validateNodeID(parts[2]);
  } catch {
    throw new InvalidInputError(`malformed node key: ${key}`);
  }
  return { type: parts[1], id: parts[2] };
}

export function parseEdgeKey(key: string): ParsedEdgeKey {
  const parts = splitKey(key, 6);
  if (!parts || parts[0] !== 'e') throw new InvalidInputError(`malformed edge key: ${key}`);
  try {
    validateComponent(parts[1]);
    validateNodeID(parts[2]);
    validateComponent(parts[3]);
    validateComponent(parts[4]);
    validateNodeID(parts[5]);
  } catch {
    throw new InvalidInputError(`malformed edge key: ${key}`);
  }
  return { fromType: parts[1], fromID: parts[2], relation: parts[3], toType: parts[4], toID: parts[5] };
}

export function parseEdgeIndexKey(key: string): ParsedEdgeIndexKey {
  const parts = splitKey(key, 6);
  if (!parts || parts[0] !== 'ei') throw new InvalidInputError(`malformed edge index key: ${key}`);
  try {
    validateComponent(parts[1]);
    validateNodeID(parts[2]);
    validateComponent(parts[3]);
    validateComponent(parts[4]);
    validateNodeID(parts[5]);
  } catch {
    throw new InvalidInputError(`malformed edge index key: ${key}`);
  }
  return { toType: parts[1], toID: parts[2], relation: parts[3], fromType: parts[4], fromID: parts[5] };
}

export function parseTagKey(key: string): [string, string] {
  const parts = splitKey(key, 3);
  if (!parts || parts[0] !== 't') throw new InvalidInputError(`malformed tag key: ${key}`);
  try {
    validateTag(parts[1]);
    validateNodeID(parts[2]);
  } catch {
    throw new InvalidInputError(`malformed tag key: ${key}`);
  }
  return [parts[1], parts[2]];
}

export function parseTemporalKey(key: string): void {
  const parts = key.split(':');
  if (parts.length < 4 || parts[0] !== 'ts') throw new InvalidInputError(`malformed temporal key: ${key}`);
  const tsStr = parts[1];
  if (!tsStr || (tsStr.length > 1 && tsStr[0] === '0') || !/^\d+$/.test(tsStr)) {
    throw new InvalidInputError(`malformed temporal key: ${key}`);
  }
  const suffix = parts.slice(2).join(':');
  if (parts[2] === 'n') {
    parseNodeKey(suffix);
  } else if (parts[2] === 'e') {
    parseEdgeKey(suffix);
  } else {
    throw new InvalidInputError(`malformed temporal key: ${key}`);
  }
}
