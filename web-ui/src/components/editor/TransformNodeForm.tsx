import React from "react";
import type { GraphNode, GraphState } from "../../store";
import { VariableBrowser } from "./VariableBrowser";

export interface TransformConfig {
  expressions?: Record<string, string>;
}

interface TransformNodeFormProps {
  config: TransformConfig;
  onChange: (config: TransformConfig) => void;
  node?: GraphNode;
  graphState?: GraphState;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";

const TransformNodeForm: React.FC<TransformNodeFormProps> = ({
  config,
  onChange,
  node,
  graphState,
}) => {
  const expressions = config.expressions ?? {};
  const entries = Object.entries(expressions);

  const updateKey = (oldKey: string, newKey: string) => {
    const updated: Record<string, string> = {};
    for (const [k, v] of entries) {
      updated[k === oldKey ? newKey : k] = v;
    }
    onChange({ ...config, expressions: updated });
  };

  const updateValue = (key: string, value: string) => {
    onChange({
      ...config,
      expressions: { ...expressions, [key]: value },
    });
  };

  const addExpression = () => {
    const key = `output_${entries.length}`;
    onChange({
      ...config,
      expressions: { ...expressions, [key]: "" },
    });
  };

  const removeExpression = (key: string) => {
    const updated = { ...expressions };
    delete updated[key];
    onChange({ ...config, expressions: updated });
  };

  return (
    <div className="space-y-4">
      <div>
        <label className={labelClass}>Expressions</label>
        <div className="space-y-3">
          {entries.map(([key, value]) => (
            <div
              key={key}
              className="flex items-start gap-2 bg-[rgba(255,255,255,0.02)] rounded-lg p-2"
            >
              <div className="w-28 flex-shrink-0">
                <input
                  type="text"
                  className={inputClass}
                  value={key}
                  onChange={(e) => updateKey(key, e.target.value)}
                  placeholder="Output name"
                />
              </div>
              <div className="flex-1">
                <input
                  type="text"
                  className={inputClass}
                  value={value}
                  onChange={(e) => updateValue(key, e.target.value)}
                  placeholder="input.x + input.y"
                />
              </div>
              <button
                type="button"
                onClick={() => removeExpression(key)}
                className="flex-shrink-0 mt-1.5 text-gray-500 hover:text-red-400 transition-colors"
                title="Remove expression"
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
          onClick={addExpression}
          className="mt-2 text-xs text-brand-400 hover:text-brand-300 transition-colors"
        >
          + Add Expression
        </button>
        <p className="mt-2 text-[11px] text-gray-600">
          Expressions can reference input.*, state.*, and meta.* namespaces.
        </p>

        {node && (
          <div className="mt-3">
            <VariableBrowser node={node} graphState={graphState} />
          </div>
        )}
      </div>
    </div>
  );
};

export default TransformNodeForm;
