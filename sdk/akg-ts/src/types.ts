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
