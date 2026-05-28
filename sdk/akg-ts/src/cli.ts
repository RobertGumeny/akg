import { open } from './store.js';
import { fileURLToPath } from 'node:url';
import { join, dirname } from 'node:path';
import type { Node, Edge } from './types.js';

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

  process.stderr.write('Usage: akg-ts docs <subcommand>\n');
  process.exit(1);
}

main().catch(err => {
  process.stderr.write(`Error: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
});
