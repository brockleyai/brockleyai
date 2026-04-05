import React, { useState } from "react";
import type { Graph, GraphNode, GraphEdge, StateBinding, StateField } from "../../store";
import LlmNodeForm from "./LlmNodeForm";
import ToolNodeForm from "./ToolNodeForm";
import ConditionalNodeForm from "./ConditionalNodeForm";
import TransformNodeForm from "./TransformNodeForm";
import ApiToolNodeForm from "./ApiToolNodeForm";
import SuperagentNodeForm from "./SuperagentNodeForm";
import PortEditor from "./PortEditor";
import EdgeForm from "./EdgeForm";
import { StateBindingEditor } from "./StateBindingEditor";
import { StateFieldEditor } from "./StateFieldEditor";
import { StateFieldCard } from "./StateFieldCard";

interface PropertiesPanelProps {
  selectedNodeId: string | null;
  selectedEdgeId: string | null;
  selectedStateFieldIndex: number | null;
  graph: Graph;
  onUpdateNode: (nodeId: string, updates: Partial<GraphNode>) => void;
  onDeleteNode: (nodeId: string) => void;
  onUpdateEdge: (edgeId: string, updates: Partial<GraphEdge>) => void;
  onDeleteEdge: (edgeId: string) => void;
  onUpdateStateField: (index: number, field: StateField) => void;
  onDeleteStateField: (index: number) => void;
  onSelectNode: (nodeId: string) => void;
  onSelectStateField: (index: number) => void;
  onAddStateField: () => void;
  onClose: () => void;
  jsonText: string;
  onJsonChange: (text: string) => void;
  showJson: boolean;
  onToggleJson: () => void;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const NODE_TYPE_COLORS: Record<string, string> = {
  llm: "bg-purple-500/20 text-purple-300",
  tool: "bg-blue-500/20 text-blue-300",
  conditional: "bg-amber-500/20 text-amber-300",
  transform: "bg-emerald-500/20 text-emerald-300",
  input: "bg-cyan-500/20 text-cyan-300",
  output: "bg-rose-500/20 text-rose-300",
  api_tool: "bg-sky-500/20 text-sky-300",
  superagent: "bg-pink-500/20 text-pink-300",
};

const PropertiesPanel: React.FC<PropertiesPanelProps> = ({
  selectedNodeId,
  selectedEdgeId,
  selectedStateFieldIndex,
  graph,
  onUpdateNode,
  onDeleteNode,
  onUpdateEdge,
  onDeleteEdge,
  onUpdateStateField,
  onDeleteStateField,
  onSelectNode,
  onSelectStateField,
  onAddStateField,
  onClose,
  jsonText,
  onJsonChange,
  showJson,
  onToggleJson,
}) => {
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [stateOpen, setStateOpen] = useState(false);

  const selectedNode = selectedNodeId
    ? graph.nodes.find((n) => n.id === selectedNodeId)
    : null;

  const selectedEdge = selectedEdgeId
    ? graph.edges.find((e) => e.id === selectedEdgeId)
    : null;

  const stateFields = graph.state?.fields ?? [];
  const selectedField =
    selectedStateFieldIndex !== null ? stateFields[selectedStateFieldIndex] : null;

  const handleDeleteNode = (nodeId: string) => {
    if (confirmDelete === nodeId) {
      onDeleteNode(nodeId);
      setConfirmDelete(null);
    } else {
      setConfirmDelete(nodeId);
    }
  };

  const renderNodeForm = (node: GraphNode) => {
    const nodeType = node.type;

    switch (nodeType) {
      case "llm":
        return (
          <LlmNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
            node={node}
            graphState={graph.state}
          />
        );
      case "tool":
        return (
          <ToolNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
          />
        );
      case "conditional":
        return (
          <ConditionalNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
            node={node}
            graphState={graph.state}
          />
        );
      case "transform":
        return (
          <TransformNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
            node={node}
            graphState={graph.state}
          />
        );
      case "api_tool":
        return (
          <ApiToolNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
          />
        );
      case "superagent":
        return (
          <SuperagentNodeForm
            config={node.config}
            onChange={(config) => onUpdateNode(node.id, { config })}
            node={node}
            graphState={graph.state}
          />
        );
      case "input":
      case "output":
        return null;
      default:
        return (
          <div className="text-xs text-gray-500">
            No form available for type &quot;{nodeType}&quot;
          </div>
        );
    }
  };

  const renderNodeContent = (node: GraphNode) => {
    const badgeColor =
      NODE_TYPE_COLORS[node.type] ?? "bg-gray-500/20 text-gray-300";

    const hasStateFields = stateFields.length > 0;

    return (
      <>
        {/* Node name */}
        <div>
          <label className={labelClass}>Name</label>
          <input
            type="text"
            className={inputClass}
            value={node.name}
            onChange={(e) => onUpdateNode(node.id, { name: e.target.value })}
          />
        </div>

        {/* Node type badge */}
        <div className="flex items-center gap-2">
          <span
            className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${badgeColor}`}
          >
            {node.type}
          </span>
        </div>

        {/* Type-specific form */}
        <div className={sectionClass}>{renderNodeForm(node)}</div>

        {/* Port editors for input/output nodes */}
        {(node.type === "input" || node.type === "output") && (
          <div className={sectionClass}>
            <PortEditor
              ports={node.output_ports ?? []}
              onChange={(ports) => onUpdateNode(node.id, { output_ports: ports })}
              isInput={false}
            />
          </div>
        )}

        {/* Input ports (all node types) */}
        {node.input_ports && node.input_ports.length > 0 && (
          <div className={sectionClass}>
            <PortEditor
              ports={node.input_ports}
              onChange={(ports) => onUpdateNode(node.id, { input_ports: ports })}
              isInput={true}
            />
          </div>
        )}

        {/* Output ports (non-input/output node types) */}
        {node.type !== "input" &&
          node.type !== "output" &&
          node.output_ports &&
          node.output_ports.length > 0 && (
            <div className={sectionClass}>
              <PortEditor
                ports={node.output_ports}
                onChange={(ports) =>
                  onUpdateNode(node.id, { output_ports: ports })
                }
                isInput={false}
              />
            </div>
          )}

        {/* State bindings (collapsible) */}
        {hasStateFields && (
          <div className={sectionClass}>
            <button
              type="button"
              onClick={() => setStateOpen(!stateOpen)}
              className="flex w-full items-center justify-between mb-3"
            >
              <span className="text-xs font-medium text-blue-400 uppercase tracking-wider">
                State
              </span>
              <svg
                className={`h-3 w-3 text-blue-400 transition-transform ${stateOpen ? "rotate-180" : ""}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            {stateOpen && (
              <div className="space-y-3">
                <StateBindingEditor
                  label="State Reads"
                  bindings={node.state_reads ?? []}
                  stateFields={stateFields}
                  ports={node.input_ports ?? []}
                  onChange={(bindings: StateBinding[]) =>
                    onUpdateNode(node.id, { state_reads: bindings } as Partial<GraphNode>)
                  }
                />
                <StateBindingEditor
                  label="State Writes"
                  bindings={node.state_writes ?? []}
                  stateFields={stateFields}
                  ports={node.output_ports ?? []}
                  onChange={(bindings: StateBinding[]) =>
                    onUpdateNode(node.id, { state_writes: bindings } as Partial<GraphNode>)
                  }
                />
                <p className="text-[10px] text-gray-600">
                  Reads inject state into input ports. Writes push output ports to state.
                </p>
                {node.type === "conditional" && (
                  <p className="text-[10px] text-amber-400/80 border border-amber-500/20 bg-amber-500/5 rounded px-2 py-1.5">
                    To use state in branch conditions, add a state read binding.
                    Reference the value as <code className="text-amber-300">input.port_name</code> in your condition.
                  </p>
                )}

                {/* Orphaned binding warnings */}
                {(node.state_reads ?? []).map((b, i) =>
                  b.state_field && !stateFields.some((f) => f.name === b.state_field) ? (
                    <p key={`or-${i}`} className="text-[11px] text-red-400">
                      Unknown field: {b.state_field}
                    </p>
                  ) : null
                )}
                {(node.state_writes ?? []).map((b, i) =>
                  b.state_field && !stateFields.some((f) => f.name === b.state_field) ? (
                    <p key={`ow-${i}`} className="text-[11px] text-red-400">
                      Unknown field: {b.state_field}
                    </p>
                  ) : null
                )}
              </div>
            )}
          </div>
        )}

        {/* Delete node */}
        <div className={sectionClass}>
          <button
            type="button"
            onClick={() => handleDeleteNode(node.id)}
            className={`w-full px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
              confirmDelete === node.id
                ? "text-white bg-red-600 hover:bg-red-700"
                : "text-red-400 bg-red-500/10 border border-red-500/20 hover:bg-red-500/20"
            }`}
          >
            {confirmDelete === node.id
              ? "Click again to confirm"
              : "Delete Node"}
          </button>
        </div>
      </>
    );
  };

  const renderEdgeContent = (edge: GraphEdge) => {
    const sourceNode = graph.nodes.find((n) => n.id === edge.source_node_id);
    const targetNode = graph.nodes.find((n) => n.id === edge.target_node_id);

    return (
      <EdgeForm
        data={edge}
        sourceNodeName={sourceNode?.name ?? edge.source_node_id}
        targetNodeName={targetNode?.name ?? edge.target_node_id}
        onChange={(updates) => onUpdateEdge(edge.id, updates)}
        onDelete={() => onDeleteEdge(edge.id)}
      />
    );
  };

  const renderStateFieldContent = (field: StateField) => {
    return (
      <StateFieldEditor
        field={field}
        index={selectedStateFieldIndex!}
        allFields={stateFields}
        nodes={graph.nodes}
        onChange={(updated) => onUpdateStateField(selectedStateFieldIndex!, updated)}
        onDelete={() => onDeleteStateField(selectedStateFieldIndex!)}
        onSelectNode={onSelectNode}
      />
    );
  };

  const renderEmptyState = () => (
    <div className="space-y-4">
      <div className="text-sm text-gray-500 text-center py-4">
        Select a node or edge to edit
      </div>

      {/* State fields overview */}
      <div className={sectionClass}>
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs font-medium text-blue-400 uppercase tracking-wider">
            State Fields{stateFields.length > 0 ? ` (${stateFields.length})` : ""}
          </span>
        </div>
        {stateFields.length > 0 ? (
          <div className="space-y-1.5">
            {stateFields.map((field, i) => (
              <StateFieldCard
                key={i}
                field={field}
                selected={false}
                onClick={() => onSelectStateField(i)}
                onDelete={() => onDeleteStateField(i)}
              />
            ))}
          </div>
        ) : (
          <p className="text-[11px] text-gray-600 italic">No state fields defined.</p>
        )}
        <button
          type="button"
          onClick={onAddStateField}
          className="mt-2 text-xs text-blue-400 hover:text-blue-300 transition-colors"
        >
          + Add State Field
        </button>
      </div>
    </div>
  );

  return (
    <div className="w-80 border-l border-[rgba(255,255,255,0.08)] bg-[#111318] flex flex-col overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[rgba(255,255,255,0.08)]">
        <h2 className="text-sm font-semibold text-gray-200">Properties</h2>
        <button
          type="button"
          onClick={onClose}
          className="text-gray-500 hover:text-gray-300 transition-colors"
        >
          <svg
            className="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-4">
        {selectedNode ? (
          renderNodeContent(selectedNode)
        ) : selectedEdge ? (
          renderEdgeContent(selectedEdge)
        ) : selectedField ? (
          renderStateFieldContent(selectedField)
        ) : (
          renderEmptyState()
        )}
      </div>

      {/* JSON View toggle */}
      <div className="border-t border-[rgba(255,255,255,0.08)]">
        <button
          type="button"
          onClick={onToggleJson}
          className="w-full px-4 py-2 text-xs font-medium text-gray-400 hover:text-gray-300 hover:bg-[rgba(255,255,255,0.03)] transition-colors flex items-center justify-between"
        >
          <span>JSON</span>
          <svg
            className={`w-3 h-3 transition-transform ${
              showJson ? "rotate-180" : ""
            }`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M5 15l7-7 7 7"
            />
          </svg>
        </button>
        {showJson && (
          <div className="px-4 pb-4">
            <textarea
              className="w-full h-64 bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-xs text-gray-300 font-mono focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50 resize-y"
              value={jsonText}
              onChange={(e) => onJsonChange(e.target.value)}
              spellCheck={false}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default PropertiesPanel;
