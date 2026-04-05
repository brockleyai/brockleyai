import React from "react";

export interface EdgeData {
  source_node_id: string;
  source_port: string;
  target_node_id: string;
  target_port: string;
  back_edge?: boolean;
  condition?: string;
  max_iterations?: number;
}

interface EdgeFormProps {
  data: EdgeData;
  sourceNodeName: string;
  targetNodeName: string;
  onChange: (data: Partial<EdgeData>) => void;
  onDelete: () => void;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const EdgeForm: React.FC<EdgeFormProps> = ({
  data,
  sourceNodeName,
  targetNodeName,
  onChange,
  onDelete,
}) => {
  return (
    <div className="space-y-4">
      {/* Source (read-only) */}
      <div>
        <label className={labelClass}>Source</label>
        <div className="bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-400">
          {sourceNodeName} : {data.source_port}
        </div>
      </div>

      {/* Target (read-only) */}
      <div>
        <label className={labelClass}>Target</label>
        <div className="bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-400">
          {targetNodeName} : {data.target_port}
        </div>
      </div>

      {/* Back Edge */}
      <div className={sectionClass}>
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            className="accent-brand-500"
            checked={data.back_edge ?? false}
            onChange={(e) => onChange({ back_edge: e.target.checked })}
          />
          <span className="text-sm text-gray-300">Back Edge</span>
        </label>
      </div>

      {/* Max Iterations (only when back_edge) */}
      {data.back_edge && (
        <div>
          <label className={labelClass}>Max Iterations</label>
          <input
            type="number"
            className={inputClass}
            value={data.max_iterations ?? ""}
            onChange={(e) =>
              onChange({
                max_iterations: e.target.value
                  ? parseInt(e.target.value, 10)
                  : undefined,
              })
            }
            placeholder="10"
            min={1}
          />
        </div>
      )}

      {/* Condition */}
      <div>
        <label className={labelClass}>Condition</label>
        <input
          type="text"
          className={inputClass}
          value={data.condition ?? ""}
          onChange={(e) => onChange({ condition: e.target.value })}
          placeholder="Expression (optional)"
        />
      </div>

      {/* Delete */}
      <div className={sectionClass}>
        <button
          type="button"
          onClick={onDelete}
          className="w-full px-3 py-2 text-sm font-medium text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg hover:bg-red-500/20 transition-colors"
        >
          Delete Edge
        </button>
      </div>
    </div>
  );
};

export default EdgeForm;
