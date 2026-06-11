import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { Store } from '../src/store.js';
import { NotFoundError, InvalidInputError, MissingRequiredFieldError } from '../src/errors.js';

const CONFORMANCE_DIR = join(import.meta.dirname ?? __dirname, '../../../testdata/conformance');
const MANIFEST_PATH = join(CONFORMANCE_DIR, 'manifest.json');

interface StoreExpectation {
  nodes: number;
  edges: number;
  has_uncompacted_wal: boolean;
  next_wal_sequence: number;
  absent_node?: { type: string; id: string };
}

interface Fixture {
  path: string;
  purpose: string;
  expected_result: 'accept' | 'reject';
  validation_scope: 'format' | 'store';
  store_expectation?: StoreExpectation;
  expected_error_category?: string;
  sha256?: string;
  features?: string[];
}

interface Manifest {
  version: number;
  fixtures: Fixture[];
}

function errorCategory(err: unknown): string {
  if (!(err instanceof Error)) return 'unknown';
  const msg = err.message.toLowerCase();

  if (msg.includes('invalid header') && (msg.includes('magic') || msg.includes('too short') || msg.includes('version') || msg.includes('reserved') || msg.includes('checksum algorithm'))) {
    return 'invalid_header';
  }
  if (msg.includes('wal checksum mismatch')) return 'wal_checksum_mismatch';
  if (msg.includes('checksum mismatch')) return 'checksum_mismatch';
  if (msg.includes('invalid section table') || msg.includes('wrong section counts') || msg.includes('section too short')) return 'invalid_section_table';
  if (msg.includes('invalid section ranges') || msg.includes('sections overlap') || msg.includes('out of bounds')) return 'invalid_section_ranges';
  if (msg.includes('invalid bloom section')) return 'invalid_bloom_section';
  if (msg.includes('unknown wal operation')) return 'unknown_wal_operation';
  if (msg.includes('wal checksum mismatch')) return 'wal_checksum_mismatch';
  if (msg.includes('invalid wal record') || msg.includes('non-increasing sequence')) return 'invalid_wal_record';
  if (msg.includes('invalid wal payload')) return 'invalid_wal_payload';
  if (msg.includes('invalid data payload')) return 'invalid_data_payload';
  if (msg.includes('derived index mismatch')) return 'derived_index_mismatch';
  if (msg.includes('malformed') && msg.includes('key')) return 'malformed_key';
  return 'unknown';
}

const manifest: Manifest = JSON.parse(readFileSync(MANIFEST_PATH, 'utf-8'));

describe('conformance', () => {
  for (const fixture of manifest.fixtures) {
    it(fixture.path + ' — ' + fixture.purpose, () => {
      const filePath = join(CONFORMANCE_DIR, fixture.path);
      const buf = readFileSync(filePath);
      const bytes = new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);

      if (fixture.expected_result === 'accept') {
        let store: ReturnType<typeof Store.fromBytes>;
        expect(() => {
          store = Store.fromBytes(bytes, filePath);
        }, `fixture ${fixture.path} should open without error`).not.toThrow();

        if (fixture.store_expectation && fixture.validation_scope === 'store') {
          const exp = fixture.store_expectation;
          const nodes = store!.listNodes();
          const edges: unknown[] = [];
          for (const node of nodes) {
            const out = store!.outboundEdges({ type: node.type, id: node.id });
            edges.push(...out);
          }

          expect(nodes.length, `node count for ${fixture.path}`).toBe(exp.nodes);

          const edgeCount = countEdges(store!);
          expect(edgeCount, `edge count for ${fixture.path}`).toBe(exp.edges);

          expect(store!.hasUncompactedWAL, `has_uncompacted_wal for ${fixture.path}`).toBe(exp.has_uncompacted_wal);
          expect(Number(store!.nextWALSequence), `next_wal_sequence for ${fixture.path}`).toBe(exp.next_wal_sequence);

          if (exp.absent_node) {
            const n = store!.getNode(exp.absent_node.type, exp.absent_node.id);
            expect(n, `absent node in ${fixture.path}`).toBeNull();
          }
        }
      } else {
        let caughtError: unknown;
        try {
          Store.fromBytes(bytes, filePath);
        } catch (e) {
          caughtError = e;
        }
        expect(caughtError, `fixture ${fixture.path} should throw`).toBeDefined();

        if (fixture.expected_error_category) {
          const category = errorCategory(caughtError);
          expect(category, `error category for ${fixture.path}: ${caughtError instanceof Error ? caughtError.message : String(caughtError)}`).toBe(fixture.expected_error_category);
        }
      }
    });
  }
});

function countEdges(store: ReturnType<typeof Store.fromBytes>): number {
  const nodes = store.listNodes();
  const seen = new Set<string>();
  for (const node of nodes) {
    const edges = store.outboundEdges({ type: node.type, id: node.id });
    for (const e of edges) {
      const key = `${e.from.type}:${e.from.id}:${e.relation}:${e.to.type}:${e.to.id}`;
      seen.add(key);
    }
  }
  return seen.size;
}
