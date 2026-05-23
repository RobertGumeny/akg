import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { open, Store } from '../src/store.js';
import { NotFoundError, InvalidInputError, MissingRequiredFieldError } from '../src/errors.js';

let dir: string;
let storePath: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), 'akg-ts-test-'));
  storePath = join(dir, 'test.akg');
});

afterEach(() => {
  rmSync(dir, { recursive: true });
});

describe('empty-store round-trip', () => {
  it('reopens an immediately-closed store with zero nodes and zero edges', async () => {
    const s = await open(storePath);
    await s.close();

    const s2 = await open(storePath);
    expect(s2.listNodes().length).toBe(0);
    expect(s2.outboundEdges({ type: 'note', id: 'ghost' }).length).toBe(0);
    await s2.close();
  });
});

describe('store lifecycle', () => {
  it('creates a new store and reopens it', async () => {
    const s = await open(storePath);
    expect(existsSync(storePath)).toBe(true);
    await s.close();

    const s2 = await open(storePath);
    expect(s2.listNodes().length).toBe(0);
    await s2.close();
  });

  it('commit is no-op when nothing pending', async () => {
    const s = await open(storePath);
    await s.commit();
    await s.commit();
    await s.close();
  });

  it('close on already-closed store is no-op', async () => {
    const s = await open(storePath);
    await s.close();
    await expect(s.close()).resolves.toBeUndefined();
  });

  it('commit-on-close: mutations survive close without explicit commit', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice' }, []);
    await s.close();

    const s2 = await open(storePath);
    const node = s2.getNode('person', 'alice');
    expect(node).not.toBeNull();
    expect(node!.title).toBe('Alice');
    await s2.close();
  });
});

describe('node operations', () => {
  it('puts and gets a node', async () => {
    const s = await open(storePath);
    const ref = s.putNode('person', 'alice', { title: 'Alice', body: 'A researcher', meta: { role: 'lead' } }, ['active']);
    expect(ref.type).toBe('person');
    expect(ref.id).toBe('alice');

    const node = s.getNode('person', 'alice');
    expect(node).not.toBeNull();
    expect(node!.title).toBe('Alice');
    expect(node!.body).toBe('A researcher');
    expect(node!.tags).toEqual(['active']);
    expect(node!.meta).toEqual({ role: 'lead' });
    await s.close();
  });

  it('returns null for missing node', async () => {
    const s = await open(storePath);
    expect(s.getNode('person', 'nobody')).toBeNull();
    await s.close();
  });

  it('generates ID when empty string passed', async () => {
    const s = await open(storePath);
    const ref = s.putNode('person', '', { title: 'Alice' }, []);
    expect(ref.id).not.toBe('');
    expect(ref.id.length).toBeGreaterThan(0);
    await s.close();
  });

  it('putNode replaces existing node', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice v1' }, []);
    s.putNode('person', 'alice', { title: 'Alice v2' }, []);
    const node = s.getNode('person', 'alice');
    expect(node!.title).toBe('Alice v2');
    expect(node!.version).toBe(2);
    await s.close();
  });

  it('listNodesByTag returns matching nodes', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice' }, ['active', 'researcher']);
    s.putNode('person', 'bob', { title: 'Bob' }, ['active']);
    s.putNode('person', 'eve', { title: 'Eve' }, ['researcher']);

    const active = s.listNodesByTag('active');
    expect(active.length).toBe(2);
    expect(active.map(n => n.id)).toEqual(['alice', 'bob']);

    const researcher = s.listNodesByTag('researcher');
    expect(researcher.length).toBe(2);
    await s.close();
  });

  it('listNodes returns all nodes', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice' }, []);
    s.putNode('paper', 'p1', { title: 'Paper' }, []);

    const all = s.listNodes();
    expect(all.length).toBe(2);

    const people = s.listNodes('person');
    expect(people.length).toBe(1);
    expect(people[0].id).toBe('alice');

    const unknown = s.listNodes('unknown_type');
    expect(unknown.length).toBe(0);
    await s.close();
  });

  it('validates type name', async () => {
    const s = await open(storePath);
    expect(() => s.putNode('BadType', 'id', { title: 'T' }, [])).toThrow(InvalidInputError);
    expect(() => s.putNode('bad:type', 'id', { title: 'T' }, [])).toThrow(InvalidInputError);
    await s.close();
  });

  it('requires title', async () => {
    const s = await open(storePath);
    expect(() => s.putNode('person', 'id', { title: '' }, [])).toThrow(MissingRequiredFieldError);
    await s.close();
  });

  it('nodes survive round-trip through close/reopen', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice', body: 'Body', meta: { k: 'v' } }, ['active']);
    await s.close();

    const s2 = await open(storePath);
    const node = s2.getNode('person', 'alice');
    expect(node).not.toBeNull();
    expect(node!.title).toBe('Alice');
    expect(node!.body).toBe('Body');
    expect(node!.meta).toEqual({ k: 'v' });
    expect(node!.tags).toEqual(['active']);
    await s2.close();
  });
});

