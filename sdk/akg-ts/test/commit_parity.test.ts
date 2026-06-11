// Cross-SDK write-path parity tests.
//
// The reference SDK (internal/store), akg-go, and akg-ts must all produce
// byte-identical output for the canonical commit-append sequence. This file is
// akg-ts's reproduction; the golden it compares against
// (testdata/behavior/parity-commit-append.akg) is shared by all three. If akg-ts
// diverges on the write path — re-materializing Data, encoding the WAL
// differently, or emitting a different container — TestCommitAppendByteParity
// fails. It also asserts CONF-3's no-re-materialization contract directly.
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { open, _setTestNow } from '../src/store.js';
import { decodeContainer } from '../src/internal/format.js';

const GOLDEN = resolve(__dirname, '../../../testdata/behavior/parity-commit-append.akg');

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), 'akg-ts-parity-'));
  // Constant clock so byte output cannot depend on how many times the
  // implementation samples its clock per mutation: every record stamped 1_000_000.
  _setTestNow(1_000_000n);
});

afterEach(() => {
  _setTestNow(null);
  rmSync(dir, { recursive: true });
});

// Applies the canonical commit-append sequence to a fresh store, then returns the
// resulting file bytes: putNode n1 / commit / putNode n2 / commit, which must
// leave Data+Bloom empty and grow the WAL to four records.
async function parityAppendSequence(path: string): Promise<Uint8Array> {
  const s = await open(path);
  s.putNode('note', 'n1', { title: 'One' }, []);
  await s.commit();
  s.putNode('note', 'n2', { title: 'Two' }, []);
  await s.commit();
  const buf = readFileSync(path);
  return new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
}

describe('commit-append cross-SDK byte parity', () => {
  it('reproduces the shared golden byte-for-byte', async () => {
    const got = await parityAppendSequence(join(dir, 'out.akg'));

    if (process.env.WRITE_PARITY_GOLDEN) {
      writeFileSync(GOLDEN, got);
      return;
    }

    const buf = readFileSync(GOLDEN);
    const want = new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
    expect(Buffer.from(got).equals(Buffer.from(want))).toBe(true);
  });

  it('does not re-materialize Data on a single-record commit (CONF-3)', async () => {
    const path = join(dir, 'rematerialize.akg');
    const s = await open(path);
    s.putNode('note', 'n1', { title: 'One' }, []);
    await s.compact(); // establish a non-empty Data baseline

    const before = decodeContainer(toBytes(readFileSync(path)));

    s.putNode('note', 'n2', { title: 'Two' }, []);
    await s.commit();

    const after = decodeContainer(toBytes(readFileSync(path)));

    expect(Buffer.from(after.data).equals(Buffer.from(before.data)),
      'Data section changed on commit — store re-materialized Data instead of appending to the WAL').toBe(true);
    const beforeBloom = before.bloom ?? new Uint8Array(0);
    const afterBloom = after.bloom ?? new Uint8Array(0);
    expect(Buffer.from(afterBloom).equals(Buffer.from(beforeBloom)),
      'Bloom section changed on commit — Bloom is a Data index and must only change on compaction').toBe(true);
    const beforeWal = before.wal ?? new Uint8Array(0);
    const afterWal = after.wal ?? new Uint8Array(0);
    expect(afterWal.length).toBeGreaterThan(beforeWal.length);

    expect(s.getNode('note', 'n2')).not.toBeNull();
  });
});

function toBytes(buf: Buffer): Uint8Array {
  return new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
}
