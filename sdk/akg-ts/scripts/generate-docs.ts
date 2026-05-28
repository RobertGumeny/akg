import { readFileSync, writeFileSync, unlinkSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { open, _setTestNow } from '../src/store.js';
import type { NodeRef } from '../src/types.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, '..');

interface ManifestNode {
  id: string;
  type: string;
  title: string;
  body: string;
  tags: string[];
  anchor: string;
  heading: string;
}

interface ManifestEdge {
  from: string;
  relation: string;
  to: string;
}

interface Manifest {
  meta: {
    language: string;
    package: string;
    version: string;
    source_path: string;
    generated_at: string;
  };
  nodes: ManifestNode[];
  edges: ManifestEdge[];
}

const manifest: Manifest = JSON.parse(
  readFileSync(join(root, 'docs/manifest.json'), 'utf-8'),
);

// Pin clock for determinism: all createdAt/updatedAt will be this fixed value.
const fixedMicros = BigInt(new Date(manifest.meta.generated_at).getTime()) * 1000n;
_setTestNow(fixedMicros);

const outAkg = join(root, 'docs/akg-ts-docs.akg');
const outJson = join(root, 'docs/akg-ts-docs.json');

// Always start from an empty file for determinism.
if (existsSync(outAkg)) unlinkSync(outAkg);

const store = await open(outAkg);

const refs = new Map<string, NodeRef>();

for (const node of manifest.nodes) {
  const colonIdx = node.id.indexOf(':');
  const akgType = node.id.slice(0, colonIdx);
  const akgId = node.id.slice(colonIdx + 1);

  const ref = store.putNode(
    akgType,
    akgId,
    {
      title: node.title,
      body: node.body,
      meta: {
        source_path: manifest.meta.source_path,
        heading: node.heading,
        anchor: node.anchor,
        language: manifest.meta.language,
        package: manifest.meta.package,
        version: manifest.meta.version,
        generated_at: manifest.meta.generated_at,
      },
    },
    node.tags,
  );
  refs.set(node.id, ref);
}

for (const edge of manifest.edges) {
  const fromRef = refs.get(edge.from);
  const toRef = refs.get(edge.to);
  if (!fromRef || !toRef) {
    console.error(`Edge references unknown node: ${edge.from} -> ${edge.to}`);
    process.exit(1);
  }
  store.putEdge(fromRef, edge.relation, toRef, {});
}

const snap = store.snapshot();

// compact() rewrites without WAL section, keeping binary output minimal and stable.
await store.compact();
await store.close();

_setTestNow(null);

writeFileSync(outJson, JSON.stringify(snap, null, 2) + '\n', 'utf-8');

console.log(`Generated ${manifest.nodes.length} nodes, ${manifest.edges.length} edges`);
console.log(`  ${outAkg}`);
console.log(`  ${outJson}`);