describe('edge operations', () => {
  it('puts and reads outbound edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { strength: 0.9 });

    const edges = s.outboundEdges(alice);
    expect(edges.length).toBe(1);
    expect(edges[0].to.id).toBe('bob');
    expect(edges[0].relation).toBe('knows');
    expect(edges[0].strength).toBe(0.9);
    await s.close();
  });

  it('reads inbound edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});

    const inbound = s.inboundEdges(bob);
    expect(inbound.length).toBe(1);
    expect(inbound[0].from.id).toBe('alice');
    await s.close();
  });

  it('filters by relation', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const paper = s.putNode('paper', 'p1', { title: 'Paper' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.putEdge(alice, 'authored', paper, {});

    const knows = s.outboundEdges(alice, 'knows');
    expect(knows.length).toBe(1);
    expect(knows[0].to.id).toBe('bob');
    await s.close();
  });

  it('requires both nodes to exist', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    expect(() => s.putEdge(alice, 'knows', { type: 'person', id: 'ghost' }, {})).toThrow(NotFoundError);
    await s.close();
  });

  it('edges survive round-trip', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { strength: 0.7 });
    await s.close();

    const s2 = await open(storePath);
    const edges = s2.outboundEdges({ type: 'person', id: 'alice' });
    expect(edges.length).toBe(1);
    expect(edges[0].strength).toBe(0.7);
    await s2.close();
  });

  it('edge strength defaults to 0.0', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    const edges = s.outboundEdges(alice);
    expect(edges[0].strength).toBe(0);
    await s.close();
  });
});

describe('delete operations', () => {
  it('deletes a node', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice' }, []);
    s.deleteNode('person', 'alice');
    expect(s.getNode('person', 'alice')).toBeNull();
    await s.close();
  });

  it('deleteNode throws NotFoundError for missing node', async () => {
    const s = await open(storePath);
    expect(() => s.deleteNode('person', 'nobody')).toThrow(NotFoundError);
    await s.close();
  });

  it('deleteNode throws InvalidInputError when node has edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    expect(() => s.deleteNode('person', 'alice')).toThrow(InvalidInputError);
    await s.close();
  });

  it('deletes an edge', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.deleteEdge(alice, 'knows', bob);
    expect(s.outboundEdges(alice).length).toBe(0);
    await s.close();
  });

  it('deleteEdge throws NotFoundError for missing edge', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    expect(() => s.deleteEdge(alice, 'knows', bob)).toThrow(NotFoundError);
    await s.close();
  });

  it('node deletion survives reopen', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'Note' }, []);
    await s.commit();
    s.deleteNode('note', 'n1');
    await s.close();

    const s2 = await open(storePath);
    expect(s2.getNode('note', 'n1')).toBeNull();
    await s2.close();
  });

  it('edge deletion survives reopen', async () => {
    const s = await open(storePath);
    const a = s.putNode('person', 'a', { title: 'A' }, []);
    const b = s.putNode('person', 'b', { title: 'B' }, []);
    s.putEdge(a, 'knows', b, {});
    await s.commit();
    s.deleteEdge(a, 'knows', b);
    await s.close();

    const s2 = await open(storePath);
    expect(s2.outboundEdges({ type: 'person', id: 'a' }).length).toBe(0);
    expect(s2.listNodes().length).toBe(2);
    await s2.close();
  });
});

