import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { open, Store, _setTestNow } from '../src/store.js';
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

  it('edge strength defaults to 0.5 when omitted', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    const edges = s.outboundEdges(alice);
    expect(edges[0].strength).toBe(0.5);
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

// --- SDK-PARITY-001: edge strength default ---

describe('edge strength default', () => {
  it('explicitly supplied strength round-trips unchanged', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { strength: 0.75 });
    await s.close();
    const s2 = await open(storePath);
    const edges = s2.outboundEdges({ type: 'person', id: 'alice' });
    expect(edges[0].strength).toBe(0.75);
    await s2.close();
  });
});

// --- SDK-PARITY-002: compaction ---

describe('compact', () => {
  it('commits pending mutations before compacting', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'hello' }, []);
    await s.compact();
    const node = s.getNode('note', 'n1');
    expect(node).not.toBeNull();
    expect(node?.title).toBe('hello');
    await s.close();
  });

  it('produces a file with an empty WAL section after compaction', async () => {
    const { readFileSync } = await import('node:fs');
    const { decodeContainer } = await import('../src/internal/format.js');
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.deleteEdge(alice, 'knows', bob);
    await s.compact();
    const file = readFileSync(storePath);
    const bytes = new Uint8Array(file.buffer, file.byteOffset, file.byteLength);
    const c = decodeContainer(bytes);
    // Compaction resets the WAL to an empty section (present but zero-length),
    // matching the Go reference so incremental commit() can append onto it.
    expect(c.wal).not.toBeNull();
    expect(c.wal!.length).toBe(0);
    await s.close();
  });

  it('store remains usable and graph content is unchanged after compaction', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, { strength: 0.7 });
    await s.compact();
    expect(s.listNodes()).toHaveLength(2);
    expect(s.listEdges()[0].strength).toBe(0.7);
    await s.close();
    const s2 = await open(storePath);
    expect(s2.listNodes()).toHaveLength(2);
    expect(s2.listEdges()[0].strength).toBe(0.7);
    await s2.close();
  });
});

// --- SDK-PARITY-003: global edge listing and snapshots ---

describe('listEdges', () => {
  it('returns all live edges with empty filter', async () => {
    const s = await open(storePath);
    const n1 = s.putNode('note', 'n1', { title: 'one' }, []);
    const n2 = s.putNode('note', 'n2', { title: 'two' }, []);
    const n3 = s.putNode('note', 'n3', { title: 'three' }, []);
    s.putEdge(n1, 'links_to', n2, {});
    s.putEdge(n2, 'mentions', n3, {});
    s.putEdge(n1, 'mentions', n3, {});
    expect(s.listEdges()).toHaveLength(3);
    await s.close();
  });

  it('filters by relation', async () => {
    const s = await open(storePath);
    const n1 = s.putNode('note', 'n1', { title: 'one' }, []);
    const n2 = s.putNode('note', 'n2', { title: 'two' }, []);
    const n3 = s.putNode('note', 'n3', { title: 'three' }, []);
    s.putEdge(n1, 'links_to', n2, {});
    s.putEdge(n1, 'mentions', n3, {});
    const edges = s.listEdges({ relation: 'links_to' });
    expect(edges).toHaveLength(1);
    expect(edges[0].relation).toBe('links_to');
    await s.close();
  });

  it('filters by metadata', async () => {
    const s = await open(storePath);
    const n1 = s.putNode('note', 'n1', { title: 'one' }, []);
    const n2 = s.putNode('note', 'n2', { title: 'two' }, []);
    const n3 = s.putNode('note', 'n3', { title: 'three' }, []);
    s.putEdge(n1, 'links_to', n2, { meta: { source: 'inferred' } });
    s.putEdge(n1, 'links_to', n3, { meta: { source: 'manual' } });
    const edges = s.listEdges({ meta: { source: 'inferred' } });
    expect(edges).toHaveLength(1);
    expect(edges[0].to.id).toBe('n2');
    await s.close();
  });

  it('combines relation and meta with AND semantics', async () => {
    const s = await open(storePath);
    const n1 = s.putNode('note', 'n1', { title: 'one' }, []);
    const n2 = s.putNode('note', 'n2', { title: 'two' }, []);
    const n3 = s.putNode('note', 'n3', { title: 'three' }, []);
    s.putEdge(n1, 'links_to', n2, { meta: { source: 'inferred' } });
    s.putEdge(n1, 'mentions', n3, { meta: { source: 'inferred' } });
    const edges = s.listEdges({ relation: 'links_to', meta: { source: 'inferred' } });
    expect(edges).toHaveLength(1);
    expect(edges[0].relation).toBe('links_to');
    await s.close();
  });
});

describe('snapshot', () => {
  it('returns all live nodes and edges', async () => {
    const s = await open(storePath);
    const n1 = s.putNode('note', 'n1', { title: 'one' }, []);
    const n2 = s.putNode('note', 'n2', { title: 'two' }, []);
    s.putEdge(n1, 'links_to', n2, {});
    const snap = s.snapshot();
    expect(snap.nodes).toHaveLength(2);
    expect(snap.edges).toHaveLength(1);
    const encoded = JSON.stringify(snap);
    expect(typeof encoded).toBe('string');
    await s.close();
  });
});

