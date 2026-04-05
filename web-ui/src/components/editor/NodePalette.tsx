import { useState, type DragEvent } from "react";
import type { StateField } from "../../store";
import { StateFieldCard } from "./StateFieldCard";

interface NodePaletteProps {
  graphName?: string;
  graphStatus?: string;
  graphVersion?: number;
  nodeCount: number;
  edgeCount: number;
  onStatusChange?: (status: string) => void;
  stateFields: StateField[];
  selectedStateFieldIndex: number | null;
  onSelectStateField: (index: number) => void;
  onAddStateField: () => void;
  onDeleteStateField: (index: number) => void;
}

interface PaletteItem {
  type: string;
  label: string;
  description: string;
  borderColor: string;
  textColor: string;
}

const PALETTE_ITEMS: PaletteItem[] = [
  {
    type: "llm",
    label: "LLM",
    description: "Call a language model",
    borderColor: "border-brand-500/30",
    textColor: "text-brand-400",
  },
  {
    type: "tool",
    label: "Tool",
    description: "Invoke an external tool",
    borderColor: "border-emerald-500/30",
    textColor: "text-emerald-400",
  },
  {
    type: "conditional",
    label: "Conditional",
    description: "Branch on a condition",
    borderColor: "border-amber-500/30",
    textColor: "text-amber-400",
  },
  {
    type: "transform",
    label: "Transform",
    description: "Transform data with expressions",
    borderColor: "border-cyan-500/30",
    textColor: "text-cyan-400",
  },
  {
    type: "foreach",
    label: "ForEach",
    description: "Iterate over a collection",
    borderColor: "border-violet-500/30",
    textColor: "text-violet-400",
  },
  {
    type: "subgraph",
    label: "Subgraph",
    description: "Embed another graph",
    borderColor: "border-purple-500/30",
    textColor: "text-purple-400",
  },
  {
    type: "human_in_the_loop",
    label: "HITL",
    description: "Wait for human approval",
    borderColor: "border-orange-500/30",
    textColor: "text-orange-400",
  },
  {
    type: "api_tool",
    label: "API Tool",
    description: "Call a REST API endpoint",
    borderColor: "border-sky-500/30",
    textColor: "text-sky-400",
  },
  {
    type: "superagent",
    label: "Superagent",
    description: "Autonomous agent with tools",
    borderColor: "border-pink-500/30",
    textColor: "text-pink-400",
  },
];

function StatusBadge({ status }: { status: string }) {
  let classes = "rounded-full px-2 py-0.5 text-[10px] font-medium border ";
  switch (status) {
    case "active":
      classes += "bg-emerald-500/20 text-emerald-400 border-emerald-500/30";
      break;
    case "archived":
      classes += "bg-gray-600/20 text-gray-500 border-gray-600/30";
      break;
    default:
      classes += "bg-gray-500/20 text-gray-400 border-gray-500/30";
  }
  return <span className={classes}>{status}</span>;
}

function handleDragStart(e: DragEvent, item: PaletteItem) {
  e.dataTransfer.setData("application/reactflow-type", item.type);
  e.dataTransfer.setData("application/reactflow-label", item.label);
  e.dataTransfer.effectAllowed = "move";
}

function NodePalette({
  graphName,
  graphStatus,
  graphVersion,
  nodeCount,
  edgeCount,
  onStatusChange,
  stateFields,
  selectedStateFieldIndex,
  onSelectStateField,
  onAddStateField,
  onDeleteStateField,
}: NodePaletteProps) {
  const [stateExpanded, setStateExpanded] = useState(stateFields.length > 0);

  return (
    <aside className="flex w-48 shrink-0 flex-col border-r border-[rgba(255,255,255,0.08)] bg-[#111318]">
      {/* Graph info */}
      <div className="space-y-2 p-3">
        {graphName && (
          <div className="text-sm font-semibold text-white">{graphName}</div>
        )}
        <div className="flex items-center gap-2">
          {graphStatus && (
            <button
              onClick={() => onStatusChange?.(graphStatus === "active" ? "draft" : "active")}
              title={`Click to mark as ${graphStatus === "active" ? "draft" : "active"}`}
              className="cursor-pointer transition-opacity hover:opacity-80"
            >
              <StatusBadge status={graphStatus} />
            </button>
          )}
          {graphVersion !== undefined && (
            <span className="text-xs text-gray-500">v{graphVersion}</span>
          )}
        </div>
        <div className="text-xs text-gray-600">
          {nodeCount} nodes &middot; {edgeCount} edges
        </div>
      </div>

      {/* Separator */}
      <div className="border-b border-[rgba(255,255,255,0.08)]" />

      {/* State Fields */}
      <div className="border-b border-[rgba(255,255,255,0.08)]">
        <button
          type="button"
          onClick={() => setStateExpanded(!stateExpanded)}
          className="flex w-full items-center justify-between px-3 py-2 text-xs font-medium uppercase tracking-wider text-blue-400 hover:text-blue-300 transition-colors"
        >
          <span>
            State Fields{stateFields.length > 0 ? ` (${stateFields.length})` : ""}
          </span>
          <svg
            className={`h-3 w-3 transition-transform ${stateExpanded ? "rotate-180" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {stateExpanded && (
          <div className="space-y-1.5 px-3 pb-3">
            {stateFields.map((field, i) => (
              <StateFieldCard
                key={i}
                field={field}
                selected={selectedStateFieldIndex === i}
                onClick={() => onSelectStateField(i)}
                onDelete={() => onDeleteStateField(i)}
              />
            ))}
            <button
              type="button"
              onClick={onAddStateField}
              className="w-full text-left text-xs text-blue-400 hover:text-blue-300 transition-colors py-1"
            >
              + Add State Field
            </button>
          </div>
        )}
      </div>

      {/* Node types */}
      <div className="flex-1 overflow-y-auto p-3">
        <div className="mb-3 text-xs font-medium uppercase tracking-wider text-gray-500">
          Node Types
        </div>

        <div className="space-y-2">
          {PALETTE_ITEMS.map((item) => (
            <div
              key={item.type}
              draggable
              onDragStart={(e) => handleDragStart(e, item)}
              className={`cursor-grab rounded-lg border p-3 text-sm font-medium transition-colors hover:brightness-110 active:cursor-grabbing ${item.borderColor} ${item.textColor}`}
            >
              {item.label}
              <div className="mt-0.5 text-[10px] text-gray-500">
                {item.description}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Drag hint */}
      <div className="border-t border-[rgba(255,255,255,0.08)] px-3 py-2">
        <p className="text-[10px] italic text-gray-600">
          Drag nodes onto the canvas
        </p>
      </div>
    </aside>
  );
}

export default NodePalette;
