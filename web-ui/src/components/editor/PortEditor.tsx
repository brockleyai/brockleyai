import React from "react";
import type { Port } from "../../store";

interface PortEditorProps {
  ports: Port[];
  onChange: (ports: Port[]) => void;
  isInput?: boolean;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";

const SCHEMA_TYPES = ["string", "number", "boolean", "object", "array"];

function schemaToType(schema: Record<string, unknown>): string {
  const t = schema.type;
  if (typeof t === "string" && SCHEMA_TYPES.includes(t)) return t;
  return "string";
}

function typeToSchema(type: string): Record<string, unknown> {
  return { type };
}

const PortEditor: React.FC<PortEditorProps> = ({
  ports,
  onChange,
  isInput = false,
}) => {
  const updatePort = (index: number, patch: Partial<Port>) => {
    const updated = ports.map((p, i) =>
      i === index ? { ...p, ...patch } : p
    );
    onChange(updated);
  };

  const addPort = () => {
    onChange([
      ...ports,
      { name: "", schema: { type: "string" }, required: isInput ? false : undefined },
    ]);
  };

  const removePort = (index: number) => {
    onChange(ports.filter((_, i) => i !== index));
  };

  return (
    <div className="space-y-3">
      <label className={labelClass}>
        {isInput ? "Input Ports" : "Output Ports"}
      </label>
      {ports.map((port, i) => (
        <div
          key={i}
          className="flex items-center gap-2 bg-[rgba(255,255,255,0.02)] rounded-lg p-2"
        >
          <div className="flex-1">
            <input
              type="text"
              className={inputClass}
              value={port.name}
              onChange={(e) => updatePort(i, { name: e.target.value })}
              placeholder="Port name"
            />
          </div>
          <div className="w-24 flex-shrink-0">
            <select
              className={inputClass}
              value={schemaToType(port.schema)}
              onChange={(e) =>
                updatePort(i, { schema: typeToSchema(e.target.value) })
              }
            >
              {SCHEMA_TYPES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          {isInput && (
            <label className="flex items-center gap-1 flex-shrink-0 cursor-pointer">
              <input
                type="checkbox"
                className="accent-brand-500"
                checked={port.required ?? false}
                onChange={(e) => updatePort(i, { required: e.target.checked })}
              />
              <span className="text-[11px] text-gray-500">Req</span>
            </label>
          )}
          <button
            type="button"
            onClick={() => removePort(i)}
            className="flex-shrink-0 text-gray-500 hover:text-red-400 transition-colors"
            title="Remove port"
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
      <button
        type="button"
        onClick={addPort}
        className="text-xs text-brand-400 hover:text-brand-300 transition-colors"
      >
        + Add Port
      </button>
    </div>
  );
};

export default PortEditor;
