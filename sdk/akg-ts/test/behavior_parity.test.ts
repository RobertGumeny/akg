// Cross-SDK behavioral parity tests.
// Loads testdata/behavior/parity-graph.akg and asserts against
// testdata/behavior/parity-spec.json — the same spec the Go SDK uses.
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { Store } from '../src/store.js';

const root = resolve(__dirname, '../../../testdata/behavior');
const spec = JSON.parse(readFileSync(join(root, 'parity-spec.json'), 'utf8'));
const a = spec.assertions;

function openFixture(): Store {
  const bytes = readFileSync(join(root, 'parity-graph.akg'));
  return Store.fromBytes(new Uint8Array(bytes.buffer, bytes.byteOffset, bytes.byteLength), join(root, 'parity-graph.akg'));
}

describe('behavioral parity (parity-graph.akg)', () => {
  it('node and edge counts', () => {
    const s = openFixture();
    expect(s.listNodes()).toHaveLength(spec.node_count);
    expect(s.listEdges()).toHaveLength(spec.edge_count);
  });

  it('listEdges: empty filter returns all edges', () => {
    const s = openFixture();
    expect(s.listEdges()).toHaveLength(a.list_edges_all_count);
  });

  it('listEdges: relation filters', () => {
    const s = openFixture();
    expect(s.listEdges({ relation: 'knows' })).toHaveLength(a.list_edges_knows_count);
    expect(s.listEdges({ relation: 'assigned' })).toHaveLength(a.list_edges_assigned_count);
    expect(s.listEdges({ relation: 'manages' })).toHaveLength(a.list_edges_manages_count);
  });

  it('listEdges: metadata filters', () => {
    const s = openFixture();
    expect(s.listEdges({ meta: { source: 'inferred' } })).toHaveLength(a.list_edges_meta_source_inferred_count);
    expect(s.listEdges({ meta: { source: 'manual' } })).toHaveLength(a.list_edges_meta_source_manual_count);
  });

  it('edge strength: explicit zero round-trips as 0.0, not 0.5', () => {
    const s = openFixture();
    const edges = s.outboundEdges({ type: 'person', id: 'bob' }, 'knows');
    expect(edges[0].strength).toBe(a.bob_knows_alice_strength);
  });

  it('edge strength: omitted strength defaults to 0.5', () => {
    const s = openFixture();
    const edges = s.outboundEdges({ type: 'person', id: 'alice' }, 'assigned');
    expect(edges[0].strength).toBe(a.alice_assigned_t1_strength);
  });

  it('edge strength: explicit non-default value round-trips unchanged', () => {
    const s = openFixture();
    const edges = s.outboundEdges({ type: 'person', id: 'alice' }, 'knows');
    const e = edges.find(e => e.to.id === 'bob')!;
    expect(e.strength).toBe(a.alice_knows_bob_strength);
  });

  it('listNodesFiltered: type + tag (AND semantics)', () => {
    const s = openFixture();
    const nodes = s.listNodesFiltered({ type: 'person', tag: 'active' });
    expect(nodes.map(n => n.id)).toEqual(a.list_nodes_type_person_tag_active_ids);
  });

  it('listNodesFiltered: type + specific tag', () => {
    const s = openFixture();
    const nodes = s.listNodesFiltered({ type: 'person', tag: 'researcher' });
    expect(nodes.map(n => n.id)).toEqual(a.list_nodes_type_person_tag_researcher_ids);
  });

  it('listNodesFiltered: metadata scalar equality', () => {
    const s = openFixture();
    const nodes = s.listNodesFiltered({ meta: { role: 'engineer' } });
    expect(nodes.map(n => n.id)).toEqual(a.list_nodes_meta_role_engineer_ids);
  });

  it('listNodesFiltered: metadata array equality', () => {
    const s = openFixture();
    const nodes = s.listNodesFiltered({ meta: { tags: ['urgent', 'p1'] } });
    expect(nodes.map(n => n.id)).toEqual(a.list_nodes_meta_tags_array_ids);
  });

  it('snapshot: correct node and edge counts', () => {
    const s = openFixture();
    const snap = s.snapshot();
    expect(snap.nodes).toHaveLength(a.snapshot_node_count);
    expect(snap.edges).toHaveLength(a.snapshot_edge_count);
  });

  it('tag-index key collision: same id across types stays distinct', () => {
    const s = openFixture();
    // Two nodes share id "preflop__vpip" across types (counter, tendency), both
    // tagged "preflop". The major-2 type-qualified tag key keeps them distinct.
    const byTag = s.listNodesFiltered({ tag: 'preflop' });
    expect(byTag).toHaveLength(a.collision_tag_preflop_count);

    const refs = a.collision_get_nodes_input as Array<{ type: string; id: string }>;
    const results = s.getNodes(refs);
    const expectedTitles = a.collision_get_nodes_titles as string[];
    for (let i = 0; i < expectedTitles.length; i++) {
      expect(results[i]?.title).toBe(expectedTitles[i]);
    }
  });

  it('getNodes: preserves input order and returns null for missing refs', () => {
    const s = openFixture();
    const refs = (a.get_nodes_input as Array<{ type: string; id: string }>);
    const results = s.getNodes(refs);
    const expectedTitles = a.get_nodes_titles as Array<string | null>;
    for (let i = 0; i < expectedTitles.length; i++) {
      if (expectedTitles[i] === null) {
        expect(results[i]).toBeNull();
      } else {
        expect(results[i]?.title).toBe(expectedTitles[i]);
      }
    }
  });
});
