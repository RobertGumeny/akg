import { describe, it, expect } from 'vitest';
import {
  validateComponent, validateNodeID, buildNodeKey, buildEdgeKey, buildEdgeIndexKey, buildTagKey,
  parseNodeKey, parseEdgeKey, parseEdgeIndexKey, parseTagKey,
} from '../src/internal/keys.js';
import { InvalidInputError } from '../src/errors.js';

describe('validateComponent', () => {
  it('accepts valid components', () => {
    expect(() => validateComponent('person')).not.toThrow();
    expect(() => validateComponent('node_type')).not.toThrow();
    expect(() => validateComponent('abc123')).not.toThrow();
  });

  // CONF-1: type/relation/tag are any key-safe UTF-8 string — casing and
  // word-separation are an SDK convention, not a format rule (spec 04:80).
  it('accepts uppercase and non-ascii (no snake_case rule)', () => {
    expect(() => validateComponent('Person')).not.toThrow();
    expect(() => validateComponent('BadType')).not.toThrow();
    expect(() => validateComponent('café')).not.toThrow();
    expect(() => validateComponent('_type')).not.toThrow();
    expect(() => validateComponent('type_')).not.toThrow();
    expect(() => validateComponent('type__name')).not.toThrow();
    expect(() => validateComponent('in progress')).not.toThrow();
  });

  it('rejects empty', () => {
    expect(() => validateComponent('')).toThrow(InvalidInputError);
  });

  it('rejects colons', () => {
    expect(() => validateComponent('bad:type')).toThrow(InvalidInputError);
  });

  // CONF-2: the cap is 64 UTF-8 bytes, not codepoints.
  it('accepts exactly 64 bytes, rejects 65', () => {
    expect(() => validateComponent('a'.repeat(64))).not.toThrow();
    expect(() => validateComponent('a'.repeat(65))).toThrow(InvalidInputError);
  });

  it('accepts 32 multibyte chars (64 bytes), rejects 33 (66 bytes)', () => {
    expect(() => validateComponent('é'.repeat(32))).not.toThrow();
    expect(() => validateComponent('é'.repeat(33))).toThrow(InvalidInputError);
  });
});

describe('validateNodeID', () => {
  it('accepts valid IDs', () => {
    expect(() => validateNodeID('alice')).not.toThrow();
    expect(() => validateNodeID('a'.repeat(64))).not.toThrow();
  });

  it('rejects empty', () => {
    expect(() => validateNodeID('')).toThrow(InvalidInputError);
  });

  it('rejects colons', () => {
    expect(() => validateNodeID('bad:id')).toThrow(InvalidInputError);
  });

  it('rejects IDs over 64 bytes', () => {
    expect(() => validateNodeID('a'.repeat(65))).toThrow(InvalidInputError);
    // 33 two-byte chars = 66 bytes > 64, even though only 33 codepoints.
    expect(() => validateNodeID('é'.repeat(33))).toThrow(InvalidInputError);
  });
});

describe('key builders and parsers', () => {
  it('round-trips node key', () => {
    const key = buildNodeKey('person', 'alice');
    expect(key).toBe('n:person:alice');
    const parsed = parseNodeKey(key);
    expect(parsed.type).toBe('person');
    expect(parsed.id).toBe('alice');
  });

  it('round-trips edge key', () => {
    const key = buildEdgeKey('person', 'alice', 'knows', 'person', 'bob');
    expect(key).toBe('e:person:alice:knows:person:bob');
    const parsed = parseEdgeKey(key);
    expect(parsed.fromType).toBe('person');
    expect(parsed.fromID).toBe('alice');
    expect(parsed.relation).toBe('knows');
    expect(parsed.toType).toBe('person');
    expect(parsed.toID).toBe('bob');
  });

  it('round-trips edge index key', () => {
    const key = buildEdgeIndexKey('person', 'bob', 'knows', 'person', 'alice');
    expect(key).toBe('ei:person:bob:knows:person:alice');
    const parsed = parseEdgeIndexKey(key);
    expect(parsed.toType).toBe('person');
    expect(parsed.toID).toBe('bob');
    expect(parsed.relation).toBe('knows');
    expect(parsed.fromType).toBe('person');
    expect(parsed.fromID).toBe('alice');
  });

  it('builds type-qualified (major-2) tag key', () => {
    const key = buildTagKey('active', 'user', 'alice');
    expect(key).toBe('t:active:user:alice');
  });

  it('parses a major-2 (4-part) tag key', () => {
    expect(parseTagKey('t:active:user:alice')).toEqual(['active', 'user', 'alice']);
  });

  it('parses a major-1 (3-part) tag key with empty type (read-compat)', () => {
    expect(parseTagKey('t:active:alice')).toEqual(['active', '', 'alice']);
  });
});
