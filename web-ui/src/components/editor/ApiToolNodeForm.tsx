import React, { useEffect, useState } from "react";
import type { APIToolDefinition } from "../../store";
import { useAppStore } from "../../store";
import { fetchAPITools } from "../../api";

export interface ApiToolConfig {
  api_tool_id?: string;
  endpoint?: string;
  inline_endpoint?: {
    base_url?: string;
    method?: string;
    path?: string;
    default_headers?: { name: string; value: string }[];
    input_schema?: Record<string, unknown>;
    output_schema?: Record<string, unknown>;
  };
}

interface ApiToolNodeFormProps {
  config: ApiToolConfig;
  onChange: (config: ApiToolConfig) => void;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];

const ApiToolNodeForm: React.FC<ApiToolNodeFormProps> = ({ config, onChange }) => {
  const { serverUrl, apiKey } = useAppStore();
  const [apiToolDefs, setApiToolDefs] = useState<APIToolDefinition[]>([]);
  const [mode, setMode] = useState<"reference" | "inline">(
    config.inline_endpoint ? "inline" : "reference"
  );

  useEffect(() => {
    fetchAPITools(serverUrl, apiKey).then(setApiToolDefs).catch(() => {});
  }, [serverUrl, apiKey]);

  const update = (patch: Partial<ApiToolConfig>) => {
    onChange({ ...config, ...patch });
  };

  const selectedDef = apiToolDefs.find((d) => d.id === config.api_tool_id);

  return (
    <div className="space-y-4">
      {/* Mode toggle */}
      <div>
        <label className={labelClass}>Mode</label>
        <div className="flex gap-1">
          {(["reference", "inline"] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={`rounded-md border px-3 py-1 text-xs font-medium transition-all ${
                mode === m
                  ? "border-brand-500/40 bg-brand-500/10 text-brand-400"
                  : "border-transparent text-gray-600 hover:text-gray-400"
              }`}
            >
              {m === "reference" ? "Library Ref" : "Inline"}
            </button>
          ))}
        </div>
      </div>

      {mode === "reference" ? (
        <>
          {/* API Tool Definition selector */}
          <div>
            <label className={labelClass}>API Tool Definition</label>
            <select
              className={inputClass}
              value={config.api_tool_id ?? ""}
              onChange={(e) =>
                update({ api_tool_id: e.target.value, endpoint: "", inline_endpoint: undefined })
              }
            >
              <option value="">Select API tool...</option>
              {apiToolDefs.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({d.endpoints.length} endpoints)
                </option>
              ))}
            </select>
          </div>

          {/* Endpoint selector */}
          {selectedDef && (
            <div>
              <label className={labelClass}>Endpoint</label>
              <select
                className={inputClass}
                value={config.endpoint ?? ""}
                onChange={(e) => update({ endpoint: e.target.value })}
              >
                <option value="">Select endpoint...</option>
                {selectedDef.endpoints.map((ep) => (
                  <option key={ep.name} value={ep.name}>
                    {ep.method} {ep.name} &mdash; {ep.description}
                  </option>
                ))}
              </select>
            </div>
          )}
        </>
      ) : (
        <>
          {/* Inline endpoint config */}
          <div>
            <label className={labelClass}>Base URL</label>
            <input
              type="text"
              className={inputClass}
              value={config.inline_endpoint?.base_url ?? ""}
              onChange={(e) =>
                update({
                  inline_endpoint: {
                    ...config.inline_endpoint,
                    base_url: e.target.value,
                  },
                  api_tool_id: undefined,
                  endpoint: undefined,
                })
              }
              placeholder="https://api.example.com"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={labelClass}>Method</label>
              <select
                className={inputClass}
                value={config.inline_endpoint?.method ?? "GET"}
                onChange={(e) =>
                  update({
                    inline_endpoint: {
                      ...config.inline_endpoint,
                      method: e.target.value,
                    },
                  })
                }
              >
                {HTTP_METHODS.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className={labelClass}>Path</label>
              <input
                type="text"
                className={inputClass}
                value={config.inline_endpoint?.path ?? ""}
                onChange={(e) =>
                  update({
                    inline_endpoint: {
                      ...config.inline_endpoint,
                      path: e.target.value,
                    },
                  })
                }
                placeholder="/endpoint"
              />
            </div>
          </div>

          <div className={sectionClass}>
            <label className={labelClass}>Input Schema (JSON)</label>
            <textarea
              className={`${inputClass} font-mono text-xs resize-y`}
              rows={3}
              value={
                config.inline_endpoint?.input_schema
                  ? JSON.stringify(config.inline_endpoint.input_schema, null, 2)
                  : ""
              }
              onChange={(e) => {
                try {
                  const schema = e.target.value ? JSON.parse(e.target.value) : undefined;
                  update({
                    inline_endpoint: { ...config.inline_endpoint, input_schema: schema },
                  });
                } catch {
                  // Let user keep typing
                }
              }}
              placeholder='{"type":"object","properties":{...}}'
            />
          </div>
        </>
      )}
    </div>
  );
};

export default ApiToolNodeForm;
