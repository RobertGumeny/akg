/** A lightweight reference to a node by its (type, id). Safe to serialize and pass between systems. */
export interface NodeRef {
  type: string;
  id: string;
}

/** Caller-supplied fields for putNode. `title` is required; `body` and `meta` are optional. */
export interface NodeFields {
  title: string;
  body?: string;
  meta?: Record<string, unknown>;
}

/** A fully materialized node, including SDK-managed timestamps and version. */
export interface Node {
  type: string;
  id: string;
  title: string;
  body: string;
  meta: Record<string, unknown>;
  tags: string[];
  /** Unix timestamp in microseconds when the node was first written. */
  createdAt: number;
  /** Unix timestamp in microseconds of the most recent putNode. */
  updatedAt: number;
  /** Starts at 1, increments by 1 on each overwrite. */
  version: number;
}

/** Caller-supplied fields for putEdge. All optional; defaults are applied when omitted. */
export interface EdgeFields {
  /** Caller-defined weight; default 0.5 when omitted. */
  strength?: number;
  /** Caller-defined certainty; default null (asserted directly, no value recorded). */
  confidence?: number | null;
  meta?: Record<string, unknown>;
}

/** A fully materialized edge between two nodes, including SDK-managed timestamps and version. */
export interface Edge {
  from: NodeRef;
  relation: string;
  to: NodeRef;
  /** Caller-defined weight; default 0.5. */
  strength: number;
  /** Caller-defined certainty, or null if none recorded. */
  confidence: number | null;
  meta: Record<string, unknown>;
  /** Unix timestamp in microseconds when the edge was first written. */
  createdAt: number;
  /** Unix timestamp in microseconds of the most recent putEdge. */
  updatedAt: number;
  /** Starts at 1, increments by 1 on each overwrite. */
  version: number;
}

// --- Filtering types ---

/** Filter for listEdges. Non-empty fields combine with AND semantics. */
export interface EdgeFilter {
  relation?: string;
  meta?: Record<string, unknown>;
}

/** Filter for listNodesFiltered. Non-empty fields combine with AND semantics. */
export interface NodeFilter {
  type?: string;
  tag?: string;
  meta?: Record<string, unknown>;
}

/** A point-in-time view of all live nodes and edges; JSON-serializable. */
export interface Snapshot {
  nodes: Node[];
  edges: Edge[];
}

// --- Recency filter types ---

/** Filter for recentNodes. Non-empty fields combine with AND semantics. */
export interface RecencyFilter {
  type?: string;
  tag?: string;
  /** inclusive: updatedAt >= sinceUpdatedAt */
  sinceUpdatedAt?: number;
  /** inclusive: updatedAt <= untilUpdatedAt */
  untilUpdatedAt?: number;
  /** 0 or omitted = unlimited; negative = invalid */
  limit?: number;
}

/** Filter for recentEdges. Non-empty fields combine with AND semantics. */
export interface EdgeRecencyFilter {
  relation?: string;
  from?: NodeRef;
  to?: NodeRef;
  /** inclusive: updatedAt >= sinceUpdatedAt */
  sinceUpdatedAt?: number;
  /** inclusive: updatedAt <= untilUpdatedAt */
  untilUpdatedAt?: number;
  /** 0 or omitted = unlimited; negative = invalid */
  limit?: number;
}

// --- Reconciliation result ---

/** Counts returned by reconcileOutboundEdges: edges added, removed, and left unchanged. */
export interface ReconcileResult {
  added: number;
  removed: number;
  unchanged: number;
}

// --- Cascade delete result ---

/** Counts returned by deleteNodeCascade: inbound/outbound edges removed and whether the node was deleted. */
export interface CascadeDeleteResult {
  deletedInboundEdges: number;
  deletedOutboundEdges: number;
  deletedNode: boolean;
}
