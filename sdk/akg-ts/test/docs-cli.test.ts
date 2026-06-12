import { describe, it, expect } from 'vitest';
import { spawnSync } from 'node:child_process';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { existsSync } from 'node:fs';

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

describe('docs CLI', () => {
  it('dist/cli.js must exist (run npm run build first)', () => {
    expect(existsSync(cli)).toBe(true);
  });

  it('docs overview', () => {
    const { stdout, exitCode } = runCli('docs', 'overview');
    expect(exitCode).toBe(0);
    expect(stdout).toMatchSnapshot();
  });

  it('docs explain putNode', () => {
    const { stdout, exitCode } = runCli('docs', 'explain', 'putNode');
    expect(exitCode).toBe(0);
    expect(stdout).toMatchSnapshot();
  });

  it('docs explain putNode output contains putNode and relation heading', () => {
    const { stdout } = runCli('docs', 'explain', 'putNode');
    expect(stdout).toContain('putNode');
    expect(stdout).toMatch(/### (implements|see_also|depends_on|source_of_truth)/);
  });

  it('docs search "commit"', () => {
    const { stdout, exitCode } = runCli('docs', 'search', 'commit');
    expect(exitCode).toBe(0);
    expect(stdout).toMatchSnapshot();
  });

  it('docs search "delete" contains deleteNode and deleteEdge', () => {
    const { stdout } = runCli('docs', 'search', 'delete');
    expect(stdout).toContain('deleteNode');
    expect(stdout).toContain('deleteEdge');
  });

  it('docs dump --format json', () => {
    const { stdout, exitCode } = runCli('docs', 'dump', '--format', 'json');
    expect(exitCode).toBe(0);
    const parsed = JSON.parse(stdout);
    expect(parsed).toHaveProperty('nodes');
    expect(parsed).toHaveProperty('edges');
    expect(stdout).toMatchSnapshot();
  });

  it('docs dump --format markdown', () => {
    const { stdout, exitCode } = runCli('docs', 'dump', '--format', 'markdown');
    expect(exitCode).toBe(0);
    expect(stdout.length).toBeGreaterThan(0);
    expect(stdout).toMatchSnapshot();
  });

  it('docs explain unknownSymbol exits 1 with not found message', () => {
    const { stderr, exitCode } = runCli('docs', 'explain', 'unknownSymbol');
    expect(exitCode).toBe(1);
    expect(stderr).toContain('Not found');
  });
});
