import React, { useEffect, useState } from "react";
import type { GraphNode, GraphState, APIToolDefinition } from "../../store";
import { useAppStore } from "../../store";
import { fetchAPITools } from "../../api";
import { VariableBrowser } from "./VariableBrowser";

export interface PromptMessage {
  role: string;
  content: string;
}

export interface APIToolRefConfig {
  api_tool_id: string;
  endpoint: string;
  tool_name?: string;
}

export interface LlmConfig {
  provider?: string;
  model?: string;
  api_key?: string;
  system_prompt?: string;
  user_prompt?: string;
  messages?: PromptMessage[];
  response_format?: string;
  temperature?: number;
  max_tokens?: number;
  extra_headers?: Record<string, string>;
  api_tools?: APIToolRefConfig[];
}

interface LlmNodeFormProps {
  config: LlmConfig;
  onChange: (config: LlmConfig) => void;
  node?: GraphNode;
  graphState?: GraphState;
}

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const PROVIDERS = ["openai", "anthropic", "google", "openrouter", "bedrock"];
const RESPONSE_FORMATS = ["text", "json"];
const ROLES = ["system", "user", "assistant"];

const ROLE_COLORS: Record<string, string> = {
  system: "border-amber-500/40 bg-amber-500/10 text-amber-400",
  user: "border-brand-500/40 bg-brand-500/10 text-brand-400",
  assistant: "border-emerald-500/40 bg-emerald-500/10 text-emerald-400",
};

// Convert legacy system_prompt + user_prompt to messages array
function getMessages(config: LlmConfig): PromptMessage[] {
  if (config.messages && config.messages.length > 0) {
    return config.messages;
  }
  const msgs: PromptMessage[] = [];
  if (config.system_prompt) {
    msgs.push({ role: "system", content: config.system_prompt });
  }
  msgs.push({ role: "user", content: config.user_prompt || "" });
  return msgs;
}

