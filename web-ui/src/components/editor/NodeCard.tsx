import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";

export interface NodeCardData {
  label: string;
  nodeType: string;
  outputLabel?: string;
  hasError?: boolean;
  isRunning?: boolean;
  isCompleted?: boolean;
  isFailed?: boolean;
  isSkipped?: boolean;
  errorMessage?: string;
}

const TYPE_STYLES: Record<string, { badge: string; label: string }> = {
  llm: {
    badge: "bg-brand-500/20 text-brand-400 border border-brand-500/30",
    label: "LLM",
  },
  tool: {
    badge: "bg-emerald-500/20 text-emerald-400 border border-emerald-500/30",
    label: "Tool",
  },
  conditional: {
    badge: "bg-amber-500/20 text-amber-400 border border-amber-500/30",
    label: "Conditional",
  },
  transform: {
    badge: "bg-cyan-500/20 text-cyan-400 border border-cyan-500/30",
    label: "Transform",
  },
  foreach: {
    badge: "bg-violet-500/20 text-violet-400 border border-violet-500/30",
    label: "ForEach",
  },
  subgraph: {
    badge: "bg-purple-500/20 text-purple-400 border border-purple-500/30",
    label: "Subgraph",
  },
  human_in_the_loop: {
    badge: "bg-orange-500/20 text-orange-400 border border-orange-500/30",
    label: "HITL",
  },
  input: {
    badge: "bg-cyan-500/20 text-cyan-400 border border-cyan-500/30",
    label: "Input",
  },
  output: {
    badge: "bg-rose-500/20 text-rose-400 border border-rose-500/30",
    label: "Output",
  },
  superagent: {
    badge: "bg-pink-500/20 text-pink-400 border border-pink-500/30",
    label: "Superagent",
  },
  api_tool: {
    badge: "bg-sky-500/20 text-sky-400 border border-sky-500/30",
    label: "API Tool",
  },
};

function getBorderClasses(data: NodeCardData): string {
  if (data.isFailed || data.hasError) {
    return "border-red-400 ring-2 ring-red-400/40";
  }
  if (data.isRunning) {
    return "border-brand-400 ring-2 ring-brand-400/40 animate-pulse";
  }
  if (data.isCompleted) {
    return "border-emerald-400";
  }
  return "border-[rgba(255,255,255,0.08)]";
}

function NodeCard({ data }: NodeProps & { data: NodeCardData }) {
  const typeStyle = TYPE_STYLES[data.nodeType] || {
    badge: "bg-gray-500/20 text-gray-400 border border-gray-500/30",
    label: data.nodeType,
  };

  const borderClasses = getBorderClasses(data);
  const skippedClass = data.isSkipped ? "opacity-50" : "";

  return (
    <div
      className={`min-w-[180px] rounded-lg border bg-[#1a1d25] p-3 shadow-lg ${borderClasses} ${skippedClass}`}
    >
      {/* Target handle (left) — hidden for input nodes */}
      {data.nodeType !== "input" && (
        <Handle
          type="target"
          position={Position.Left}
          className="!h-3 !w-3 !border-2 !border-gray-600 !bg-gray-400"
        />
      )}

      {/* Type badge */}
      <span
        className={`inline-block rounded-full px-2 py-0.5 text-[10px] font-semibold ${typeStyle.badge}`}
      >
        {typeStyle.label}
      </span>

      {/* Node name */}
      <div className="mt-1.5 text-sm font-medium text-gray-100">
        {data.label}
      </div>

      {/* Output label */}
      {data.outputLabel && (
        <div className="mt-1 text-xs text-gray-500">
          Out: {data.outputLabel}
        </div>
      )}

      {/* Error message */}
      {data.errorMessage && (
        <div className="mt-1.5 flex items-start gap-1 text-[11px] text-red-400">
          <svg
            className="mt-px h-3 w-3 shrink-0"
            viewBox="0 0 16 16"
            fill="currentColor"
          >
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm-.75 3.75a.75.75 0 011.5 0v3.5a.75.75 0 01-1.5 0v-3.5zM8 11a1 1 0 110 2 1 1 0 010-2z" />
          </svg>
          <span>{data.errorMessage}</span>
        </div>
      )}

      {/* Source handle (right) — hidden for output nodes */}
      {data.nodeType !== "output" && (
        <Handle
          type="source"
          position={Position.Right}
          className="!h-3 !w-3 !border-2 !border-gray-600 !bg-gray-400"
        />
      )}
    </div>
  );
}

export default memo(NodeCard);
