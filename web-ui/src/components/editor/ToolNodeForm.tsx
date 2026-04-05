import React from "react";

export interface ToolConfig {
  tool_name?: string;
  mcp_url?: string;
  mcp_transport?: string;
}

interface ToolNodeFormProps {
  config: ToolConfig;
  onChange: (config: ToolConfig) => void;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";

const TRANSPORTS = ["sse", "stdio"];

const ToolNodeForm: React.FC<ToolNodeFormProps> = ({ config, onChange }) => {
  const update = (patch: Partial<ToolConfig>) => {
    onChange({ ...config, ...patch });
  };

  return (
    <div className="space-y-4">
      {/* Tool Name */}
      <div>
        <label className={labelClass}>Tool Name</label>
        <input
          type="text"
          className={inputClass}
          value={config.tool_name ?? ""}
          onChange={(e) => update({ tool_name: e.target.value })}
          placeholder="my_tool"
        />
      </div>

      {/* MCP URL */}
      <div>
        <label className={labelClass}>MCP URL</label>
        <input
          type="text"
          className={inputClass}
          value={config.mcp_url ?? ""}
          onChange={(e) => update({ mcp_url: e.target.value })}
          placeholder="https://mcp-server.example.com/sse"
        />
      </div>

      {/* MCP Transport */}
      <div>
        <label className={labelClass}>MCP Transport</label>
        <select
          className={inputClass}
          value={config.mcp_transport ?? "sse"}
          onChange={(e) => update({ mcp_transport: e.target.value })}
        >
          {TRANSPORTS.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
};

export default ToolNodeForm;
