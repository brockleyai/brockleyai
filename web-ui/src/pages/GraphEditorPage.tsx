import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  ReactFlowProvider,
  addEdge,
  useNodesState,
  useEdgesState,
  useReactFlow,
  type Node as FlowNode,
  type Edge as FlowEdge,
  type Connection,
  type NodeTypes,
  type EdgeTypes,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { useAppStore, type Graph, type GraphNode, type GraphEdge, type StateField, type ValidationResult, type Execution } from "../store";
import { getGraph, updateGraph, validateGraph, invokeGraph, getExecution, getExecutionSteps } from "../api";
import NodeCard, { type NodeCardData } from "../components/editor/NodeCard";
import BrockleyEdge from "../components/editor/BrockleyEdge";
import NodePalette from "../components/editor/NodePalette";
import PropertiesPanel from "../components/editor/PropertiesPanel";
import StatusBar from "../components/editor/StatusBar";
import ExecuteDialog from "../components/editor/ExecuteDialog";
import { useToast } from "../components/Toast";
import { autoLayout } from "../components/editor/autoLayout";

const nodeTypes: NodeTypes = { nodeCard: NodeCard as unknown as NodeTypes[string] };
const edgeTypes: EdgeTypes = { brockley: BrockleyEdge as unknown as EdgeTypes[string] };

let nodeCounter = 0;

export default function GraphEditorPage() {
  return (
    <ReactFlowProvider>
      <GraphEditorInner />
    </ReactFlowProvider>
  );
}

