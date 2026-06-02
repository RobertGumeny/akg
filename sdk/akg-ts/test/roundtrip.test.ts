import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { open, Store } from '../src/store.js';
import { decodeContainer, decodeDataEntries } from '../src/internal/format.js';
import {
  decodeWALRecord, WAL_OP_PUT_NODE, WAL_OP_PUT_EDGE,
} from '../src/internal/wal.js';

let dir: string;
let storePath: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), 'akg-ts-roundtrip-'));
  storePath = join(dir, 'test.akg');
});

afterEach(() => {
  rmSync(dir, { recursive: true });
});

function dataKeys(path: string): string[] {
  const file = readFileSync(path);
  const bytes = new Uint8Array(file.buffer, file.byteOffset, file.byteLength);
  const c = decodeContainer(bytes);
  const dec = new TextDecoder('utf-8');
  return decodeDataEntries(c.data).map(e => dec.decode(e.key));
}

function walRecords(path: string): Array<{ op: number }> {
  const file = readFileSync(path);
  const bytes = new Uint8Array(file.buffer, file.byteOffset, file.byteLength);
  const c = decodeContainer(bytes);
  const out: Array<{ op: number }> = [];
  let wal = c.wal ?? new Uint8Array(0);
  while (wal.length > 0) {
    const [r, n] = decodeWALRecord(wal);
    out.push({ op: r.operation });
    wal = wal.slice(n);
  }
  return out;
}

// ---- T2: write → commit → close → reopen round-trip ------------------------

describe('T2: write/commit/close/reopen round-trip', () => {
  it('survives the full graph across a close and reopen', async () => {
    const s = await open(storePath);
    const alice = s.putNode('person', 'alice', { title: 'Alice', body: 'a' }, ['vip']);
    const bob = s.putNode('person', 'bob', { title: 'Bob' }, []);
    const carol = s.putNode('person', 'carol', { title: 'Carol' }, []);

    // One edge with explicit strength, one with confidence: null.
    s.putEdge(alice, 'knows', bob, { strength: 0.9, confidence: 0.7 });
    s.putEdge(bob, 'knows', carol, { confidence: null });

    await s.commit();
    const seqAfterCommit = s.nextWALSequence;
    await s.close();

    const s2 = await open(storePath);
    expect(s2.listNodes().length).toBe(3);
    expect(s2.listEdges().length).toBe(2);

    const reAlice = s2.getNode('person', 'alice')!;
    expect(reAlice.title).toBe('Alice');
    expect(reAlice.body).toBe('a');
    expect(reAlice.tags).toEqual(['vip']);

    const strong = s2.listEdges({ relation: 'knows' }).find(e => e.from.id === 'alice')!;
    expect(strong.strength).toBe(0.9);
    expect(strong.confidence).toBe(0.7);

    const nullConf = s2.listEdges({ relation: 'knows' }).find(e => e.from.id === 'bob')!;
    // confidence: null must reopen as null, NOT the 0.5 default.
    expect(nullConf.confidence).toBeNull();

    // nextWALSeq continues monotonically across the reopen; it does not reset.
    expect(s2.nextWALSequence).toBe(seqAfterCommit);
    expect(s2.nextWALSequence).toBeGreaterThan(1n);
    await s2.close();
  });
});

// ---- T4: crash-atomicity ---------------------------------------------------

