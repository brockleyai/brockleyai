import dagre from "@dagrejs/dagre";

interface LayoutNode {
  id: string;
  position?: { x: number; y: number };
}

interface LayoutEdge {
  id: string;
  source_node_id: string;
  target_node_id: string;
  back_edge?: boolean;
}

const NODE_WIDTH = 200;
const NODE_HEIGHT = 80;

export function autoLayout(
  nodes: LayoutNode[],
  edges: LayoutEdge[],
): Record<string, { x: number; y: number }> {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "LR", nodesep: 60, ranksep: 120 });

  for (const node of nodes) {
    g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  }

  // Filter out back-edges to avoid cycles in dagre
  for (const edge of edges) {
    if (!edge.back_edge) {
      g.setEdge(edge.source_node_id, edge.target_node_id);
    }
  }

  dagre.layout(g);

  const positions: Record<string, { x: number; y: number }> = {};
  for (const node of nodes) {
    const pos = g.node(node.id);
    if (pos) {
      // dagre returns center coordinates; convert to top-left for React Flow
      positions[node.id] = {
        x: pos.x - NODE_WIDTH / 2,
        y: pos.y - NODE_HEIGHT / 2,
      };
    }
  }

  return positions;
}
