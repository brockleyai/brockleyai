import React from "react";
import type { GraphNode, GraphState } from "../../store";
import { VariableBrowser } from "./VariableBrowser";

export interface Branch {
  label: string;
  condition: string;
}

export interface ConditionalConfig {
  branches?: Branch[];
  default_label?: string;
}

interface ConditionalNodeFormProps {
  config: ConditionalConfig;
  onChange: (config: ConditionalConfig) => void;
  node?: GraphNode;
  graphState?: GraphState;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const ConditionalNodeForm: React.FC<ConditionalNodeFormProps> = ({
  config,
  onChange,
  node,
  graphState,
}) => {
  const branches = config.branches ?? [];

  const updateBranch = (index: number, patch: Partial<Branch>) => {
    const updated = branches.map((b, i) =>
      i === index ? { ...b, ...patch } : b
    );
    onChange({ ...config, branches: updated });
  };

  const addBranch = () => {
    onChange({
      ...config,
      branches: [...branches, { label: "", condition: "" }],
    });
  };

  const removeBranch = (index: number) => {
    onChange({
      ...config,
      branches: branches.filter((_, i) => i !== index),
    });
  };

  return (
    <div className="space-y-4">
      {/* Branches */}
      <div>
        <label className={labelClass}>Branches</label>
        <div className="space-y-3">
          {branches.map((branch, i) => (
            <div
              key={i}
              className="flex items-start gap-2 bg-[rgba(255,255,255,0.02)] rounded-lg p-2"
            >
              <div className="flex-shrink-0 w-24">
                <input
                  type="text"
                  className={inputClass}
                  value={branch.label}
                  onChange={(e) => updateBranch(i, { label: e.target.value })}
                  placeholder="Label"
                />
              </div>
              <div className="flex-1">
                <input
                  type="text"
                  className={inputClass}
                  value={branch.condition}
                  onChange={(e) =>
                    updateBranch(i, { condition: e.target.value })
                  }
                  placeholder='input.value.field == "x"'
                />
              </div>
              <button
                type="button"
                onClick={() => removeBranch(i)}
                className="flex-shrink-0 mt-1.5 text-gray-500 hover:text-red-400 transition-colors"
                title="Remove branch"
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
          ))}
        </div>
        <button
          type="button"
          onClick={addBranch}
          className="mt-2 text-xs text-brand-400 hover:text-brand-300 transition-colors"
        >
          + Add Branch
        </button>
        <p className="mt-2 text-[11px] text-gray-600">
          Conditions can reference input.*, state.*, and meta.* namespaces.
        </p>

        {node && (
          <div className="mt-3">
            <VariableBrowser node={node} graphState={graphState} />
          </div>
        )}
      </div>

      {/* Default Label */}
      <div className={sectionClass}>
        <label className={labelClass}>Default Label</label>
        <input
          type="text"
          className={inputClass}
          value={config.default_label ?? ""}
          onChange={(e) => onChange({ ...config, default_label: e.target.value })}
          placeholder="default"
        />
      </div>
    </div>
  );
};

export default ConditionalNodeForm;
