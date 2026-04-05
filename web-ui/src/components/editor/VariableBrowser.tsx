import { useState } from "react";
import type { GraphNode, GraphState } from "../../store";

interface VariableBrowserProps {
  node: GraphNode;
  graphState?: GraphState;
}

const META_FIELDS = [
  "node_id",
  "node_name",
  "node_type",
  "execution_id",
  "graph_id",
  "graph_name",
  "iteration",
];

const EXPRESSION_NODE_TYPES = ["llm", "transform", "conditional", "superagent"];

export function VariableBrowser({ node, graphState }: VariableBrowserProps) {
  const [showMeta, setShowMeta] = useState(false);

  if (!EXPRESSION_NODE_TYPES.includes(node.type)) {
    return null;
  }

  const inputPorts = node.input_ports || [];
  const stateFields = graphState?.fields || [];
  const hasState = stateFields.length > 0;

  return (
    <div className="rounded-lg border border-brand-500/20 bg-brand-500/5 px-3 py-2.5">
      <div className="text-[11px] font-semibold text-brand-400 uppercase tracking-wide mb-2">
        Variables
      </div>

      <div className="flex flex-wrap gap-1.5">
        {/* input.* variables */}
        {inputPorts.map((port) => (
          <code
            key={`i-${port.name}`}
            className="rounded bg-emerald-500/15 border border-emerald-500/20 px-1.5 py-0.5 text-[11px] text-emerald-400 cursor-default"
            title={`Input port — ${(port.schema as { type?: string })?.type || "any"}`}
          >
            {node.type === "llm" ? `{{input.${port.name}}}` : `input.${port.name}`}
          </code>
        ))}

        {/* state.* variables */}
        {stateFields.map((field) => (
          <code
            key={`s-${field.name}`}
            className="rounded bg-blue-500/15 border border-blue-500/20 px-1.5 py-0.5 text-[11px] text-blue-400 cursor-default"
            title={`State field — ${(field.schema as { type?: string })?.type || "any"} (${field.reducer})`}
          >
            {node.type === "llm" ? `{{state.${field.name}}}` : `state.${field.name}`}
          </code>
        ))}

        {/* meta toggle */}
        <button
          onClick={() => setShowMeta(!showMeta)}
          className="rounded bg-amber-500/10 border border-amber-500/20 px-1.5 py-0.5 text-[11px] text-amber-400/70 hover:text-amber-400 transition-colors"
        >
          meta.* {showMeta ? "▾" : "▸"}
        </button>
      </div>

      {/* Expanded meta fields */}
      {showMeta && (
        <div className="flex flex-wrap gap-1.5 mt-1.5">
          {META_FIELDS.map((name) => (
            <code
              key={`m-${name}`}
              className="rounded bg-amber-500/10 border border-amber-500/20 px-1.5 py-0.5 text-[11px] text-amber-400 cursor-default"
            >
              {node.type === "llm" ? `{{meta.${name}}}` : `meta.${name}`}
            </code>
          ))}
        </div>
      )}

      {inputPorts.length === 0 && !hasState && (
        <div className="text-[11px] text-gray-600 italic mt-1">
          No input ports or state fields defined
        </div>
      )}
    </div>
  );
}