function GraphEditorInner() {
  const { serverUrl, apiKey, currentGraphId, navigate } = useAppStore();
  const { screenToFlowPosition, fitView } = useReactFlow();
  const { showToast } = useToast();

  // ─── Core State ───
  const [graph, setGraph] = useState<Graph | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [lastSaved, setLastSaved] = useState<string | null>(null);

  // ─── Selection ───
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);
  const [selectedStateFieldIndex, setSelectedStateFieldIndex] = useState<number | null>(null);

  // ─── Validation ───
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
  const [nodeErrors, setNodeErrors] = useState<Record<string, string>>({});

  // ─── Execution ───
  const [showExecuteDialog, setShowExecuteDialog] = useState(false);
  const [executionId, setExecutionId] = useState<string | null>(null);
  const [nodeStatuses, setNodeStatuses] = useState<Record<string, string>>({});
  const executionPollRef = useRef<ReturnType<typeof setInterval>>();

  // ─── JSON View ───
  const [showJson, setShowJson] = useState(false);
  const [jsonText, setJsonText] = useState("");

  // ─── Editing name ───
  const [isEditingName, setIsEditingName] = useState(false);
  const [editingName, setEditingName] = useState("");

  // ─── React Flow State ───
  const [flowNodes, setFlowNodes, onNodesChange] = useNodesState([]);
  const [flowEdges, setFlowEdges, onEdgesChange] = useEdgesState([]);

  // ─── Load Graph ───
  useEffect(() => {
    if (!currentGraphId) return;
    setLoading(true);
    getGraph(serverUrl, apiKey, currentGraphId)
      .then((g) => {
        setGraph(g);
        setJsonText(JSON.stringify(g, null, 2));
        syncFlowFromGraph(g);
        setLastSaved(new Date().toISOString());
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load graph"))
      .finally(() => setLoading(false));
  }, [serverUrl, apiKey, currentGraphId]);

  // ─── Convert Graph → Flow ───
  const syncFlowFromGraph = useCallback((g: Graph) => {
    const nodes: FlowNode[] = (g.nodes || []).map((n, i) => ({
      id: n.id,
      type: "nodeCard",
      position: n.position || { x: 100 + (i % 4) * 250, y: 100 + Math.floor(i / 4) * 150 },
      data: {
        label: n.name || n.id,
        nodeType: n.type,
        outputLabel: getOutputLabel(n),
        hasError: !!nodeErrors[n.id],
        errorMessage: nodeErrors[n.id],
        isRunning: nodeStatuses[n.id] === "running",
        isCompleted: nodeStatuses[n.id] === "completed",
        isFailed: nodeStatuses[n.id] === "failed",
        isSkipped: nodeStatuses[n.id] === "skipped",
      } satisfies NodeCardData,
    }));

    const edges: FlowEdge[] = (g.edges || []).map((e) => ({
      id: e.id,
      source: e.source_node_id,
      target: e.target_node_id,
      type: "brockley",
      data: {
        backEdge: e.back_edge,
        label: e.source_port !== e.target_port ? `${e.source_port} → ${e.target_port}` : "",
        onDelete: () => handleDeleteEdge(e.id),
      },
    }));

    setFlowNodes(nodes);
    setFlowEdges(edges);
  }, [nodeErrors, nodeStatuses]);

  // ─── Keep flow in sync when statuses change ───
  useEffect(() => {
    if (!graph) return;
    setFlowNodes((nds) =>
      nds.map((n) => ({
        ...n,
        data: {
          ...n.data,
          hasError: !!nodeErrors[n.id],
          errorMessage: nodeErrors[n.id],
          isRunning: nodeStatuses[n.id] === "running",
          isCompleted: nodeStatuses[n.id] === "completed",
          isFailed: nodeStatuses[n.id] === "failed",
          isSkipped: nodeStatuses[n.id] === "skipped",
        },
      }))
    );
  }, [nodeErrors, nodeStatuses, graph]);

  function getOutputLabel(n: GraphNode): string {
    if (n.type === "llm") {
      const fmt = (n.config as Record<string, unknown>)?.response_format;
      return fmt === "json" ? "json" : "text";
    }
    return "";
  }

  // ─── Convert Flow → Graph (for saving) ───
  const buildGraphPayload = useCallback((): Record<string, unknown> => {
    if (!graph) return {};

    const updatedNodes = graph.nodes.map((n) => {
      const flowNode = flowNodes.find((fn) => fn.id === n.id);
      return {
        ...n,
        position: flowNode?.position || n.position,
      };
    });

    return {
      name: graph.name,
      description: graph.description,
      status: graph.status,
      nodes: updatedNodes,
      edges: graph.edges,
      state: graph.state,
    };
  }, [graph, flowNodes]);

  // ─── Mark dirty on any change ───
  const markDirty = useCallback(() => setIsDirty(true), []);

  // ─── Node CRUD ───
  const handleUpdateNode = useCallback((nodeId: string, updates: Partial<GraphNode>) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      const nodes = prev.nodes.map((n) =>
        n.id === nodeId ? { ...n, ...updates, config: updates.config !== undefined ? { ...n.config, ...updates.config } : n.config } : n
      );
      return { ...prev, nodes };
    });
    // Update flow node label if name changed
    if (updates.name) {
      setFlowNodes((nds) =>
        nds.map((n) => n.id === nodeId ? { ...n, data: { ...n.data, label: updates.name } } : n)
      );
    }
    markDirty();
  }, [graph, markDirty]);

  const handleDeleteNode = useCallback((nodeId: string) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        nodes: prev.nodes.filter((n) => n.id !== nodeId),
        edges: prev.edges.filter((e) => e.source_node_id !== nodeId && e.target_node_id !== nodeId),
      };
    });
    setFlowNodes((nds) => nds.filter((n) => n.id !== nodeId));
    setFlowEdges((eds) => eds.filter((e) => e.source !== nodeId && e.target !== nodeId));
    setSelectedNodeId(null);
    markDirty();
  }, [graph, markDirty]);

  // ─── Edge CRUD ───
  const handleDeleteEdge = useCallback((edgeId: string) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      return { ...prev, edges: prev.edges.filter((e) => e.id !== edgeId) };
    });
    setFlowEdges((eds) => eds.filter((e) => e.id !== edgeId));
    setSelectedEdgeId(null);
    markDirty();
  }, [graph, markDirty]);

  const handleUpdateEdge = useCallback((edgeId: string, updates: Partial<GraphEdge>) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        edges: prev.edges.map((e) => e.id === edgeId ? { ...e, ...updates } : e),
      };
    });
    markDirty();
  }, [graph, markDirty]);

  const onConnect = useCallback((connection: Connection) => {
    if (!graph || !connection.source || !connection.target) return;
    const edgeId = `edge_${Date.now()}`;
    const newEdge: GraphEdge = {
      id: edgeId,
      source_node_id: connection.source,
      source_port: "output",
      target_node_id: connection.target,
      target_port: "input",
    };
    setGraph((prev) => {
      if (!prev) return prev;
      return { ...prev, edges: [...prev.edges, newEdge] };
    });
    setFlowEdges((eds) =>
      addEdge(
        {
          ...connection,
          id: edgeId,
          type: "brockley",
          data: { onDelete: () => handleDeleteEdge(edgeId) },
        },
        eds
      )
    );
    markDirty();
  }, [graph, markDirty, handleDeleteEdge]);

  // ─── Drag and Drop (from palette) ───
  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  }, []);

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      if (!graph) return;

      const type = e.dataTransfer.getData("application/reactflow-type");
      const label = e.dataTransfer.getData("application/reactflow-label");
      if (!type) return;

      const position = screenToFlowPosition({ x: e.clientX, y: e.clientY });
      nodeCounter++;
      const id = `${type}-${nodeCounter}-${Date.now().toString(36)}`;
      const name = `${label} ${nodeCounter}`;

      const newNode: GraphNode = {
        id,
        type,
        name,
        config: {},
        input_ports: [{ name: "input", schema: { type: "object" } }],
        output_ports: [{ name: "output", schema: { type: "object" } }],
        position: { x: Math.round(position.x), y: Math.round(position.y) },
      };

      setGraph((prev) => {
        if (!prev) return prev;
        return { ...prev, nodes: [...prev.nodes, newNode] };
      });

      setFlowNodes((nds) => [
        ...nds,
        {
          id,
          type: "nodeCard",
          position: newNode.position!,
          data: { label: name, nodeType: type } satisfies NodeCardData,
        },
      ]);
      markDirty();
    },
    [graph, screenToFlowPosition, markDirty]
  );

  // ─── Node drag → update position + dirty ───
  const onNodeDragStop = useCallback(
    (_: unknown, node: FlowNode) => {
      if (!graph) return;
      setGraph((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          nodes: prev.nodes.map((n) =>
            n.id === node.id ? { ...n, position: { x: Math.round(node.position.x), y: Math.round(node.position.y) } } : n
          ),
        };
      });
      markDirty();
    },
    [graph, markDirty]
  );

  // ─── Selection (mutual exclusion) ───
  const onNodeClick = useCallback((_: unknown, node: FlowNode) => {
    setSelectedNodeId(node.id);
    setSelectedEdgeId(null);
    setSelectedStateFieldIndex(null);
  }, []);

  const onEdgeClick = useCallback((_: unknown, edge: FlowEdge) => {
    setSelectedEdgeId(edge.id);
    setSelectedNodeId(null);
    setSelectedStateFieldIndex(null);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(null);
    setSelectedEdgeId(null);
    setSelectedStateFieldIndex(null);
  }, []);

  const handleSelectStateField = useCallback((index: number) => {
    setSelectedStateFieldIndex(index);
    setSelectedNodeId(null);
    setSelectedEdgeId(null);
  }, []);

  const handleSelectNode = useCallback((nodeId: string) => {
    setSelectedNodeId(nodeId);
    setSelectedEdgeId(null);
    setSelectedStateFieldIndex(null);
  }, []);

  // ─── State Field CRUD ───
  const handleAddStateField = useCallback(() => {
    if (!graph) return;
    const newField: StateField = {
      name: "",
      schema: { type: "string" },
      reducer: "replace",
    };
    setGraph((prev) => {
      if (!prev) return prev;
      const fields = [...(prev.state?.fields ?? []), newField];
      return { ...prev, state: { fields } };
    });
    const newIndex = graph.state?.fields?.length ?? 0;
    setSelectedStateFieldIndex(newIndex);
    setSelectedNodeId(null);
    setSelectedEdgeId(null);
    markDirty();
  }, [graph, markDirty]);

  const handleUpdateStateField = useCallback((index: number, field: StateField) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      const oldFields = prev.state?.fields ?? [];
      const oldName = oldFields[index]?.name;
      const newName = field.name;
      const fields = oldFields.map((f, i) => (i === index ? field : f));

      // Rename propagation: update all bindings referencing the old name
      let nodes = prev.nodes;
      if (oldName && newName && oldName !== newName) {
        nodes = prev.nodes.map((n) => {
          const reads = n.state_reads?.map((b) =>
            b.state_field === oldName ? { ...b, state_field: newName } : b
          );
          const writes = n.state_writes?.map((b) =>
            b.state_field === oldName ? { ...b, state_field: newName } : b
          );
          return { ...n, state_reads: reads, state_writes: writes };
        });
      }

      return { ...prev, state: { fields }, nodes };
    });
    markDirty();
  }, [graph, markDirty]);

  const handleDeleteStateField = useCallback((index: number) => {
    if (!graph) return;
    setGraph((prev) => {
      if (!prev) return prev;
      const oldFields = prev.state?.fields ?? [];
      const deletedName = oldFields[index]?.name;
      const fields = oldFields.filter((_, i) => i !== index);

      // Clean up all bindings referencing the deleted field
      const nodes = prev.nodes.map((n) => ({
        ...n,
        state_reads: n.state_reads?.filter((b) => b.state_field !== deletedName),
        state_writes: n.state_writes?.filter((b) => b.state_field !== deletedName),
      }));

      return { ...prev, state: { fields }, nodes };
    });
    if (selectedStateFieldIndex === index) {
      setSelectedStateFieldIndex(null);
    } else if (selectedStateFieldIndex !== null && selectedStateFieldIndex > index) {
      setSelectedStateFieldIndex(selectedStateFieldIndex - 1);
    }
    markDirty();
  }, [graph, markDirty, selectedStateFieldIndex]);

  // ─── Save ───
  const handleSave = useCallback(async () => {
    if (!graph || !currentGraphId) return;
    setSaving(true);
    try {
      const payload = buildGraphPayload();
      const updated = await updateGraph(serverUrl, apiKey, currentGraphId, payload);
      setGraph(updated);
      setJsonText(JSON.stringify(updated, null, 2));
      setIsDirty(false);
      setLastSaved(new Date().toISOString());
      showToast("Saved", "success");
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to save";
      setError(msg);
      showToast(msg, "error");
    } finally {
      setSaving(false);
    }
  }, [graph, currentGraphId, serverUrl, apiKey, buildGraphPayload, showToast]);

  // ─── Status Toggle ───
  const handleStatusChange = useCallback(async (newStatus: string) => {
    if (!graph || !currentGraphId) return;
    try {
      const payload = { ...buildGraphPayload(), status: newStatus };
      const updated = await updateGraph(serverUrl, apiKey, currentGraphId, payload);
      setGraph(updated);
      setJsonText(JSON.stringify(updated, null, 2));
      setIsDirty(false);
      setLastSaved(new Date().toISOString());
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update status");
    }
  }, [graph, currentGraphId, serverUrl, apiKey, buildGraphPayload]);

  // ─── Validate ───
  const handleValidate = useCallback(async () => {
    if (!currentGraphId) return;
    // Save first if dirty
    if (isDirty) await handleSave();
    try {
      const result = await validateGraph(serverUrl, apiKey, currentGraphId) as unknown as ValidationResult;
      setValidationResult(result);
      // Map errors to nodes
      const errors: Record<string, string> = {};
      if (result.errors) {
        for (const err of result.errors) {
          if (err.node_id) {
            errors[err.node_id] = err.message;
          }
        }
      }
      setNodeErrors(errors);
      if (result.valid) {
        showToast("Graph is valid", "success");
      } else {
        showToast(`${(result.errors || []).length} validation issue(s)`, "error");
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Validation failed";
      setError(msg);
      showToast(msg, "error");
    }
  }, [currentGraphId, isDirty, handleSave, serverUrl, apiKey, showToast]);

  // ─── Execute ───
  const handleExecute = useCallback(async (input: Record<string, unknown>, mode: string) => {
    if (!currentGraphId) return;
    // Save first if dirty
    if (isDirty) await handleSave();
    try {
      const exec = await invokeGraph(serverUrl, apiKey, currentGraphId, input, mode as "sync" | "async");
      setExecutionId(exec.id);
      setShowExecuteDialog(false);
      setNodeStatuses({});
      // Start polling
      if (executionPollRef.current) clearInterval(executionPollRef.current);
      executionPollRef.current = setInterval(async () => {
        try {
          const execution = await getExecution(serverUrl, apiKey, exec.id);
          if (execution.status === "completed" || execution.status === "failed" || execution.status === "cancelled") {
            clearInterval(executionPollRef.current);
          }
          // Fetch steps
          const steps = await getExecutionSteps(serverUrl, apiKey, exec.id);
          const statuses: Record<string, string> = {};
          for (const step of steps) {
            statuses[step.node_id] = step.status;
          }
          setNodeStatuses(statuses);
        } catch {
          clearInterval(executionPollRef.current);
        }
      }, 1000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Execution failed");
    }
  }, [currentGraphId, isDirty, handleSave, serverUrl, apiKey]);

  // Execute that returns the final result (for the dialog)
  const handleExecuteWithResult = useCallback(async (input: Record<string, unknown>, mode: string): Promise<Execution | null> => {
    if (!currentGraphId) return null;
    if (isDirty) await handleSave();
    try {
      const exec = await invokeGraph(serverUrl, apiKey, currentGraphId, input, mode as "sync" | "async");

      // If sync, the server returns the completed execution directly
      if (mode === "sync" && exec.status === "completed") {
        return exec;
      }
      if (mode === "sync" && exec.status === "failed") {
        return exec;
      }

      // For async or if sync returned running, poll until done
      return new Promise<Execution>((resolve) => {
        const poll = setInterval(async () => {
          try {
            const updated = await getExecution(serverUrl, apiKey, exec.id);
            // Also update node statuses on canvas
            const steps = await getExecutionSteps(serverUrl, apiKey, exec.id);
            const statuses: Record<string, string> = {};
            for (const step of steps) {
              statuses[step.node_id] = step.status;
            }
            setNodeStatuses(statuses);

            if (updated.status === "completed" || updated.status === "failed" || updated.status === "cancelled") {
              clearInterval(poll);
              resolve(updated);
            }
          } catch {
            clearInterval(poll);
            resolve(exec);
          }
        }, 1000);
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Execution failed");
      return null;
    }
  }, [currentGraphId, isDirty, handleSave, serverUrl, apiKey]);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (executionPollRef.current) clearInterval(executionPollRef.current);
    };
  }, []);

  // ─── Graph name editing ───
  const startEditingName = useCallback(() => {
    if (!graph) return;
    setEditingName(graph.name);
    setIsEditingName(true);
  }, [graph]);

  const commitName = useCallback(() => {
    if (!graph || !editingName.trim()) return;
    setGraph((prev) => prev ? { ...prev, name: editingName.trim() } : prev);
    setIsEditingName(false);
    markDirty();
  }, [editingName, graph, markDirty]);

  // ─── JSON editing ───
  const handleJsonChange = useCallback((text: string) => {
    setJsonText(text);
  }, []);

  // ─── Auto-layout ───
  const handleAutoLayout = useCallback(() => {
    if (!graph || graph.nodes.length < 2) return;
    const positions = autoLayout(graph.nodes, graph.edges);
    setGraph((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        nodes: prev.nodes.map((n) => ({
          ...n,
          position: positions[n.id] || n.position,
        })),
      };
    });
    setFlowNodes((nds) =>
      nds.map((n) => ({
        ...n,
        position: positions[n.id] || n.position,
      })),
    );
    markDirty();
    setTimeout(() => fitView({ padding: 0.15, duration: 300 }), 50);
  }, [graph, markDirty, fitView]);

  // ─── Keyboard shortcuts ───
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        handleSave();
        return;
      }
      if (e.key === "Delete" || e.key === "Backspace") {
        const tag = (e.target as HTMLElement).tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || (e.target as HTMLElement).isContentEditable) return;
        e.preventDefault();
        if (selectedNodeId) handleDeleteNode(selectedNodeId);
        else if (selectedEdgeId) handleDeleteEdge(selectedEdgeId);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [handleSave, selectedNodeId, selectedEdgeId, handleDeleteNode, handleDeleteEdge]);

  // ─── Warn on navigate away ───
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [isDirty]);

  // ─── Minimap colors ───
  const minimapNodeColor = useCallback((node: FlowNode) => {
    const type = (node.data as NodeCardData)?.nodeType;
    const colors: Record<string, string> = {
      llm: "#818cf8", tool: "#34d399", conditional: "#fbbf24",
      transform: "#22d3ee", input: "#22d3ee", output: "#f43f5e",
      superagent: "#ec4899",
    };
    return colors[type] || "#6b7280";
  }, []);

  // ─── Render ───
  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-[var(--text-secondary)]">
        Loading graph...
      </div>
    );
  }

  if (error && !graph) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-red-400">{error}</p>
        <button onClick={() => navigate("graphs")} className="text-sm text-brand-400 hover:underline">
          Back to graphs
        </button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* ═══ Toolbar ═══ */}
      <div className="flex h-12 items-center justify-between border-b border-[var(--border-primary)] bg-[var(--bg-surface)] px-4">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate("graphs")}
            className="text-sm text-[var(--text-secondary)] hover:text-white"
          >
            &larr; Graphs
          </button>
          <span className="text-[var(--border-primary)]">/</span>
          {isEditingName ? (
            <input
              autoFocus
              value={editingName}
              onChange={(e) => setEditingName(e.target.value)}
              onBlur={commitName}
              onKeyDown={(e) => { if (e.key === "Enter") commitName(); if (e.key === "Escape") setIsEditingName(false); }}
              className="rounded bg-transparent px-1 text-sm font-semibold text-white outline-none ring-1 ring-brand-500"
            />
          ) : (
            <button
              onClick={startEditingName}
              className="text-sm font-semibold text-white hover:text-brand-400"
              title="Click to rename"
            >
              {graph?.name}
            </button>
          )}
          {isDirty && <span className="h-2 w-2 rounded-full bg-amber-400" title="Unsaved changes" />}
        </div>

        <div className="flex items-center gap-2">
          {error && (
            <span className="text-xs text-red-400 mr-2">{error}</span>
          )}
          <button
            onClick={handleAutoLayout}
            disabled={!graph || graph.nodes.length < 2}
            className="rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface-hover)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors hover:text-white disabled:opacity-40"
          >
            Layout
          </button>
          <button
            onClick={handleValidate}
            className="rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface-hover)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors hover:text-white"
          >
            Validate
          </button>
          {graph?.status !== "active" && (
            <button
              onClick={() => handleStatusChange("active")}
              className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-400 transition-colors hover:bg-emerald-500/20"
            >
              Publish
            </button>
          )}
          <button
            onClick={handleSave}
            disabled={!isDirty || saving}
            className="rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface-hover)] px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-40 text-[var(--text-secondary)] hover:text-white"
          >
            {saving ? "Saving..." : "Save"}
          </button>
          <button
            onClick={() => setShowExecuteDialog(true)}
            className="rounded-lg bg-brand-500 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-brand-600"
          >
            Execute
          </button>
        </div>
      </div>

      {/* ═══ Validation Banner ═══ */}
      {validationResult && (
        <div
          className={`border-b px-4 py-2 text-xs ${
            validationResult.valid
              ? "border-emerald-400/20 bg-emerald-400/10 text-emerald-400"
              : "border-red-400/20 bg-red-400/10 text-red-400"
          }`}
        >
          {validationResult.valid
            ? "✓ Graph is valid"
            : `⚠ ${(validationResult.errors || []).length} issue(s): ${(validationResult.errors || []).map((e) => e.message).join(", ")}`}
        </div>
      )}

      {/* ═══ Main Editor Area ═══ */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Node Palette */}
        <NodePalette
          graphName={graph?.name}
          graphStatus={graph?.status}
          graphVersion={graph?.version}
          nodeCount={graph?.nodes?.length || 0}
          edgeCount={graph?.edges?.length || 0}
          onStatusChange={handleStatusChange}
          stateFields={graph?.state?.fields ?? []}
          selectedStateFieldIndex={selectedStateFieldIndex}
          onSelectStateField={handleSelectStateField}
          onAddStateField={handleAddStateField}
          onDeleteStateField={handleDeleteStateField}
        />

        {/* Center: Canvas */}
        <div className="flex-1" onDragOver={onDragOver} onDrop={onDrop}>
          <ReactFlow
            nodes={flowNodes}
            edges={flowEdges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            deleteKeyCode={null}
            onEdgeClick={onEdgeClick}
            onPaneClick={onPaneClick}
            onNodeDragStop={onNodeDragStop}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            fitView
            proOptions={{ hideAttribution: true }}
            style={{ background: "var(--bg-primary)" }}
            defaultEdgeOptions={{ type: "brockley" }}
          >
            <Background color="rgba(255,255,255,0.05)" gap={20} />
            <Controls
              className="!border-[var(--border-primary)] !bg-[var(--bg-surface)] [&>button]:!border-[var(--border-primary)] [&>button]:!bg-[var(--bg-surface)] [&>button]:!fill-[var(--text-secondary)]"
            />
            <MiniMap
              nodeColor={minimapNodeColor}
              maskColor="rgba(0,0,0,0.7)"
              className="!bg-[#111318] !border-[var(--border-primary)]"
            />
          </ReactFlow>
        </div>

        {/* Right: Properties Panel */}
        {graph && (
          <PropertiesPanel
            selectedNodeId={selectedNodeId}
            selectedEdgeId={selectedEdgeId}
            selectedStateFieldIndex={selectedStateFieldIndex}
            graph={graph}
            onUpdateNode={handleUpdateNode}
            onDeleteNode={handleDeleteNode}
            onUpdateEdge={handleUpdateEdge}
            onDeleteEdge={handleDeleteEdge}
            onUpdateStateField={handleUpdateStateField}
            onDeleteStateField={handleDeleteStateField}
            onSelectNode={handleSelectNode}
            onSelectStateField={handleSelectStateField}
            onAddStateField={handleAddStateField}
            onClose={() => { setSelectedNodeId(null); setSelectedEdgeId(null); setSelectedStateFieldIndex(null); }}
            jsonText={jsonText}
            onJsonChange={handleJsonChange}
            showJson={showJson}
            onToggleJson={() => setShowJson((v) => !v)}
          />
        )}
      </div>

      {/* ═══ Status Bar ═══ */}
      <StatusBar
        validationResult={validationResult ? { valid: validationResult.valid, errorCount: (validationResult.errors || []).length } : undefined}
        nodeCount={graph?.nodes?.length || 0}
        edgeCount={graph?.edges?.length || 0}
        version={graph?.version || 1}
        status={graph?.status || "draft"}
        isDirty={isDirty}
        lastSaved={lastSaved || undefined}
      />

      {/* ═══ Execute Dialog ═══ */}
      {showExecuteDialog && graph && (
        <ExecuteDialog
          graphId={currentGraphId || ""}
          nodes={graph.nodes || []}
          onClose={() => setShowExecuteDialog(false)}
          onExecute={handleExecuteWithResult}
        />
      )}
    </div>
  );
}
