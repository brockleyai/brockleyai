import { useState } from "react";
import type { StateField, GraphNode } from "../../store";

interface StateFieldEditorProps {
  field: StateField;
  index: number;
  allFields: StateField[];
  nodes: GraphNode[];
  onChange: (field: StateField) => void;
  onDelete: () => void;
  onSelectNode: (nodeId: string) => void;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const SCHEMA_TYPES = ["string", "number", "integer", "boolean", "object", "array"] as const;

const REDUCER_OPTIONS: Array<{
  value: StateField["reducer"];
  label: string;
  activeClass: string;
}> = [
  { value: "replace", label: "replace", activeClass: "bg-gray-500/20 text-gray-300 border-gray-500/30" },
  { value: "append", label: "append", activeClass: "bg-blue-500/20 text-blue-400 border-blue-500/30" },
  { value: "merge", label: "merge", activeClass: "bg-violet-500/20 text-violet-400 border-violet-500/30" },
];

const INITIAL_PLACEHOLDERS: Record<string, string> = {
  string: '""',
  number: "0",
  integer: "0",
  boolean: "false",
  object: "{}",
  array: "[]",
};

function getReducerValidation(reducer: string, schemaType: string): string | null {
  if (reducer === "append" && schemaType !== "array") return "append requires array type";
  if (reducer === "merge" && schemaType !== "object") return "merge requires object type";
  return null;
}

function isValidJson(value: string): boolean {
  if (!value.trim()) return true;
  try {
    JSON.parse(value);
    return true;
  } catch {
    return false;
  }
}

export function StateFieldEditor({
  field,
  index,
  allFields,
  nodes,
  onChange,
  onDelete,
  onSelectNode,
}: StateFieldEditorProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [initialText, setInitialText] = useState(
    field.initial !== undefined ? JSON.stringify(field.initial) : ""
  );

  const schemaType = (field.schema.type as string) || "string";
  const itemsType = (field.schema.items as Record<string, unknown>)?.type as string | undefined;

  // Validation
  const isDuplicateName = allFields.some(
    (f, i) => i !== index && f.name === field.name && field.name !== ""
  );
  const isEmptyName = field.name.trim() === "";
  const reducerError = getReducerValidation(field.reducer, schemaType);
  const isInitialInvalid = !isValidJson(initialText);

  // Find nodes that reference this field
  const usedBy: Array<{ nodeId: string; nodeName: string; direction: "reads" | "writes"; port: string }> = [];
  for (const node of nodes) {
    for (const binding of node.state_reads ?? []) {
      if (binding.state_field === field.name) {
        usedBy.push({ nodeId: node.id, nodeName: node.name, direction: "reads", port: binding.port });
      }
    }
    for (const binding of node.state_writes ?? []) {
      if (binding.state_field === field.name) {
        usedBy.push({ nodeId: node.id, nodeName: node.name, direction: "writes", port: binding.port });
      }
    }
  }

  const updateSchema = (type: string, items?: string) => {
    const newSchema: Record<string, unknown> = { type };
    if (type === "array" && items) {
      newSchema.items = { type: items };
    } else if (type === "array") {
      newSchema.items = { type: "string" };
    }
    onChange({ ...field, schema: newSchema });
  };

  const handleInitialChange = (text: string) => {
    setInitialText(text);
    if (!text.trim()) {
      onChange({ ...field, initial: undefined });
      return;
    }
    try {
      const parsed = JSON.parse(text);
      onChange({ ...field, initial: parsed });
    } catch {
      // Keep text for display but don't update field until valid
    }
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-2">
        <span className="h-2.5 w-2.5 rounded-full bg-blue-400" />
        <span className="text-xs font-semibold uppercase tracking-wider text-blue-400">
          State Field
        </span>
      </div>

      {/* Name */}
      <div>
        <label className={labelClass}>Name</label>
        <input
          type="text"
          className={`${inputClass} ${isDuplicateName || isEmptyName ? "!border-red-500/50" : ""}`}
          value={field.name}
          onChange={(e) => onChange({ ...field, name: e.target.value })}
          placeholder="field_name"
        />
        {isDuplicateName && (
          <p className="mt-1 text-[11px] text-red-400">Name already exists</p>
        )}
        {isEmptyName && field.name !== undefined && (
          <p className="mt-1 text-[11px] text-red-400">Name required</p>
        )}
      </div>

      {/* Schema Type */}
      <div className={sectionClass}>
        <label className={labelClass}>Schema Type</label>
        <select
          value={schemaType}
          onChange={(e) => updateSchema(e.target.value, e.target.value === "array" ? (itemsType || "string") : undefined)}
          className={inputClass}
        >
          {SCHEMA_TYPES.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </div>

      {/* Items Type (array only) */}
      {schemaType === "array" && (
        <div>
          <label className={labelClass}>Items Type</label>
          <select
            value={itemsType || "string"}
            onChange={(e) => updateSchema("array", e.target.value)}
            className={inputClass}
          >
            {SCHEMA_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
        </div>
      )}

      {/* Reducer */}
      <div className={sectionClass}>
        <label className={labelClass}>Reducer</label>
        <div className="flex rounded-lg border border-[rgba(255,255,255,0.08)] overflow-hidden">
          {REDUCER_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => onChange({ ...field, reducer: opt.value })}
              className={`flex-1 px-3 py-1.5 text-xs font-medium transition-colors border-r last:border-r-0 border-[rgba(255,255,255,0.08)] ${
                field.reducer === opt.value
                  ? opt.activeClass
                  : "bg-transparent text-gray-600 hover:text-gray-400"
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
        {reducerError && (
          <p className="mt-1.5 text-[11px] text-red-400">{reducerError}</p>
        )}
      </div>

      {/* Initial Value */}
      <div className={sectionClass}>
        <label className={labelClass}>Initial Value</label>
        <input
          type="text"
          className={`${inputClass} font-mono text-xs ${isInitialInvalid ? "!border-red-500/50" : ""}`}
          value={initialText}
          onChange={(e) => handleInitialChange(e.target.value)}
          placeholder={INITIAL_PLACEHOLDERS[schemaType] || "null"}
        />
        <p className="mt-1 text-[10px] text-gray-600">
          JSON value. Leave empty for type default.
        </p>
        {isInitialInvalid && (
          <p className="mt-1 text-[11px] text-red-400">Invalid JSON</p>
        )}
      </div>

      {/* Used By */}
      <div className={sectionClass}>
        <label className={labelClass}>Used By</label>
        {usedBy.length === 0 ? (
          <p className="text-[11px] text-gray-600 italic">No nodes use this field yet.</p>
        ) : (
          <div className="space-y-1">
            {usedBy.map((u, i) => (
              <button
                key={i}
                type="button"
                onClick={() => onSelectNode(u.nodeId)}
                className="block w-full text-left text-[11px] text-blue-400 hover:text-blue-300 transition-colors"
              >
                {u.nodeName}: {u.direction === "reads" ? "reads" : "writes"}{" "}
                {u.direction === "reads" ? "→" : "←"} {u.port}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Delete */}
      <div className={sectionClass}>
        <button
          type="button"
          onClick={() => {
            if (confirmDelete) {
              onDelete();
              setConfirmDelete(false);
            } else {
              setConfirmDelete(true);
            }
          }}
          className={`w-full px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
            confirmDelete
              ? "text-white bg-red-600 hover:bg-red-700"
              : "text-red-400 bg-red-500/10 border border-red-500/20 hover:bg-red-500/20"
          }`}
        >
          {confirmDelete ? "Click again to confirm" : "Delete State Field"}
        </button>
        {confirmDelete && usedBy.length > 0 && (
          <p className="mt-1.5 text-[11px] text-amber-400">
            Used by {usedBy.length} binding{usedBy.length > 1 ? "s" : ""}. Deleting will remove all bindings.
          </p>
        )}
      </div>
    </div>
  );
}