// --- SDK-PARITY-004: node filtering and batch inspection ---

describe('listNodesFiltered', () => {
  it('filters by type and tag with AND semantics', async () => {
    const s = await open(storePath);
    s.putNode('person', 'alice', { title: 'Alice' }, ['active']);
    s.putNode('person', 'bob', { title: 'Bob' }, ['inactive']);
    s.putNode('task', 't1', { title: 'Task' }, ['active']);
    const nodes = s.listNodesFiltered({ type: 'person', tag: 'active' });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].id).toBe('alice');
    await s.close();
  });

  it('filters by metadata scalar', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one', meta: { status: 'accepted' } }, []);
    s.putNode('note', 'n2', { title: 'two', meta: { status: 'rejected' } }, []);
    const nodes = s.listNodesFiltered({ meta: { status: 'accepted' } });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].id).toBe('n1');
    await s.close();
  });

  it('filters by metadata array equality', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one', meta: { tags: ['a', 'b'] } }, []);
    s.putNode('note', 'n2', { title: 'two', meta: { tags: ['a'] } }, []);
    const nodes = s.listNodesFiltered({ meta: { tags: ['a', 'b'] } });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].id).toBe('n1');
    await s.close();
  });

  it('filters by metadata object equality ignoring key order', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one', meta: { obj: { x: 1, y: 2 } } }, []);
    s.putNode('note', 'n2', { title: 'two', meta: { obj: { x: 1, y: 99 } } }, []);
    const nodes = s.listNodesFiltered({ meta: { obj: { y: 2, x: 1 } } });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].id).toBe('n1');
    await s.close();
  });

  it('excludes nodes when filter key is missing', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one' }, []);
    const nodes = s.listNodesFiltered({ meta: { nonexistent: 'x' } });
    expect(nodes).toHaveLength(0);
    await s.close();
  });

  it('returns empty for unknown type', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one' }, []);
    const nodes = s.listNodesFiltered({ type: 'nonexistent' });
    expect(nodes).toHaveLength(0);
    await s.close();
  });
});

describe('getNodes', () => {
  it('preserves input order and handles missing refs as null', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one' }, []);
    s.putNode('note', 'n2', { title: 'two' }, []);
    const results = s.getNodes([
      { type: 'note', id: 'n2' },
      { type: 'note', id: 'n1' },
      { type: 'note', id: 'missing' },
    ]);
    expect(results).toHaveLength(3);
    expect(results[0]?.id).toBe('n2');
    expect(results[1]?.id).toBe('n1');
    expect(results[2]).toBeNull();
    await s.close();
  });

  it('preserves duplicate refs as duplicate positions', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'one' }, []);
    const results = s.getNodes([
      { type: 'note', id: 'n1' },
      { type: 'note', id: 'n1' },
    ]);
    expect(results).toHaveLength(2);
    expect(results[0]?.id).toBe('n1');
    expect(results[1]?.id).toBe('n1');
    await s.close();
  });
});

// --- SDK-PARITY-005: recency helpers ---

describe('recentNodes', () => {
  afterEach(() => _setTestNow(null));

  it('returns nodes newest-first by updatedAt', async () => {
    const s = await open(storePath);
    _setTestNow(100n);
    s.putNode('task', 't1', { title: 'Task1' }, []);
    _setTestNow(200n);
    s.putNode('task', 't2', { title: 'Task2' }, []);
    _setTestNow(300n);
    s.putNode('task', 't3', { title: 'Task3' }, []);
    const nodes = s.recentNodes();
    expect(nodes.map(n => n.id)).toEqual(['t3', 't2', 't1']);
    await s.close();
  });

  it('filters by type and tag', async () => {
    const s = await open(storePath);
    _setTestNow(100n);
    s.putNode('task', 't1', { title: 'T1' }, ['active']);
    s.putNode('task', 't2', { title: 'T2' }, ['inactive']);
    s.putNode('note', 'n1', { title: 'N1' }, ['active']);
    const nodes = s.recentNodes({ type: 'task', tag: 'active' });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].id).toBe('t1');
    await s.close();
  });

  it('applies inclusive sinceUpdatedAt and untilUpdatedAt bounds', async () => {
    const s = await open(storePath);
    _setTestNow(100n);
    s.putNode('task', 't1', { title: 'T1' }, []);
    _setTestNow(200n);
    s.putNode('task', 't2', { title: 'T2' }, []);
    _setTestNow(300n);
    s.putNode('task', 't3', { title: 'T3' }, []);

    const since = s.recentNodes({ sinceUpdatedAt: 200 });
    expect(since.map(n => n.id)).toEqual(['t3', 't2']);

    const until = s.recentNodes({ untilUpdatedAt: 200 });
    expect(until.map(n => n.id)).toEqual(['t2', 't1']);

    const range = s.recentNodes({ sinceUpdatedAt: 150, untilUpdatedAt: 250 });
    expect(range.map(n => n.id)).toEqual(['t2']);
    await s.close();
  });

  it('applies positive limit after sorting', async () => {
    const s = await open(storePath);
    _setTestNow(100n);
    s.putNode('task', 't1', { title: 'T1' }, []);
    _setTestNow(200n);
    s.putNode('task', 't2', { title: 'T2' }, []);
    _setTestNow(300n);
    s.putNode('task', 't3', { title: 'T3' }, []);
    const nodes = s.recentNodes({ limit: 2 });
    expect(nodes.map(n => n.id)).toEqual(['t3', 't2']);
    await s.close();
  });

  it('returns all records when limit is 0', async () => {
    const s = await open(storePath);
    s.putNode('task', 't1', { title: 'T1' }, []);
    s.putNode('task', 't2', { title: 'T2' }, []);
    const nodes = s.recentNodes({ limit: 0 });
    expect(nodes).toHaveLength(2);
    await s.close();
  });

  it('throws InvalidInputError for negative limit', async () => {
    const s = await open(storePath);
    expect(() => s.recentNodes({ limit: -1 })).toThrow(InvalidInputError);
    await s.close();
  });
});