const LlmNodeForm: React.FC<LlmNodeFormProps> = ({ config, onChange, node, graphState }) => {
  const { serverUrl, apiKey } = useAppStore();
  const [apiToolDefs, setApiToolDefs] = useState<APIToolDefinition[]>([]);

  useEffect(() => {
    fetchAPITools(serverUrl, apiKey).then(setApiToolDefs).catch(() => {});
  }, [serverUrl, apiKey]);

  const update = (patch: Partial<LlmConfig>) => {
    onChange({ ...config, ...patch });
  };

  const messages = getMessages(config);

  const updateMessages = (msgs: PromptMessage[]) => {
    // Also update legacy fields for backward compat
    const systemMsg = msgs.find((m) => m.role === "system");
    const userMsg = msgs.filter((m) => m.role === "user").pop();
    update({
      messages: msgs,
      system_prompt: systemMsg?.content || "",
      user_prompt: userMsg?.content || "",
    });
  };

  const updateMessage = (index: number, patch: Partial<PromptMessage>) => {
    const updated = messages.map((m, i) =>
      i === index ? { ...m, ...patch } : m
    );
    updateMessages(updated);
  };

  const addMessage = () => {
    updateMessages([...messages, { role: "user", content: "" }]);
  };

  const removeMessage = (index: number) => {
    if (messages.length <= 1) return;
    updateMessages(messages.filter((_, i) => i !== index));
  };

  const moveMessage = (index: number, direction: -1 | 1) => {
    const newIndex = index + direction;
    if (newIndex < 0 || newIndex >= messages.length) return;
    const updated = [...messages];
    [updated[index], updated[newIndex]] = [updated[newIndex], updated[index]];
    updateMessages(updated);
  };

  return (
    <div className="space-y-4">
      {/* Provider */}
      <div>
        <label className={labelClass}>Provider</label>
        <select
          className={inputClass}
          value={config.provider ?? ""}
          onChange={(e) => update({ provider: e.target.value })}
        >
          <option value="">Select provider...</option>
          {PROVIDERS.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
      </div>

      {/* Model */}
      <div>
        <label className={labelClass}>Model</label>
        <input
          type="text"
          className={inputClass}
          value={config.model ?? ""}
          onChange={(e) => update({ model: e.target.value })}
          placeholder="gpt-4o, claude-sonnet-4-20250514, etc."
        />
      </div>

      {/* API Key */}
      <div>
        <label className={labelClass}>API Key</label>
        <input
          type="text"
          className={inputClass}
          value={config.api_key ?? ""}
          onChange={(e) => update({ api_key: e.target.value })}
          placeholder="${OPENROUTER_API_KEY}"
        />
      </div>

      {/* ─── Prompt Chain ─── */}
      <div className={sectionClass}>
        <div className="flex items-center justify-between mb-3">
          <label className="text-xs font-medium text-gray-400">
            Prompt Chain
            <span className="ml-2 font-normal text-gray-600">
              ({messages.length} message{messages.length !== 1 ? "s" : ""})
            </span>
          </label>
          <button
            type="button"
            onClick={addMessage}
            className="rounded-md border border-[rgba(255,255,255,0.08)] px-2 py-1 text-[11px] font-medium text-gray-400 transition-colors hover:border-brand-500/40 hover:text-brand-400"
          >
            + Add Message
          </button>
        </div>

        <div className="space-y-3">
          {messages.map((msg, i) => (
            <div
              key={i}
              className="rounded-lg border border-[rgba(255,255,255,0.06)] bg-[#0a0a0a] p-3"
            >
              {/* Header: role selector + actions */}
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  {/* Role selector */}
                  <div className="flex gap-1">
                    {ROLES.map((role) => (
                      <button
                        key={role}
                        type="button"
                        onClick={() => updateMessage(i, { role })}
                        className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase transition-all ${
                          msg.role === role
                            ? ROLE_COLORS[role]
                            : "border-transparent text-gray-600 hover:text-gray-400"
                        }`}
                      >
                        {role}
                      </button>
                    ))}
                  </div>
                  <span className="text-[10px] text-gray-700">#{i + 1}</span>
                </div>

                {/* Actions */}
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => moveMessage(i, -1)}
                    disabled={i === 0}
                    className="rounded p-1 text-gray-600 transition-colors hover:text-gray-300 disabled:opacity-30 disabled:hover:text-gray-600"
                    title="Move up"
                  >
                    <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                      <path d="M6 2v8M3 5l3-3 3 3" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    onClick={() => moveMessage(i, 1)}
                    disabled={i === messages.length - 1}
                    className="rounded p-1 text-gray-600 transition-colors hover:text-gray-300 disabled:opacity-30 disabled:hover:text-gray-600"
                    title="Move down"
                  >
                    <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                      <path d="M6 10V2M3 7l3 3 3-3" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    onClick={() => removeMessage(i)}
                    disabled={messages.length <= 1}
                    className="rounded p-1 text-gray-600 transition-colors hover:text-red-400 disabled:opacity-30 disabled:hover:text-gray-600"
                    title="Remove message"
                  >
                    <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                      <path d="M3 3l6 6M9 3l-6 6" strokeLinecap="round" />
                    </svg>
                  </button>
                </div>
              </div>

              {/* Content textarea */}
              <textarea
                className="w-full rounded-md border border-[rgba(255,255,255,0.06)] bg-[#060608] px-3 py-2 font-mono text-xs text-gray-200 outline-none transition-colors focus:border-brand-500/30 resize-y"
                rows={msg.role === "system" ? 3 : 4}
                value={msg.content}
                onChange={(e) => updateMessage(i, { content: e.target.value })}
                placeholder={
                  msg.role === "system"
                    ? "You are a helpful assistant..."
                    : msg.role === "assistant"
                    ? "Example assistant response..."
                    : "Analyze the following: {{input.text}}"
                }
              />
            </div>
          ))}
        </div>

        <p className="mt-2 text-[11px] text-gray-600">
          Use {"{{input.var}}"}, {"{{state.field}}"}, or {"{{meta.node_id}}"} for template variables. Messages are sent to the LLM in order.
        </p>

        {node && (
          <div className="mt-3">
            <VariableBrowser node={node} graphState={graphState} />
          </div>
        )}
      </div>

      {/* Response Format */}
      <div className={sectionClass}>
        <label className={labelClass}>Response Format</label>
        <select
          className={inputClass}
          value={config.response_format ?? "text"}
          onChange={(e) => update({ response_format: e.target.value })}
        >
          {RESPONSE_FORMATS.map((f) => (
            <option key={f} value={f}>{f}</option>
          ))}
        </select>
      </div>

      {/* Temperature */}
      <div>
        <label className={labelClass}>
          Temperature{" "}
          <span className="text-gray-500 ml-1">{config.temperature ?? 0.7}</span>
        </label>
        <input
          type="range"
          className="w-full accent-brand-500"
          min={0}
          max={2}
          step={0.1}
          value={config.temperature ?? 0.7}
          onChange={(e) => update({ temperature: parseFloat(e.target.value) })}
        />
      </div>

      {/* Max Tokens */}
      <div>
        <label className={labelClass}>Max Tokens</label>
        <input
          type="number"
          className={inputClass}
          value={config.max_tokens ?? ""}
          onChange={(e) =>
            update({
              max_tokens: e.target.value ? parseInt(e.target.value, 10) : undefined,
            })
          }
          placeholder="4096"
          min={1}
        />
      </div>

      {/* API Tools */}
      <div className={sectionClass}>
        <div className="flex items-center justify-between mb-3">
          <label className="text-xs font-medium text-gray-400">
            API Tools
            {(config.api_tools?.length ?? 0) > 0 && (
              <span className="ml-2 font-normal text-gray-600">
                ({config.api_tools!.length} ref{config.api_tools!.length !== 1 ? "s" : ""})
              </span>
            )}
          </label>
          <button
            type="button"
            onClick={() => {
              const refs = config.api_tools ?? [];
              update({ api_tools: [...refs, { api_tool_id: "", endpoint: "" }] });
            }}
            className="rounded-md border border-[rgba(255,255,255,0.08)] px-2 py-1 text-[11px] font-medium text-gray-400 transition-colors hover:border-brand-500/40 hover:text-brand-400"
          >
            + Add API Tool
          </button>
        </div>

        {(config.api_tools ?? []).length === 0 ? (
          <p className="text-[10px] text-gray-600 italic">
            No API tools attached. Add one to enable LLM tool calling against REST APIs.
          </p>
        ) : (
          <div className="space-y-2">
            {(config.api_tools ?? []).map((ref, i) => {
              const selectedDef = apiToolDefs.find((d) => d.id === ref.api_tool_id);
              return (
                <div
                  key={i}
                  className="rounded-lg border border-[rgba(255,255,255,0.06)] bg-[#0a0a0a] p-3 space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-gray-600">#{i + 1}</span>
                    <button
                      type="button"
                      onClick={() => {
                        const refs = (config.api_tools ?? []).filter((_, j) => j !== i);
                        update({ api_tools: refs });
                      }}
                      className="rounded p-1 text-gray-600 transition-colors hover:text-red-400"
                    >
                      <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                        <path d="M3 3l6 6M9 3l-6 6" strokeLinecap="round" />
                      </svg>
                    </button>
                  </div>

                  {/* API Tool Definition selector */}
                  <select
                    className={inputClass}
                    value={ref.api_tool_id}
                    onChange={(e) => {
                      const refs = [...(config.api_tools ?? [])];
                      refs[i] = { ...refs[i], api_tool_id: e.target.value, endpoint: "" };
                      update({ api_tools: refs });
                    }}
                  >
                    <option value="">Select API tool...</option>
                    {apiToolDefs.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name} ({d.endpoints.length} endpoints)
                      </option>
                    ))}
                  </select>

                  {/* Endpoint selector */}
                  {selectedDef && (
                    <select
                      className={inputClass}
                      value={ref.endpoint}
                      onChange={(e) => {
                        const refs = [...(config.api_tools ?? [])];
                        refs[i] = { ...refs[i], endpoint: e.target.value };
                        update({ api_tools: refs });
                      }}
                    >
                      <option value="">Select endpoint...</option>
                      {selectedDef.endpoints.map((ep) => (
                        <option key={ep.name} value={ep.name}>
                          {ep.method} {ep.name}
                        </option>
                      ))}
                    </select>
                  )}

                  {/* Optional tool name override */}
                  {ref.endpoint && (
                    <input
                      type="text"
                      className={inputClass}
                      value={ref.tool_name ?? ""}
                      onChange={(e) => {
                        const refs = [...(config.api_tools ?? [])];
                        refs[i] = { ...refs[i], tool_name: e.target.value || undefined };
                        update({ api_tools: refs });
                      }}
                      placeholder={`Tool name override (default: ${ref.endpoint})`}
                    />
                  )}
                </div>
              );
            })}
          </div>
        )}

        <p className="mt-2 text-[10px] text-gray-600">
          Selected endpoints become callable tools for this LLM node.
        </p>
      </div>
    </div>
  );
};

export default LlmNodeForm;
