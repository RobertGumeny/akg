// Generates testdata/roundtrip/ts-written.akg: a deterministic .akg file written
// by the akg-ts SDK, used by the cross-SDK round-trip test (sdk/akg-go/
// roundtrip_test.go) to prove a Go process can open a TS-written file and see
// identical graph state. Run with: npm run generate:roundtrip
//
// The graph here is the contract; if you change it, update the Go assertions to
// match. Output is committed to the repo.
import { mkdirSync, existsSync, unlinkSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { open, _setTestNow } from '../src/store.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, '..', '..', '..');
const outPath = join(repoRoot, 'testdata', 'roundtrip', 'ts-written.akg');

async function main(): Promise<void> {
  mkdirSync(dirname(outPath), { recursive: true });
  if (existsSync(outPath)) unlinkSync(outPath);

  // Pin the clock so the fixture is as reproducible as the SDK allows. (Byte
  // determinism is not guaranteed across SDKs — see PRD Open Question 1 — so the
  // Go test asserts semantic equality, not byte-identity.)
  _setTestNow(1_700_000_000_000_000n);

  const store = await open(outPath);

  const alice = store.putNode('person', 'alice', { title: 'Alice Researcher', body: 'Maintains the spec.' }, ['core', 'author']);
  const bob = store.putNode('person', 'bob', { title: 'Bob Engineer' }, ['core']);
  const akg = store.putNode('topic', 'akg', { title: 'AKG Format', body: 'The knowledge-graph file format.' }, []);

  // One edge with an explicit strength + confidence, one with confidence: null.
  store.putEdge(alice, 'authored', akg, { strength: 0.9, confidence: 0.95 });
  store.putEdge(bob, 'reviews', akg, { strength: 0.42, confidence: null });

  await store.commit();
  await store.close();

  _setTestNow(null);
  // eslint-disable-next-line no-console
  console.log(`wrote ${outPath}`);
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error(err);
  process.exit(1);
});
