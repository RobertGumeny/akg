import { open } from './store.js';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, dirname } from 'node:path';
import type { Node, Edge, Snapshot } from './types.js';

// Node types above this count are collapsed to a summary so a long session
// (hundreds of nodes of one type) still prints as one clean screen. --all overrides.
const COLLAPSE_THRESHOLD = 12;

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const DOCS_PATH = join(__dirname, '../docs/akg-ts-docs.akg');

async function loadStore() {
  return open(DOCS_PATH);
}

// ---- formatting helpers ----

function nodeToMarkdown(node: Node): string {
  const lines: string[] = [];
  lines.push(`**${node.title}** (\`${node.type}/${node.id}\`)`);
  if (node.body) lines.push(`> ${node.body}`);
  if (node.tags.length > 0) lines.push(`Tags: ${node.tags.map(t => `\`${t}\``).join(', ')}`);
  return lines.join('\n');
}

// ---- subcommand: show <PATH> ----

function titleOf(n: Node): string {
  return n.title.trim() !== '' ? n.title : n.id;
}

function edgeKey(e: Edge): string {
  return `${e.from.type}/${e.from.id}|${e.relation}|${e.to.type}/${e.to.id}`;
}

// Renders a snapshot as readable text: knowledge grouped by the node types an
// application invented, smallest groups first (curated, high-signal types lead;
// bulky high-volume types sink to the bottom), with edges as `from -relation-> to`.
function renderText(path: string, snap: Snapshot, all: boolean): string {
  const out: string[] = [`${path}\n${snap.nodes.length} nodes / ${snap.edges.length} edges`];

  const byType = new Map<string, Node[]>();
  for (const n of snap.nodes) {
    if (!byType.has(n.type)) byType.set(n.type, []);
    byType.get(n.type)!.push(n);
  }

  const types = [...byType.keys()].sort((a, b) => {
    const la = byType.get(a)!.length;
    const lb = byType.get(b)!.length;
    return la !== lb ? la - lb : a < b ? -1 : a > b ? 1 : 0;
  });

  for (const t of types) {
    const nodes = byType.get(t)!.sort((x, y) => (x.id < y.id ? -1 : x.id > y.id ? 1 : 0));
    out.push(`\n${t.toUpperCase()} (${nodes.length})`);

    if (!all && nodes.length > COLLAPSE_THRESHOLD) {
      for (const n of nodes.slice(0, 3)) out.push(`  - ${titleOf(n)}`);
      out.push(`  ... ${nodes.length - 3} more (pass --all to show every node)`);
      continue;
    }
    for (const n of nodes) {
      out.push(`  ${titleOf(n)}`);
      const body = n.body.trim();
      if (body) out.push(`    ${body.replace(/\n/g, '\n    ')}`);
    }
  }

  if (snap.edges.length > 0) {
    out.push(`\nEDGES (${snap.edges.length})`);
    const edges = [...snap.edges].sort((a, b) => {
      const ka = edgeKey(a);
      const kb = edgeKey(b);
      return ka < kb ? -1 : ka > kb ? 1 : 0;
    });
    const shown = !all && edges.length > COLLAPSE_THRESHOLD ? edges.slice(0, COLLAPSE_THRESHOLD) : edges;
    for (const e of shown) out.push(`  ${e.from.type}/${e.from.id} -${e.relation}-> ${e.to.type}/${e.to.id}`);
    if (shown.length < edges.length) out.push(`  ... ${edges.length - shown.length} more (pass --all)`);
  }

  return out.join('\n');
}

async function cmdShow(args: string[]): Promise<void> {
  const asJSON = args.includes('--json');
  const all = args.includes('--all');
  const path = args.find(a => !a.startsWith('--'));

  if (!path) {
    process.stderr.write('usage: akg-ts show [--json] [--all] PATH\n');
    process.exit(2);
  }

  // open() creates an empty store when the file is absent; for a read-only viewer a
  // missing path is a typo, so fail loudly instead of printing an empty graph.
  if (!existsSync(path)) {
    process.stderr.write(`cannot read ${path}: file does not exist\n`);
    process.exit(1);
  }

  const store = await open(path);
  const snap = store.snapshot();
  await store.close();

  if (asJSON) {
    console.log(JSON.stringify(snap, null, 2));
    return;
  }
  console.log(renderText(path, snap, all));
}

// ---- subcommand: overview ----

async function cmdOverview(): Promise<void> {
  const store = await loadStore();
  const nodes = store.listNodes();
  await store.close();

  const byType = new Map<string, Node[]>();
  for (const n of nodes) {
    if (!byType.has(n.type)) byType.set(n.type, []);
    byType.get(n.type)!.push(n);
  }

  const types = [...byType.keys()].sort();
  for (const type of types) {
    console.log(`\n## ${type}\n`);
    for (const n of byType.get(type)!) {
      console.log(`- **${n.title}** — ${n.body}`);
    }
  }
}

// ---- subcommand: explain <name> ----

