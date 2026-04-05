import type { StateField } from "../../store";

interface StateFieldCardProps {
  field: StateField;
  selected?: boolean;
  onClick: () => void;
  onDelete: () => void;
}

const REDUCER_STYLES: Record<string, string> = {
  replace: "bg-gray-500/20 text-gray-400 border-gray-500/30",
  append: "bg-blue-500/20 text-blue-400 border-blue-500/30",
  merge: "bg-violet-500/20 text-violet-400 border-violet-500/30",
};

function getSchemaTypeLabel(schema: Record<string, unknown>): string {
  const type = schema.type as string | undefined;
  if (!type) return "any";
  if (type === "array") {
    const items = schema.items as Record<string, unknown> | undefined;
    const itemType = items?.type as string | undefined;
    return itemType ? `array<${itemType}>` : "array";
  }
  return type;
}

export function StateFieldCard({
  field,
  selected,
  onClick,
  onDelete,
}: StateFieldCardProps) {
  const reducerStyle = REDUCER_STYLES[field.reducer] ?? REDUCER_STYLES.replace;

  return (
    <button
      type="button"
      onClick={onClick}
      className={`group w-full rounded-lg border p-2.5 text-left transition-colors ${
        selected
          ? "border-blue-500/50 bg-blue-500/10"
          : "border-[rgba(255,255,255,0.08)] bg-[rgba(255,255,255,0.02)] hover:bg-[rgba(255,255,255,0.04)]"
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-white">{field.name || "unnamed"}</span>
        <div className="flex items-center gap-1.5 shrink-0">
          <span
            className={`rounded-full border px-1.5 py-0 text-[10px] font-medium ${reducerStyle}`}
          >
            {field.reducer}
          </span>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
            className="hidden text-gray-600 transition-colors hover:text-red-400 group-hover:block"
            title="Delete state field"
          >
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
      <div className="mt-0.5 text-[10px] text-gray-500">
        {getSchemaTypeLabel(field.schema)}
      </div>
    </button>
  );
}