describe('recentEdges', () => {
  afterEach(() => _setTestNow(null));

  it('returns edges newest-first by updatedAt', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    _setTestNow(100n);
    s.putEdge(alice, 'knows', bob, {});
    _setTestNow(200n);
    s.putEdge(alice, 'knows', carol, {});
    const edges = s.recentEdges({ from: alice });
    expect(edges.map(e => e.to.id)).toEqual(['carol', 'bob']);
    await s.close();
  });

  it('filters by from endpoint', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.putEdge(alice, 'knows', carol, {});
    s.putEdge(bob, 'knows', carol, {});
    const edges = s.recentEdges({ from: alice });
    expect(edges).toHaveLength(2);
    await s.close();
  });

  it('filters by to endpoint', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'knows', carol, {});
    s.putEdge(bob, 'knows', carol, {});
    s.putEdge(alice, 'knows', bob, {});
    const edges = s.recentEdges({ to: carol });
    expect(edges).toHaveLength(2);
    await s.close();
  });

  it('combines from, relation, and time filters with AND semantics', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.putEdge(alice, 'likes', carol, {});
    const edges = s.recentEdges({ from: alice, relation: 'knows' });
    expect(edges).toHaveLength(1);
    expect(edges[0].to.id).toBe('bob');
    await s.close();
  });

  it('throws InvalidInputError for negative limit', async () => {
    const s = await open(storePath);
    expect(() => s.recentEdges({ limit: -1 })).toThrow(InvalidInputError);
    await s.close();
  });
});

// --- SDK-PARITY-006: edge reconciliation ---

describe('reconcileOutboundEdges', () => {
  it('adds missing desired edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    const result = s.reconcileOutboundEdges(alice, 'knows', [bob, carol], { strength: 0.8 });
    expect(result.added).toBe(2);
    expect(result.removed).toBe(0);
    expect(result.unchanged).toBe(0);
    expect(s.outboundEdges(alice, 'knows')).toHaveLength(2);
    await s.close();
  });

  it('removes stale edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.putEdge(alice, 'knows', carol, {});
    const result = s.reconcileOutboundEdges(alice, 'knows', [bob], {});
    expect(result.added).toBe(0);
    expect(result.removed).toBe(1);
    expect(result.unchanged).toBe(1);
    expect(s.outboundEdges(alice, 'knows')).toHaveLength(1);
    await s.close();
  });

  it('leaves unrelated edges unchanged', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'likes', carol, {});
    s.reconcileOutboundEdges(alice, 'knows', [bob], {});
    expect(s.outboundEdges(alice, 'likes')).toHaveLength(1);
    await s.close();
  });
});

// --- SDK-PARITY-007: cascade delete ---

describe('deleteNodeCascade', () => {
  it('deletes inbound and outbound edges before the node', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);
    s.putEdge(alice, 'knows', bob, {});
    s.putEdge(carol, 'knows', alice, {});
    const result = s.deleteNodeCascade('person', 'alice');
    expect(result.deletedInboundEdges).toBe(1);
    expect(result.deletedOutboundEdges).toBe(1);
    expect(result.deletedNode).toBe(true);
    expect(s.getNode('person', 'alice')).toBeNull();
    expect(s.inboundEdges({ type: 'person', id: 'bob' }, 'knows')).toHaveLength(0);
    await s.close();
  });

  it('normal deleteNode still rejects nodes with live edges', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice' }, []);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(alice, 'knows', bob, {});
    expect(() => s.deleteNode('person', 'alice')).toThrow(InvalidInputError);
    await s.close();
  });

  it('throws NotFoundError for nonexistent node', async () => {
    const s = await open(storePath);
    const { NotFoundError } = await import('../src/errors.js');
    expect(() => s.deleteNodeCascade('person', 'nonexistent')).toThrow(NotFoundError);
    await s.close();
  });
});
