import type { StateBinding, StateField, Port } from "../../store";

interface StateBindingEditorProps {
  label: string;
  bindings: StateBinding[];
  stateFields: StateField[];
  ports: Port[];
  onChange: (bindings: StateBinding[]) => void;
}

export function StateBindingEditor({
  label,
  bindings,
  stateFields,
  ports,
  onChange,
}: StateBindingEditorProps) {
  const addBinding = () => {
    onChange([...bindings, { state_field: "", port: "" }]);
  };

  const removeBinding = (index: number) => {
    onChange(bindings.filter((_, i) => i !== index));
  };

  const updateBinding = (
    index: number,
    field: keyof StateBinding,
    value: string
  ) => {
    const updated = bindings.map((b, i) =>
      i === index ? { ...b, [field]: value } : b
    );
    onChange(updated);
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="text-xs font-medium text-zinc-400">{label}</label>
        <button
          onClick={addBinding}
          className="text-xs text-blue-400 hover:text-blue-300"
        >
          + Add
        </button>
      </div>

      {bindings.map((binding, index) => (
        <div key={index} className="flex items-center gap-2">
          <select
            value={binding.state_field}
            onChange={(e) =>
              updateBinding(index, "state_field", e.target.value)
            }
            className="flex-1 bg-zinc-800 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-200"
          >
            <option value="">State field...</option>
            {stateFields.map((f) => (
              <option key={f.name} value={f.name}>
                {f.name}
              </option>
            ))}
          </select>

          <span className="text-zinc-600 text-xs">↔</span>

          <select
            value={binding.port}
            onChange={(e) => updateBinding(index, "port", e.target.value)}
            className="flex-1 bg-zinc-800 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-200"
          >
            <option value="">Port...</option>
            {ports.map((p) => (
              <option key={p.name} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>

          <button
            onClick={() => removeBinding(index)}
            className="text-zinc-500 hover:text-red-400 text-xs"
          >
            x
          </button>
        </div>
      ))}

      {bindings.length === 0 && (
        <div className="text-xs text-zinc-600 italic">No bindings</div>
      )}
    </div>
  );
}
