import { InvalidInputError } from '../errors.js';

// MAX_COMPONENT_BYTES caps every key component — node-id, type, relation, and
// tag — at 64 UTF-8 bytes (spec 04:31/34/54/77, echoed 01:18/62/116). Bytes, not
// codepoints: unambiguous and identical across language implementations.
export const MAX_COMPONENT_BYTES = 64;

const utf8Encoder = new TextEncoder();

// validateComponent enforces only the format-level key-safety rules that apply
// to every component (type, relation, tag, node-id): non-empty, valid UTF-8, no
// colon delimiter, and at most 64 UTF-8 bytes (spec 01:18/62/116, 04:31/34/54/77).
// Casing and word-separation (lowercase, snake_case) are an SDK-level convention,
// not a format rule (04:80) — writers must not reject or silently correct them.
export function validateComponent(value: string): void {
  if (!value) throw new InvalidInputError('empty component');
  if (value.includes(':')) throw new InvalidInputError(`invalid component: ${value}`);
  // A JS string holding a lone surrogate is not valid UTF-8; encode→decode(fatal)
  // surfaces that before we measure byte length.
  const bytes = utf8Encoder.encode(value);
  try {
    new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    throw new InvalidInputError(`invalid component: ${value}`);
  }
  if (bytes.length > MAX_COMPONENT_BYTES) {
    throw new InvalidInputError(`component exceeds ${MAX_COMPONENT_BYTES} bytes: ${value}`);
  }
}

export function validateTag(tag: string): void {
  validateComponent(tag);
}

export function validateNodeID(id: string): void {
  validateComponent(id);
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
