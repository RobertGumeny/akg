// DOC-2: tie docs/manifest.json (the source the doc-graph is generated from) to
// the package's actual exported surface. CI already guarantees the .akg graph
// matches the manifest; this guarantees the manifest matches reality. Add a
// public export or Store method without documenting it — or document a symbol
// that no longer exists — and this test fails.
//
// undocumentedExports lists exported identifiers intentionally absent from the
// README-derived doc-graph. Growing it is a deliberate, reviewed choice, not a
// silent escape hatch.
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import ts from 'typescript';

const srcDir = resolve(__dirname, '../src');
const manifestPath = resolve(__dirname, '../docs/manifest.json');

// Exported but intentionally outside the doc-graph (with the reason).
const undocumentedExports: Record<string, string> = {
  RecencyFilter: 'recency filter option type; not part of the README-documented surface',
  EdgeRecencyFilter: 'recency filter option type; not part of the README-documented surface',
  hasUncompactedWAL: 'WAL introspection accessor; not part of the README-documented surface',
  nextWALSequence: 'WAL introspection accessor; not part of the README-documented surface',
  uncompactedWALEntryCount: 'WAL introspection accessor; not part of the README-documented surface',
  uncompactedWALByteCount: 'WAL introspection accessor; not part of the README-documented surface',
};

function sourceFile(name: string): ts.SourceFile {
  const file = join(srcDir, name);
  return ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true);
}

// Named exports declared in src/index.ts (both `export {}` and `export type {}`).
function indexExports(): Set<string> {
  const out = new Set<string>();
  sourceFile('index.ts').forEachChild((node) => {
    if (ts.isExportDeclaration(node) && node.exportClause && ts.isNamedExports(node.exportClause)) {
      for (const el of node.exportClause.elements) out.add(el.name.text);
    }
  });
  return out;
}

// Public instance methods and getters of the Store class (excludes private,
// protected, static, and constructor).
function storeMembers(): Set<string> {
  const out = new Set<string>();
  sourceFile('store.ts').forEachChild((node) => {
    if (!ts.isClassDeclaration(node) || node.name?.text !== 'Store') return;
    for (const m of node.members) {
      if (!ts.isMethodDeclaration(m) && !ts.isGetAccessorDeclaration(m)) continue;
      const mods = ts.canHaveModifiers(m) ? ts.getModifiers(m) ?? [] : [];
      const hidden = mods.some((mod) =>
        mod.kind === ts.SyntaxKind.PrivateKeyword ||
        mod.kind === ts.SyntaxKind.ProtectedKeyword ||
        mod.kind === ts.SyntaxKind.StaticKeyword);
      if (hidden) continue;
      if (m.name && ts.isIdentifier(m.name)) out.add(m.name.text);
    }
  });
  return out;
}

function documentedSymbols(): Set<string> {
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8')) as { nodes: Array<{ id: string }> };
  const prefix = 'api_symbol:';
  const out = new Set<string>();
  for (const n of manifest.nodes) {
    if (n.id.startsWith(prefix)) out.add(n.id.slice(prefix.length));
  }
  return out;
}

describe('docs manifest ↔ exported surface fidelity', () => {
  const surface = new Set<string>([...indexExports(), ...storeMembers()]);
  const documented = documentedSymbols();

  it('documents every exported symbol (or allow-lists it)', () => {
    const undocumented = [...surface]
      .filter((name) => !documented.has(name) && !(name in undocumentedExports))
      .sort();
    expect(undocumented, 'exported symbols missing from docs/manifest.json — document them ' +
      '(and regenerate the doc-graph), or add them to undocumentedExports with a reason').toEqual([]);
  });

  it('documents only real exported symbols', () => {
    const phantom = [...documented].filter((name) => !surface.has(name)).sort();
    expect(phantom, 'docs/manifest.json documents symbols not in the exported surface — ' +
      'remove them from the manifest').toEqual([]);
  });

  it('keeps the allow-list free of rot', () => {
    const stale = Object.keys(undocumentedExports).filter((name) => !surface.has(name)).sort();
    expect(stale, 'undocumentedExports lists identifiers that are no longer exported — drop them').toEqual([]);
  });
});
