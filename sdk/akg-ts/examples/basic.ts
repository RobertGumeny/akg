import { open } from '../src/index.js';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { rmSync, existsSync } from 'node:fs';

const path = join(tmpdir(), 'akg-ts-basic-example.akg');
if (existsSync(path)) rmSync(path);

// Open (or create) a store at the given path.
const store = await open(path);

// Write nodes. Tags let you group nodes by any label you choose.
const alice = store.putNode('person', 'alice', {
  title: 'Alice',
  body: 'A researcher in knowledge graphs.',
  meta: { role: 'lead' },
}, ['active', 'researcher']);

const bob = store.putNode('person', 'bob', {
  title: 'Bob',
  body: 'A software engineer.',
  meta: { role: 'engineer' },
}, ['active']);

const paper = store.putNode('paper', 'paper-001', {
  title: 'Graph-Based Context Compression',
  body: 'Explores AKG as an agent memory substrate.',
}, ['published']);

// Write edges connecting the nodes.
store.putEdge(alice, 'authored', paper, { strength: 1.0 });
store.putEdge(bob, 'reviewed', paper, { strength: 0.8 });
store.putEdge(alice, 'collaborates_with', bob, {});

// Commit and close. Reopen to show durability.
await store.close();

const store2 = await open(path);

// Read a single node by type and ID.
const node = store2.getNode('person', 'alice')!;
console.log(`Node: ${node.type}/${node.id} — "${node.title}"`);
console.log(`  body: ${node.body}`);
console.log(`  tags: [${node.tags.join(', ')}]`);
console.log(`  meta: ${JSON.stringify(node.meta)}`);

// List all nodes carrying a tag.
console.log('\nActive people:');
const actives = store2.listNodesByTag('active');
for (const n of actives) {
  console.log(`  ${n.type}/${n.id} — "${n.title}"`);
}

// Walk outbound edges from Alice.
console.log(`\nOutbound edges from ${alice.type}/${alice.id}:`);
const edges = store2.outboundEdges(alice);
for (const e of edges) {
  console.log(`  -[${e.relation}]-> ${e.to.type}/${e.to.id} (strength ${e.strength.toFixed(1)})`);
}

await store2.close();
