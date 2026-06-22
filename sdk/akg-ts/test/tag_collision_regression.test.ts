// SDK-level regression + read-compat tests for the tag-index key collision.
// Mirrors sdk/akg-go/tag_collision_regression_test.go.
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { open } from '../src/store.js';
import { decodeContainer, decodeDataEntries, CURRENT_MAJOR } from '../src/internal/format.js';
import { parseTagKey } from '../src/internal/keys.js';

let dir: string;

beforeEach(() => {
  dir = mkdtempSync(join(tmpdir(), 'akg-ts-collision-'));
});

afterEach(() => {
  rmSync(dir, { recursive: true });
});

describe('tag-index key collision', () => {
  // Two nodes share the id "preflop__vpip" across types (counter, tendency), both
  // tagged "preflop" (EPIC-18-003). Under the major-1 tag key both collapsed to
  // one key and compaction threw "duplicate data key"; the major-2 type-qualified
  // key keeps them distinct, so commit + compact succeed and both resolve.
  it('compacts cleanly and keeps colliding nodes distinct', async () => {
    const s = await open(join(dir, 'collision.akg'));
    s.putNode('counter', 'preflop__vpip', { title: 'VPIP counter' }, ['preflop']);
    s.putNode('tendency', 'preflop__vpip', { title: 'VPIP tendency' }, ['preflop']);
    await s.commit();
    await s.compact(); // regression: previously threw "duplicate data key"

    expect(s.getNode('counter', 'preflop__vpip')?.title).toBe('VPIP counter');
    expect(s.getNode('tendency', 'preflop__vpip')?.title).toBe('VPIP tendency');
    expect(s.listNodesByTag('preflop')).toHaveLength(2);
    await s.close();
  });

  // A major-1 file (3-part tag key t:{tag}:{id}) opens and reads clean under the
  // major-2 reader, and compaction rewrites it as a major-2 file whose tag index
  // uses the type-qualified 4-part key (t:{tag}:{type}:{id}).
  it('reads a major-1 file and upgrades its tag keys on compaction', async () => {
    const src = resolve(__dirname, '../../../testdata/conformance/m2-compacted.akg');
    const orig = readFileSync(src);
    expect(orig[4]).toBe(1); // precondition: fixture is major 1

    const path = join(dir, 'compacted.akg');
    writeFileSync(path, orig);

    const s = await open(path);
    // Fixture's single node n:note:live carries the tag "current" (v1 key).
    const before = s.listNodesByTag('current');
    expect(before).toHaveLength(1);
    expect(before[0].id).toBe('live');
    await s.compact();
    await s.close();

    const upgraded = new Uint8Array(readFileSync(path));
    expect(upgraded[4]).toBe(CURRENT_MAJOR);

    const c = decodeContainer(upgraded);
    const entries = decodeDataEntries(c.data);
    const dec = new TextDecoder('utf-8');
    const tagKeys = entries
      .map(e => dec.decode(e.key))
      .filter(k => k.startsWith('t:'));
    expect(tagKeys).toHaveLength(1);
    const [tag, type, id] = parseTagKey(tagKeys[0]);
    expect(type).not.toBe(''); // type-qualified (4-part) after upgrade
    expect([tag, type, id]).toEqual(['current', 'note', 'live']);
  });
});
