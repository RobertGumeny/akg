import { describe, it, expect } from 'vitest';
import {
  validateComponent, validateNodeID, buildNodeKey, buildEdgeKey, buildEdgeIndexKey, buildTagKey,
  parseNodeKey, parseEdgeKey, parseEdgeIndexKey,
} from '../src/internal/keys.js';
import { InvalidInputError } from '../src/errors.js';

describe('validateComponent', () => {
  it('accepts valid components', () => {
    expect(() => validateComponent('person')).not.toThrow();
    expect(() => validateComponent('node_type')).not.toThrow();
    expect(() => validateComponent('abc123')).not.toThrow();
  });

  it('rejects empty', () => {
    expect(() => validateComponent('')).toThrow(InvalidInputError);
  });

  it('rejects uppercase', () => {
    expect(() => validateComponent('BadType')).toThrow(InvalidInputError);
    expect(() => validateComponent('Person')).toThrow(InvalidInputError);
  });

  it('rejects leading underscore', () => {
    expect(() => validateComponent('_type')).toThrow(InvalidInputError);
  });

  it('rejects trailing underscore', () => {
    expect(() => validateComponent('type_')).toThrow(InvalidInputError);
  });

  it('rejects consecutive underscores', () => {
    expect(() => validateComponent('type__name')).toThrow(InvalidInputError);
  });

  it('rejects colons', () => {
    expect(() => validateComponent('bad:type')).toThrow(InvalidInputError);
  });

  it('rejects spaces', () => {
    expect(() => validateComponent('bad type')).toThrow(InvalidInputError);
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

  it('rejects IDs too long', () => {
    expect(() => validateNodeID('a'.repeat(65))).toThrow(InvalidInputError);
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

  it('builds tag key', () => {
    const key = buildTagKey('active', 'alice');
    expect(key).toBe('t:active:alice');
  });
});