async function cmdExplain(name: string): Promise<void> {
  const store = await loadStore();
  const nodes = store.listNodes();

  const match = nodes.find(
    n => n.title.toLowerCase() === name.toLowerCase(),
  );

  if (!match) {
    await store.close();
    process.stderr.write(`Not found: no node with title "${name}"\n`);
    process.exit(1);
  }

  const nodeRef = { type: match.type, id: match.id };
  const outEdges = store.outboundEdges(nodeRef);
  const inEdges = store.inboundEdges(nodeRef);
  await store.close();

  console.log(`# ${match.title}`);
  console.log(`\n**Type:** \`${match.type}\``);
  if (match.body) console.log(`\n${match.body}`);
  if (match.tags.length > 0) {
    console.log(`\n**Tags:** ${match.tags.map(t => `\`${t}\``).join(', ')}`);
  }

  const allEdges = [...outEdges, ...inEdges];
  if (allEdges.length > 0) {
    const byRelation = new Map<string, Array<{ dir: 'out' | 'in'; edge: Edge }>>();
    for (const e of outEdges) {
      if (!byRelation.has(e.relation)) byRelation.set(e.relation, []);
      byRelation.get(e.relation)!.push({ dir: 'out', edge: e });
    }
    for (const e of inEdges) {
      if (!byRelation.has(e.relation)) byRelation.set(e.relation, []);
      byRelation.get(e.relation)!.push({ dir: 'in', edge: e });
    }

    const relations = [...byRelation.keys()].sort();
    console.log('\n## Relations\n');
    for (const rel of relations) {
      console.log(`### ${rel}\n`);
      for (const { dir, edge } of byRelation.get(rel)!) {
        if (dir === 'out') {
          console.log(`- → **${edge.to.type}/${edge.to.id}**`);
        } else {
          console.log(`- ← **${edge.from.type}/${edge.from.id}**`);
        }
      }
    }
  }

  const meta = match.meta as Record<string, unknown>;
  if (meta.source_path && meta.anchor) {
    console.log(`\n**Source:** \`${meta.source_path}#${meta.anchor}\``);
  }
}

// ---- subcommand: search <query> ----

async function cmdSearch(query: string): Promise<void> {
  const store = await loadStore();
  const nodes = store.listNodes();
  await store.close();

  const q = query.toLowerCase();
  const matches = nodes.filter(n => {
    return (
      n.title.toLowerCase().includes(q) ||
      n.body.toLowerCase().includes(q) ||
      n.tags.some(t => t.toLowerCase().includes(q))
    );
  });

  if (matches.length === 0) {
    console.log(`No results for "${query}"`);
    return;
  }

  console.log(`## Search results for "${query}"\n`);
  for (const n of matches) {
    console.log(`- **${n.title}** (\`${n.type}\`) — ${n.body}`);
  }
}

// ---- subcommand: dump ----

async function cmdDump(format: 'markdown' | 'json'): Promise<void> {
  const store = await loadStore();
  if (format === 'json') {
    const snap = store.snapshot();
    await store.close();
    console.log(JSON.stringify(snap, null, 2));
    return;
  }

  const nodes = store.listNodes();
  await store.close();

  const byType = new Map<string, Node[]>();
  for (const n of nodes) {
    if (!byType.has(n.type)) byType.set(n.type, []);
    byType.get(n.type)!.push(n);
  }

  console.log('# akg-ts Documentation\n');
  const types = [...byType.keys()].sort();
  for (const type of types) {
    console.log(`\n## ${type}\n`);
    for (const n of byType.get(type)!) {
      console.log(nodeToMarkdown(n));
      console.log('');
    }
  }
}

// ---- main ----

async function main(): Promise<void> {
  const args = process.argv.slice(2);

  if (args[0] === 'show') {
    await cmdShow(args.slice(1));
    return;
  }

  if (args[0] === 'docs') {
    const sub = args[1];

    if (sub === 'overview') {
      await cmdOverview();
      return;
    }

    if (sub === 'explain') {
      const name = args[2];
      if (!name) {
        process.stderr.write('Usage: akg-ts docs explain <name>\n');
        process.exit(1);
      }
      await cmdExplain(name);
      return;
    }

    if (sub === 'search') {
      const query = args[2];
      if (!query) {
        process.stderr.write('Usage: akg-ts docs search <query>\n');
        process.exit(1);
      }
      await cmdSearch(query);
      return;
    }

    if (sub === 'dump') {
      const fmtFlag = args.indexOf('--format');
      const fmt = fmtFlag >= 0 ? args[fmtFlag + 1] : 'markdown';
      if (fmt !== 'markdown' && fmt !== 'json') {
        process.stderr.write('--format must be markdown or json\n');
        process.exit(1);
      }
      await cmdDump(fmt);
      return;
    }

    process.stderr.write(
      'Usage: akg-ts docs <overview|explain <name>|search <query>|dump [--format markdown|json]>\n',
    );
    process.exit(1);
  }

  process.stderr.write(
    'Usage: akg-ts <command>\n' +
      '  show <PATH> [--json] [--all]    render a .akg file as readable text\n' +
      '  docs <overview|explain <name>|search <query>|dump [--format markdown|json]>\n',
  );
  process.exit(1);
}

main().catch(err => {
  process.stderr.write(`Error: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
