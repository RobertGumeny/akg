import { describe, it, expect, beforeAll } from 'vitest';
import { spawnSync } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { open } from '../src/store.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const cli = join(__dirname, '../dist/cli.js');

function runCli(...args: string[]): { stdout: string; stderr: string; exitCode: number } {
  const result = spawnSync(process.execPath, [cli, ...args], { encoding: 'utf-8' });
  return {
    stdout: result.stdout ?? '',
    stderr: result.stderr ?? '',
    exitCode: result.status ?? 0,
  };
}

// Builds a small opponent/pattern graph in a temp .akg file — the same shape the
// durable poker agent produces — and returns its path.
async function writeFixture(): Promise<string> {
  const path = join(mkdtempSync(join(tmpdir(), 'akg-show-')), 'memory.akg');
  const store = await open(path);
  const opp = store.putNode('opponent', 'villain', {
    title: 'villain',
    body: 'Villain is loose-aggressive (VPIP 68%, PFR 30%).',
  }, []);
  const pat = store.putNode('pattern', 'folds-to-cbet', {
    title: 'Folds to flop c-bets',
    body: 'Villain folded to hero flop c-bet 36/75 times.',
  }, []);
  store.putEdge(opp, 'shows_pattern', pat, {});
  await store.commit();
  await store.close();
  return path;
}

describe('show CLI', () => {
  let fixture: string;

  beforeAll(async () => {
    fixture = await writeFixture();
  });

  it('renders knowledge grouped by node type with edges', () => {
    const { stdout, exitCode } = runCli('show', fixture);
    expect(exitCode).toBe(0);
    for (const want of [
      '2 nodes / 1 edges',
      'OPPONENT (1)',
      'Villain is loose-aggressive (VPIP 68%, PFR 30%).',
      'PATTERN (1)',
      'Folds to flop c-bets',
      'EDGES (1)',
      'opponent/villain -shows_pattern-> pattern/folds-to-cbet',
    ]) {
      expect(stdout).toContain(want);
    }
  });

  it('show --json emits a snapshot', () => {
    const { stdout, exitCode } = runCli('show', '--json', fixture);
    expect(exitCode).toBe(0);
    const parsed = JSON.parse(stdout);
    expect(parsed).toHaveProperty('nodes');
    expect(parsed).toHaveProperty('edges');
  });

  it('missing file exits 1 with a read error', () => {
    const missing = join(mkdtempSync(join(tmpdir(), 'akg-show-')), 'nope.akg');
    const { stderr, exitCode } = runCli('show', missing);
    expect(exitCode).toBe(1);
    expect(stderr).toContain('cannot read');
  });

  it('collapses large node types unless --all is passed', async () => {
    const path = join(mkdtempSync(join(tmpdir(), 'akg-show-')), 'big.akg');
    const store = await open(path);
    for (let i = 0; i < 15; i++) {
      store.putNode('hand', `h${i}`, { title: `Hand ${i}` }, []);
    }
    await store.commit();
    await store.close();

    const collapsed = runCli('show', path);
    expect(collapsed.exitCode).toBe(0);
    expect(collapsed.stdout).toContain('more (pass --all to show every node)');

    const all = runCli('show', '--all', path);
    expect(all.exitCode).toBe(0);
    expect(all.stdout).not.toContain('more (pass --all');
  });
});
