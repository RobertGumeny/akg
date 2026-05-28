export interface NodeRef {
  type: string;
  id: string;
}

export interface NodeFields {
  title: string;
  body?: string;
  meta?: Record<string, unknown>;
}

export interface Node {
  type: string;
  id: string;
  title: string;
  body: string;
  meta: Record<string, unknown>;
  tags: string[];
  createdAt: number;
  updatedAt: number;
  version: number;
}

export interface EdgeFields {
  strength?: number;
  confidence?: number | null;
  meta?: Record<string, unknown>;
}

export interface Edge {
  from: NodeRef;
  relation: string;
  to: NodeRef;
  strength: number;
  confidence: number | null;
  meta: Record<string, unknown>;
  createdAt: number;
  updatedAt: number;
  version: number;
}

// --- Filtering types ---

export interface EdgeFilter {
  relation?: string;
  meta?: Record<string, unknown>;
}

export interface NodeFilter {
  type?: string;
  tag?: string;
  meta?: Record<string, unknown>;
}

export interface Snapshot {
  nodes: Node[];
  edges: Edge[];
}

// --- Recency filter types ---

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

export interface ReconcileResult {
  added: number;
  removed: number;
  unchanged: number;
}

// --- Cascade delete result ---

export interface CascadeDeleteResult {
  deletedInboundEdges: number;
  deletedOutboundEdges: number;
  deletedNode: boolean;
}
