// PERF-1: the secondary indexes back listNodesByTag / outboundEdges /
// inboundEdges with O(matches) lookups instead of O(total) full scans, and stay
// consistent across every mutation path (replace, delete, cascade, WAL replay).
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { open, Store } from '../src/store.js';

interface InternalState {
  tagIndex: Map<string, Set<string>>;
  outIndex: Map<string, Set<string>>;
  inIndex: Map<string, Set<string>>;
}

// `private state` is a TypeScript compile-time modifier, present at runtime —
// reach it for white-box index assertions (mirrors the akg-go in-package test).
function internalState(s: Store): InternalState {
  return (s as unknown as { state: InternalState }).state;
}

const nk = (type: string, id: string) => `${type}\0${id}`;

let dir: string;
beforeEach(() => { dir = mkdtempSync(join(tmpdir(), 'akg-ts-idx-')); });
afterEach(() => { rmSync(dir, { recursive: true }); });

describe('secondary indexes (PERF-1)', () => {
  it('are sized to matches, not total store size', async () => {
    const s = await open(join(dir, 'idx.akg'));
    // A mass of untagged nodes the indexes must NOT have to scan. Kept under the
    // 1000-entry WAL auto-flush threshold so the test exercises indexes, not
    // repeated whole-file rewrites (IO-1's concern).
    const bulk = 500;
    for (let i = 0; i < bulk; i++) s.putNode('bulk', `b${i}`, { title: 'b' }, []);
    for (let i = 0; i < 5; i++) s.putNode('doc', `d${i}`, { title: 'd' }, ['special']);
    s.putNode('hub', 'h', { title: 'hub' }, []);
    for (let i = 0; i < 10; i++) {
      s.putEdge({ type: 'hub', id: 'h' }, 'links', { type: 'bulk', id: `b${i}` }, {});
    }

    const state = internalState(s);
    expect(state.tagIndex.get('special')?.size, 'tag index must hold only the 5 matches').toBe(5);
    expect(state.outIndex.get(nk('hub', 'h'))?.size).toBe(10);
    expect(state.inIndex.get(nk('bulk', 'b0'))?.size).toBe(1);

    expect(s.listNodesByTag('special').length).toBe(5);
    expect(s.outboundEdges({ type: 'hub', id: 'h' }).length).toBe(10);
    expect(s.inboundEdges({ type: 'bulk', id: 'b0' }).length).toBe(1);
    await s.close();
  });

  it('stay consistent across replace, delete, edge delete, and cascade', async () => {
    const s = await open(join(dir, 'consistency.akg'));
    const state = internalState(s);

    // Tag replace: old leaves, new enters.
    s.putNode('doc', 'n1', { title: 't' }, ['old']);
    s.putNode('doc', 'n1', { title: 't' }, ['new']);
    expect(state.tagIndex.has('old')).toBe(false);
    expect(state.tagIndex.get('new')?.size).toBe(1);

    // Edge add/delete keeps both directions consistent.
    s.putNode('doc', 'n2', { title: 't' }, []);
    s.putEdge({ type: 'doc', id: 'n1' }, 'links', { type: 'doc', id: 'n2' }, {});
    expect(state.outIndex.get(nk('doc', 'n1'))?.size).toBe(1);
    expect(state.inIndex.get(nk('doc', 'n2'))?.size).toBe(1);
    s.deleteEdge({ type: 'doc', id: 'n1' }, 'links', { type: 'doc', id: 'n2' });
    expect(state.outIndex.has(nk('doc', 'n1'))).toBe(false);
    expect(state.inIndex.has(nk('doc', 'n2'))).toBe(false);

    // Node delete clears its tag entry.
    s.deleteNode('doc', 'n1');
    expect(state.tagIndex.has('new')).toBe(false);

    // Cascade clears incident edges (incl. a self-referential pair) and the node.
    s.putNode('doc', 'a', { title: 't' }, ['k']);
    s.putEdge({ type: 'doc', id: 'a' }, 'links', { type: 'doc', id: 'n2' }, {});
    s.putEdge({ type: 'doc', id: 'n2' }, 'links', { type: 'doc', id: 'a' }, {});
    const res = s.deleteNodeCascade('doc', 'a');
    expect(res).toEqual({ deletedInboundEdges: 1, deletedOutboundEdges: 1, deletedNode: true });
    expect(state.outIndex.has(nk('doc', 'a'))).toBe(false);
    expect(state.inIndex.has(nk('doc', 'a'))).toBe(false);
    expect(state.tagIndex.has('k')).toBe(false);
    await s.close();
  });

  it('rebuilds indexes from the committed WAL on reopen, honoring tag replace', async () => {
    const path = join(dir, 'replay.akg');
    const s = await open(path);
    // Two PUT_NODE records for the same node with different tags, both committed to
    // the WAL (no compaction) so reopen exercises the replay path, not hydrate.
    s.putNode('doc', 'r', { title: 't' }, ['first']);
    await s.commit();
    s.putNode('doc', 'r', { title: 't' }, ['second']);
    await s.close();

    const re = await open(path);
    const state = internalState(re);
    // The stale 'first' tag must not survive replay; only 'second' is indexed.
    expect(state.tagIndex.has('first')).toBe(false);
    expect(state.tagIndex.get('second')?.size).toBe(1);
    expect(re.listNodesByTag('first').length).toBe(0);
    expect(re.listNodesByTag('second').map((n) => n.id)).toEqual(['r']);
    await re.close();
  });
});