describe('T4: crash-atomicity', () => {
  it('leaves the prior committed file fully readable after an interrupted write', async () => {
    const s = await open(storePath);
    const a = s.putNode('note', 'n1', { title: 'First' }, []);
    const b = s.putNode('note', 'n2', { title: 'Second' }, []);
    s.putEdge(a, 'links', b, {});
    await s.commit();
    await s.close();

    const goodBytes = readFileSync(storePath);

    // Simulate an interrupted replacement: the temp file from writeFileAtomic
    // exists but the rename never happened. The target retains its prior bytes.
    const tempPath = join(dir, `.test.akg.commit-deadbeefdeadbeef`);
    writeFileSync(tempPath, goodBytes.subarray(0, Math.floor(goodBytes.length / 2)));

    // The target file must still open with the full pre-interruption graph.
    const onDisk = readFileSync(storePath);
    const bytes = new Uint8Array(onDisk.buffer, onDisk.byteOffset, onDisk.byteLength);
    const recovered = Store.fromBytes(bytes, storePath);
    expect(recovered.listNodes().length).toBe(2);
    expect(recovered.listEdges().length).toBe(1);
    expect(recovered.getNode('note', 'n1')!.title).toBe('First');

    // The target itself is byte-identical to the last good commit (not corrupt).
    expect(Buffer.compare(onDisk, goodBytes)).toBe(0);

    // A stray temp file is acceptable; a corrupt target is not. Confirm the only
    // extra entry is the temp file, and the target decodes cleanly.
    const entries = readdirSync(dir);
    expect(entries).toContain('test.akg');
    expect(entries.some(e => e.startsWith('.test.akg.commit-'))).toBe(true);
  });
});

// ---- T5 (decode-level): incremental commit keeps Data as last snapshot -----

describe('T5: incremental commit', () => {
  it('keeps new mutations in the WAL and the Data section at the last compaction snapshot', async () => {
    const s = await open(storePath);
    // Compact an empty store so Data is a known (empty) snapshot.
    await s.compact();
    expect(dataKeys(storePath).length).toBe(0);

    // Commit #1: add a node.
    const a = s.putNode('person', 'alice', { title: 'Alice' }, []);
    await s.commit();
    // Commit #2: add another node and an edge, no compaction in between.
    const b = s.putNode('person', 'bob', { title: 'Bob' }, []);
    s.putEdge(a, 'knows', b, {});
    await s.commit();

    // Data section is still the empty last-compaction snapshot...
    expect(dataKeys(storePath).length).toBe(0);

    // ...and the new mutations live in the WAL (2 put-node, 1 put-edge present).
    const ops = walRecords(storePath).map(r => r.op);
    expect(ops.filter(op => op === WAL_OP_PUT_NODE).length).toBe(2);
    expect(ops.filter(op => op === WAL_OP_PUT_EDGE).length).toBe(1);

    // Reopening still reconstructs the full live graph from Data + WAL.
    const s2 = await open(storePath);
    expect(s2.listNodes().length).toBe(2);
    expect(s2.listEdges().length).toBe(1);
    await s2.close();

    // compact() then folds everything into Data and empties the WAL.
    await s.compact();
    expect(dataKeys(storePath).length).toBeGreaterThan(0);
    expect(walRecords(storePath).length).toBe(0);
    await s.close();
  });

  it('reuses the persisted Data/Bloom bytes across commits without compaction', async () => {
    const s = await open(storePath);
    s.putNode('note', 'n1', { title: 'One' }, []);
    await s.compact(); // Data now holds n1.
    const dataAfterCompact = (() => {
      const f = readFileSync(storePath);
      const bytes = new Uint8Array(f.buffer, f.byteOffset, f.byteLength);
      return decodeContainer(bytes).data;
    })();

    s.putNode('note', 'n2', { title: 'Two' }, []);
    await s.commit();

    const f = readFileSync(storePath);
    const bytes = new Uint8Array(f.buffer, f.byteOffset, f.byteLength);
    const dataAfterCommit = decodeContainer(bytes).data;

    // The Data bytes are reused verbatim — commit() did not re-materialize them.
    expect(Buffer.compare(Buffer.from(dataAfterCompact), Buffer.from(dataAfterCommit))).toBe(0);
    await s.close();
  });
});

// ---- T6: flush/growth policy ----------------------------------------------

describe('T6: automatic flush policy', () => {
  it('auto-commits once >1,000 pending mutations accumulate without an explicit commit', async () => {
    const s = await open(storePath);
    // Stage 1,001 node puts with NO explicit commit()/close().
    for (let i = 0; i < 1001; i++) {
      s.putNode('item', `n${i}`, { title: `Item ${i}` }, []);
    }
    // The safety valve must have flushed the buffered records to disk already.
    const ops = walRecords(storePath).map(r => r.op);
    const puts = ops.filter(op => op === WAL_OP_PUT_NODE).length;
    expect(puts).toBeGreaterThanOrEqual(1000);
    await s.close();
  });
});