describe('error handling', () => {
  it('throws InvalidInputError on invalid component names', async () => {
    const s = await open(storePath);
    expect(() => s.putNode('Bad', 'id', { title: 'T' }, [])).toThrow(InvalidInputError);
    expect(() => s.listNodes('BadType')).toThrow(InvalidInputError);
    await s.close();
  });

  it('error classes use instanceof correctly', async () => {
    const s = await open(storePath);
    let caught: Error | undefined;
    try {
      s.getNode('bad:type', 'id');
    } catch (e) {
      caught = e as Error;
    }
    expect(caught).toBeInstanceOf(InvalidInputError);
    await s.close();
  });
});

describe('cross-type contamination', () => {
  it('outboundEdges returns no results for a different type sharing the same ID', async () => {
    const s = await open(storePath);
    const noteShared = s.putNode('note', 'shared', { title: 'Note' }, []);
    const conceptShared = s.putNode('concept', 'shared', { title: 'Concept' }, []);
    const target = s.putNode('note', 'target', { title: 'Target' }, []);
    s.putEdge(noteShared, 'links_to', target, {});

    // concept/shared must see zero outbound edges — different type, same ID string
    expect(s.outboundEdges(conceptShared).length).toBe(0);
    // note/shared must see the one edge
    expect(s.outboundEdges(noteShared).length).toBe(1);
    await s.close();
  });

  it('inboundEdges returns no results for a different type sharing the same ID', async () => {
    const s = await open(storePath);
    const source = s.putNode('note', 'src', { title: 'Source' }, []);
    const noteTarget = s.putNode('note', 'shared', { title: 'Note Target' }, []);
    s.putNode('concept', 'shared', { title: 'Concept Target' }, []);
    s.putEdge(source, 'links_to', noteTarget, {});

    // concept/shared must see zero inbound edges
    expect(s.inboundEdges({ type: 'concept', id: 'shared' }).length).toBe(0);
    // note/shared must see the one inbound edge
    expect(s.inboundEdges(noteTarget).length).toBe(1);
    await s.close();
  });
});

describe('deleteNode inbound edge guard', () => {
  it('throws InvalidInputError when the node has only inbound edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});

    // bob has no outbound edges — only an inbound edge from alice
    expect(() => s.deleteNode('person', 'bob')).toThrow(InvalidInputError);
    await s.close();
  });
});

describe('confidence field', () => {
  it('sets confidence to a number and reads it back correctly', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { confidence: 0.85 });

    const edges = s.outboundEdges(alice);
    expect(edges[0].confidence).toBe(0.85);
    await s.close();
  });

  it('confidence defaults to null when not provided', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});

    const edges = s.outboundEdges(alice);
    expect(edges[0].confidence).toBeNull();
    await s.close();
  });

  it('confidence survives close/reopen cycle', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { confidence: 0.72 });
    await s.close();

    const s2 = await open(storePath);
    const edges = s2.outboundEdges({ type: 'person', id: 'alice' });
    expect(edges[0].confidence).toBe(0.72);
    await s2.close();
  });
});

describe('node ID and tag constraints', () => {
  it('rejects a node ID containing a colon', async () => {
    const s = await open(storePath);
    expect(() => s.putNode('note', 'bad:id', { title: 'T' }, [])).toThrow(InvalidInputError);
    await s.close();
  });

  it('rejects a node ID longer than 64 characters', async () => {
    const s = await open(storePath);
    const longID = 'a'.repeat(65);
    expect(() => s.putNode('note', longID, { title: 'T' }, [])).toThrow(InvalidInputError);
    await s.close();
  });

  it('rejects a tags array with more than 32 entries', async () => {
    const s = await open(storePath);
    const tooMany = Array.from({ length: 33 }, (_, i) => `tag${i}`);
    expect(() => s.putNode('note', 'n1', { title: 'T' }, tooMany)).toThrow(InvalidInputError);
    await s.close();
  });

  it('rejects a tags array with duplicate values', async () => {
    const s = await open(storePath);
    expect(() => s.putNode('note', 'n1', { title: 'T' }, ['alpha', 'beta', 'alpha'])).toThrow(InvalidInputError);
    await s.close();
  });
});
