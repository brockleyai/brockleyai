interface StatusBarProps {
  validationResult?: { valid: boolean; errorCount: number };
  nodeCount: number;
  edgeCount: number;
  version: number;
  status: string;
  isDirty: boolean;
  lastSaved?: string;
}

function StatusBar({
  validationResult,
  nodeCount,
  edgeCount,
  version,
  status,
  isDirty,
  lastSaved,
}: StatusBarProps) {
  let validationIcon: string;
  let validationText: string;
  let validationColor: string;

  if (!validationResult) {
    validationIcon = "\u2014";
    validationText = "Not validated";
    validationColor = "text-gray-500";
  } else if (validationResult.valid) {
    validationIcon = "\u2713";
    validationText = "Valid";
    validationColor = "text-emerald-400";
  } else {
    validationIcon = "\u26A0";
    validationText = `${validationResult.errorCount} issue${validationResult.errorCount !== 1 ? "s" : ""}`;
    validationColor = "text-amber-400";
  }

  return (
    <div className="flex h-8 items-center justify-between border-t border-[rgba(255,255,255,0.08)] bg-[#111318] px-4 text-[11px]">
      {/* Left: validation */}
      <div className={`flex items-center gap-1.5 ${validationColor}`}>
        <span>{validationIcon}</span>
        <span>{validationText}</span>
      </div>

      {/* Center: counts */}
      <div className="text-gray-500">
        {nodeCount} nodes, {edgeCount} edges
      </div>

      {/* Right: version, status, dirty */}
      <div className="flex items-center gap-2 text-gray-500">
        <span>
          v{version} &middot; {status}
        </span>
        {isDirty && (
          <span
            className="inline-block h-2 w-2 rounded-full bg-amber-400"
            title="Unsaved changes"
          />
        )}
        {lastSaved && <span className="text-gray-600">{lastSaved}</span>}
      </div>
    </div>
  );
}

export default StatusBar;
