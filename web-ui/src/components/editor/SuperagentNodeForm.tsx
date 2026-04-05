import React, { useEffect, useState } from "react";
import type { GraphNode, GraphState, APIToolDefinition } from "../../store";
import { useAppStore } from "../../store";
import { fetchAPITools } from "../../api";
import { VariableBrowser } from "./VariableBrowser";

// ─── Config Types (mirrors Go SuperagentNodeConfig JSON shape) ───

export interface HeaderConfig {
  name: string;
  value: string;
}

export interface SuperagentSkillConfig {
  name: string;
  description: string;
  mcp_url?: string;
  mcp_transport?: string;
  api_tool_id?: string;
  endpoints?: string[];
  headers?: HeaderConfig[];
  prompt_fragment?: string;
  tools?: string[];
  timeout_seconds?: number;
}

export interface SharedMemoryConfig {
  enabled: boolean;
  namespace?: string;
  inject_on_start?: boolean;
  auto_flush?: boolean;
}

export interface ToolPoliciesConfig {
  allowed?: string[];
  denied?: string[];
  require_approval?: string[];
}

export interface EvaluatorOverrideConfig {
  provider?: string;
  model?: string;
  api_key?: string;
  api_key_ref?: string;
  prompt?: string;
  disabled?: boolean;
}

export interface ReflectionOverrideConfig {
  provider?: string;
  model?: string;
  api_key?: string;
  api_key_ref?: string;
  prompt?: string;
  max_reflections?: number;
  disabled?: boolean;
}

export interface ContextCompactionOverrideConfig {
  enabled?: boolean;
  provider?: string;
  model?: string;
  api_key?: string;
  api_key_ref?: string;
  prompt?: string;
  context_window_limit?: number;
  compaction_threshold?: number;
  preserve_recent_messages?: number;
}

export interface StuckDetectionOverrideConfig {
  enabled?: boolean;
  window_size?: number;
  repeat_threshold?: number;
}

export interface PromptAssemblyOverrideConfig {
  template?: string;
  tool_conventions?: string;
  style?: string;
}

export interface OutputExtractionOverrideConfig {
  prompt?: string;
  provider?: string;
  model?: string;
  api_key?: string;
  api_key_ref?: string;
}

export interface TaskTrackingOverrideConfig {
  enabled?: boolean;
  reminder_frequency?: number;
}

export interface SuperagentOverridesConfig {
  evaluator?: EvaluatorOverrideConfig;
  reflection?: ReflectionOverrideConfig;
  context_compaction?: ContextCompactionOverrideConfig;
  stuck_detection?: StuckDetectionOverrideConfig;
  prompt_assembly?: PromptAssemblyOverrideConfig;
  output_extraction?: OutputExtractionOverrideConfig;
  task_tracking?: TaskTrackingOverrideConfig;
}

export interface SuperagentConfig {
  prompt?: string;
  skills?: SuperagentSkillConfig[];
  provider?: string;
  model?: string;
  api_key?: string;
  api_key_ref?: string;
  base_url?: string;
  system_preamble?: string;
  max_iterations?: number;
  max_total_tool_calls?: number;
  max_tool_calls_per_iteration?: number;
  max_tool_loop_rounds?: number;
  timeout_seconds?: number;
  temperature?: number;
  max_tokens?: number;
  shared_memory?: SharedMemoryConfig;
  conversation_history_from_input?: string;
  tool_policies?: ToolPoliciesConfig;
  overrides?: SuperagentOverridesConfig;
}

// ─── Props ───

interface SuperagentNodeFormProps {
  config: SuperagentConfig;
  onChange: (config: SuperagentConfig) => void;
  node?: GraphNode;
  graphState?: GraphState;
}

// ─── Shared constants ───

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const PROVIDERS = ["openai", "anthropic", "google", "openrouter", "bedrock", "custom"];
const MCP_TRANSPORTS = ["http", "sse", "stdio"];

// ─── Collapsible Section helper ───

function CollapsibleSection({
  title,
  defaultOpen,
  badge,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  badge?: string;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen ?? false);
  return (
    <div className={sectionClass}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between mb-3"
      >
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          {title}
          {badge && (
            <span className="ml-2 font-normal text-gray-600 normal-case">
              ({badge})
            </span>
          )}
        </span>
        <svg
          className={`h-3 w-3 text-gray-400 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && <div className="space-y-3">{children}</div>}
    </div>
  );
}

// ─── LLM Override Fields (reused by evaluator, reflection, compaction, extraction) ───

function LLMOverrideFields({
  config,
  onChange,
}: {
  config: Record<string, unknown> | undefined;
  onChange: (patch: Record<string, unknown>) => void;
}) {
  const c = config ?? {};
  return (
    <>
      <div>
        <label className={labelClass}>Provider</label>
        <select
          className={inputClass}
          value={(c.provider as string) ?? ""}
          onChange={(e) => onChange({ provider: e.target.value || undefined })}
        >
          <option value="">Inherit from main</option>
          {PROVIDERS.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
      </div>
      <div>
        <label className={labelClass}>Model</label>
        <input
          type="text"
          className={inputClass}
          value={(c.model as string) ?? ""}
          onChange={(e) => onChange({ model: e.target.value || undefined })}
          placeholder="Inherit from main"
        />
      </div>
      <div>
        <label className={labelClass}>API Key</label>
        <input
          type="text"
          className={inputClass}
          value={(c.api_key as string) ?? ""}
          onChange={(e) => onChange({ api_key: e.target.value || undefined })}
          placeholder="Inherit from main"
        />
      </div>
    </>
  );
}

// ─── Main Component ───

const SuperagentNodeForm: React.FC<SuperagentNodeFormProps> = ({
  config,
  onChange,
  node,
  graphState,
}) => {
  const { serverUrl, apiKey } = useAppStore();
  const [apiToolDefs, setApiToolDefs] = useState<APIToolDefinition[]>([]);

  useEffect(() => {
    fetchAPITools(serverUrl, apiKey).then(setApiToolDefs).catch(() => {});
  }, [serverUrl, apiKey]);

  const update = (patch: Partial<SuperagentConfig>) => {
    onChange({ ...config, ...patch });
  };

  // ─── Skill helpers ───
  const skills = config.skills ?? [];

  const updateSkill = (index: number, patch: Partial<SuperagentSkillConfig>) => {
    const updated = skills.map((s, i) => (i === index ? { ...s, ...patch } : s));
    update({ skills: updated });
  };

  const addSkill = () => {
    update({ skills: [...skills, { name: "", description: "" }] });
  };

  const removeSkill = (index: number) => {
    update({ skills: skills.filter((_, i) => i !== index) });
  };

  // ─── Override helpers ───
  const overrides = config.overrides ?? {};

  const updateOverride = <K extends keyof SuperagentOverridesConfig>(
    key: K,
    patch: Partial<NonNullable<SuperagentOverridesConfig[K]>>
  ) => {
    update({
      overrides: {
        ...overrides,
        [key]: { ...(overrides[key] ?? {}), ...patch },
      },
    });
  };

  // ─── Shared memory helper ───
  const sharedMemory = config.shared_memory ?? { enabled: false };
  const updateSharedMemory = (patch: Partial<SharedMemoryConfig>) => {
    update({ shared_memory: { ...sharedMemory, ...patch } });
  };

  // ─── Tool policies helper ───
  const toolPolicies = config.tool_policies ?? {};
  const updateToolPolicies = (patch: Partial<ToolPoliciesConfig>) => {
    update({ tool_policies: { ...toolPolicies, ...patch } });
  };

  // Parse comma-separated string into string array
  const parseList = (val: string): string[] =>
    val.split(",").map((s) => s.trim()).filter(Boolean);

  return (
    <div className="space-y-4">
      {/* ═══════ Section A: Core LLM Settings ═══════ */}

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

      {/* API Key Ref */}
      <div>
        <label className={labelClass}>API Key Ref</label>
        <input
          type="text"
          className={inputClass}
          value={config.api_key_ref ?? ""}
          onChange={(e) => update({ api_key_ref: e.target.value || undefined })}
          placeholder="Secret store reference (alternative to inline key)"
        />
      </div>

      {/* Base URL */}
      <div>
        <label className={labelClass}>Base URL</label>
        <input
          type="text"
          className={inputClass}
          value={config.base_url ?? ""}
          onChange={(e) => update({ base_url: e.target.value || undefined })}
          placeholder="Custom provider base URL"
        />
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
            update({ max_tokens: e.target.value ? parseInt(e.target.value, 10) : undefined })
          }
          placeholder="4096"
          min={1}
        />
      </div>

      {/* ═══════ Section B: Prompt ═══════ */}
      <div className={sectionClass}>
        <label className={labelClass}>Prompt</label>
        <textarea
          className={`${inputClass} font-mono resize-y`}
          rows={6}
          value={config.prompt ?? ""}
          onChange={(e) => update({ prompt: e.target.value })}
          placeholder="Describe the agent's task. Use {{input.var}} for template variables."
        />
        <p className="mt-1.5 text-[11px] text-gray-600">
          Use {"{{input.var}}"}, {"{{state.field}}"}, or {"{{meta.node_id}}"} for template variables.
        </p>
      </div>

      {/* System Preamble */}
      <div>
        <label className={labelClass}>System Preamble</label>
        <textarea
          className={`${inputClass} font-mono resize-y`}
          rows={3}
          value={config.system_preamble ?? ""}
          onChange={(e) => update({ system_preamble: e.target.value || undefined })}
          placeholder="Persona, tone, and guardrails prepended to the system prompt..."
        />
      </div>

      {/* Conversation History */}
      <div>
        <label className={labelClass}>Conversation History Input Port</label>
        <input
          type="text"
          className={inputClass}
          value={config.conversation_history_from_input ?? ""}
          onChange={(e) => update({ conversation_history_from_input: e.target.value || undefined })}
          placeholder="Input port name for prior conversation history"
        />
      </div>

      {/* Variable Browser */}
      {node && (
        <div className="mt-3">
          <VariableBrowser node={node} graphState={graphState} />
        </div>
      )}

      {/* ═══════ Section C: Skills ═══════ */}
      <CollapsibleSection
        title="Skills"
        defaultOpen={skills.length > 0}
        badge={skills.length > 0 ? `${skills.length}` : undefined}
      >
        {skills.length === 0 ? (
          <p className="text-[10px] text-gray-600 italic">
            No skills configured. Add a skill to provide tools to the agent.
          </p>
        ) : (
          <div className="space-y-3">
            {skills.map((skill, i) => {
              const isApiToolMode = !!skill.api_tool_id;
              const selectedDef = apiToolDefs.find((d) => d.id === skill.api_tool_id);

              return (
                <div
                  key={i}
                  className="rounded-lg border border-[rgba(255,255,255,0.06)] bg-[#0a0a0a] p-3 space-y-2"
                >
                  {/* Skill header */}
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-gray-600">
                      #{i + 1}{skill.name ? ` — ${skill.name}` : ""}
                    </span>
                    <button
                      type="button"
                      onClick={() => removeSkill(i)}
                      className="rounded p-1 text-gray-600 transition-colors hover:text-red-400"
                    >
                      <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                        <path d="M3 3l6 6M9 3l-6 6" strokeLinecap="round" />
                      </svg>
                    </button>
                  </div>

                  {/* Name */}
                  <div>
                    <label className={labelClass}>Name</label>
                    <input
                      type="text"
                      className={inputClass}
                      value={skill.name ?? ""}
                      onChange={(e) => updateSkill(i, { name: e.target.value })}
                      placeholder="Skill name"
                    />
                  </div>

                  {/* Description */}
                  <div>
                    <label className={labelClass}>Description</label>
                    <input
                      type="text"
                      className={inputClass}
                      value={skill.description ?? ""}
                      onChange={(e) => updateSkill(i, { description: e.target.value })}
                      placeholder="What this skill provides"
                    />
                  </div>

                  {/* Mode toggle */}
                  <div className="flex gap-1">
                    <button
                      type="button"
                      onClick={() => updateSkill(i, { mcp_url: skill.mcp_url ?? "", api_tool_id: undefined, endpoints: undefined })}
                      className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase transition-all ${
                        !isApiToolMode
                          ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-400"
                          : "border-transparent text-gray-600 hover:text-gray-400"
                      }`}
                    >
                      MCP
                    </button>
                    <button
                      type="button"
                      onClick={() => updateSkill(i, { api_tool_id: skill.api_tool_id ?? "", mcp_url: undefined })}
                      className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase transition-all ${
                        isApiToolMode
                          ? "border-sky-500/40 bg-sky-500/10 text-sky-400"
                          : "border-transparent text-gray-600 hover:text-gray-400"
                      }`}
                    >
                      API Tool
                    </button>
                  </div>

                  {/* MCP mode fields */}
                  {!isApiToolMode && (
                    <>
                      <div>
                        <label className={labelClass}>MCP URL</label>
                        <input
                          type="text"
                          className={inputClass}
                          value={skill.mcp_url ?? ""}
                          onChange={(e) => updateSkill(i, { mcp_url: e.target.value })}
                          placeholder="http://localhost:8080/mcp"
                        />
                      </div>
                      <div>
                        <label className={labelClass}>Transport</label>
                        <select
                          className={inputClass}
                          value={skill.mcp_transport ?? "http"}
                          onChange={(e) => updateSkill(i, { mcp_transport: e.target.value })}
                        >
                          {MCP_TRANSPORTS.map((t) => (
                            <option key={t} value={t}>{t}</option>
                          ))}
                        </select>
                      </div>
                      <div>
                        <label className={labelClass}>Tool Allowlist</label>
                        <input
                          type="text"
                          className={inputClass}
                          value={(skill.tools ?? []).join(", ")}
                          onChange={(e) => updateSkill(i, { tools: parseList(e.target.value) })}
                          placeholder="Comma-separated tool names (empty = all)"
                        />
                      </div>
                    </>
                  )}

                  {/* API Tool mode fields */}
                  {isApiToolMode && (
                    <>
                      <div>
                        <label className={labelClass}>API Tool</label>
                        <select
                          className={inputClass}
                          value={skill.api_tool_id ?? ""}
                          onChange={(e) => updateSkill(i, { api_tool_id: e.target.value, endpoints: [] })}
                        >
                          <option value="">Select API tool...</option>
                          {apiToolDefs.map((d) => (
                            <option key={d.id} value={d.id}>
                              {d.name} ({d.endpoints.length} endpoints)
                            </option>
                          ))}
                        </select>
                      </div>
                      {selectedDef && (
                        <div>
                          <label className={labelClass}>Endpoints</label>
                          <div className="space-y-1">
                            {selectedDef.endpoints.map((ep) => {
                              const checked = (skill.endpoints ?? []).includes(ep.name);
                              return (
                                <label key={ep.name} className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    className="accent-brand-500"
                                    checked={checked}
                                    onChange={() => {
                                      const current = skill.endpoints ?? [];
                                      const next = checked
                                        ? current.filter((n) => n !== ep.name)
                                        : [...current, ep.name];
                                      updateSkill(i, { endpoints: next });
                                    }}
                                  />
                                  <span>{ep.method} {ep.name}</span>
                                </label>
                              );
                            })}
                          </div>
                        </div>
                      )}
                    </>
                  )}

                  {/* Prompt Fragment */}
                  <div>
                    <label className={labelClass}>Prompt Fragment</label>
                    <textarea
                      className={`${inputClass} font-mono resize-y`}
                      rows={2}
                      value={skill.prompt_fragment ?? ""}
                      onChange={(e) => updateSkill(i, { prompt_fragment: e.target.value || undefined })}
                      placeholder="Extra context injected into system prompt for this skill"
                    />
                  </div>

                  {/* Headers */}
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <label className={labelClass}>Headers</label>
                      <button
                        type="button"
                        onClick={() => {
                          const headers = [...(skill.headers ?? []), { name: "", value: "" }];
                          updateSkill(i, { headers });
                        }}
                        className="text-[10px] text-gray-500 hover:text-gray-300 transition-colors"
                      >
                        + Add
                      </button>
                    </div>
                    {(skill.headers ?? []).map((h, hi) => (
                      <div key={hi} className="flex gap-1 mb-1">
                        <input
                          type="text"
                          className={`${inputClass} flex-1`}
                          value={h.name}
                          onChange={(e) => {
                            const headers = [...(skill.headers ?? [])];
                            headers[hi] = { ...headers[hi], name: e.target.value };
                            updateSkill(i, { headers });
                          }}
                          placeholder="Header name"
                        />
                        <input
                          type="text"
                          className={`${inputClass} flex-1`}
                          value={h.value}
                          onChange={(e) => {
                            const headers = [...(skill.headers ?? [])];
                            headers[hi] = { ...headers[hi], value: e.target.value };
                            updateSkill(i, { headers });
                          }}
                          placeholder="Value"
                        />
                        <button
                          type="button"
                          onClick={() => {
                            const headers = (skill.headers ?? []).filter((_, j) => j !== hi);
                            updateSkill(i, { headers });
                          }}
                          className="rounded p-1 text-gray-600 hover:text-red-400 transition-colors"
                        >
                          <svg className="h-3 w-3" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
                            <path d="M3 3l6 6M9 3l-6 6" strokeLinecap="round" />
                          </svg>
                        </button>
                      </div>
                    ))}
                  </div>

                  {/* Timeout */}
                  <div>
                    <label className={labelClass}>Timeout (seconds)</label>
                    <input
                      type="number"
                      className={inputClass}
                      value={skill.timeout_seconds ?? ""}
                      onChange={(e) =>
                        updateSkill(i, { timeout_seconds: e.target.value ? parseInt(e.target.value, 10) : undefined })
                      }
                      placeholder="30"
                      min={1}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <button
          type="button"
          onClick={addSkill}
          className="mt-2 rounded-md border border-[rgba(255,255,255,0.08)] px-2 py-1 text-[11px] font-medium text-gray-400 transition-colors hover:border-brand-500/40 hover:text-brand-400"
        >
          + Add Skill
        </button>
      </CollapsibleSection>

      {/* ═══════ Section D: Execution Limits ═══════ */}
      <CollapsibleSection title="Execution Limits">
        <div>
          <label className={labelClass}>Max Iterations</label>
          <input
            type="number"
            className={inputClass}
            value={config.max_iterations ?? ""}
            onChange={(e) => update({ max_iterations: e.target.value ? parseInt(e.target.value, 10) : undefined })}
            placeholder="25"
            min={1}
          />
        </div>
        <div>
          <label className={labelClass}>Max Total Tool Calls</label>
          <input
            type="number"
            className={inputClass}
            value={config.max_total_tool_calls ?? ""}
            onChange={(e) => update({ max_total_tool_calls: e.target.value ? parseInt(e.target.value, 10) : undefined })}
            placeholder="200"
            min={1}
          />
        </div>
        <div>
          <label className={labelClass}>Max Tool Calls Per Iteration</label>
          <input
            type="number"
            className={inputClass}
            value={config.max_tool_calls_per_iteration ?? ""}
            onChange={(e) => update({ max_tool_calls_per_iteration: e.target.value ? parseInt(e.target.value, 10) : undefined })}
            placeholder="25"
            min={1}
          />
        </div>
        <div>
          <label className={labelClass}>Max Tool Loop Rounds</label>
          <input
            type="number"
            className={inputClass}
            value={config.max_tool_loop_rounds ?? ""}
            onChange={(e) => update({ max_tool_loop_rounds: e.target.value ? parseInt(e.target.value, 10) : undefined })}
            placeholder="10"
            min={1}
          />
        </div>
        <div>
          <label className={labelClass}>Timeout (seconds)</label>
          <input
            type="number"
            className={inputClass}
            value={config.timeout_seconds ?? ""}
            onChange={(e) => update({ timeout_seconds: e.target.value ? parseInt(e.target.value, 10) : undefined })}
            placeholder="600"
            min={1}
          />
        </div>
      </CollapsibleSection>

      {/* ═══════ Section E: Shared Memory ═══════ */}
      <CollapsibleSection title="Shared Memory" defaultOpen={sharedMemory.enabled}>
        <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
          <input
            type="checkbox"
            className="accent-brand-500"
            checked={sharedMemory.enabled}
            onChange={(e) => updateSharedMemory({ enabled: e.target.checked })}
          />
          Enable shared memory
        </label>
        {sharedMemory.enabled && (
          <>
            <div>
              <label className={labelClass}>Namespace</label>
              <input
                type="text"
                className={inputClass}
                value={sharedMemory.namespace ?? ""}
                onChange={(e) => updateSharedMemory({ namespace: e.target.value || undefined })}
                placeholder="Default: node ID"
              />
            </div>
            <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
              <input
                type="checkbox"
                className="accent-brand-500"
                checked={sharedMemory.inject_on_start ?? true}
                onChange={(e) => updateSharedMemory({ inject_on_start: e.target.checked })}
              />
              Inject on start
            </label>
            <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
              <input
                type="checkbox"
                className="accent-brand-500"
                checked={sharedMemory.auto_flush ?? true}
                onChange={(e) => updateSharedMemory({ auto_flush: e.target.checked })}
              />
              Auto-flush before compaction
            </label>
            <p className="text-[10px] text-gray-600">
              Requires <code className="text-gray-500">_superagent_memory</code> state field with merge reducer.
            </p>
          </>
        )}
      </CollapsibleSection>

      {/* ═══════ Section F: Tool Policies ═══════ */}
      <CollapsibleSection title="Tool Policies">
        <div>
          <label className={labelClass}>Allowed</label>
          <input
            type="text"
            className={inputClass}
            value={(toolPolicies.allowed ?? []).join(", ")}
            onChange={(e) => updateToolPolicies({ allowed: parseList(e.target.value) })}
            placeholder="Comma-separated tool names (empty = all allowed)"
          />
        </div>
        <div>
          <label className={labelClass}>Denied</label>
          <input
            type="text"
            className={inputClass}
            value={(toolPolicies.denied ?? []).join(", ")}
            onChange={(e) => updateToolPolicies({ denied: parseList(e.target.value) })}
            placeholder="Comma-separated tool names to deny"
          />
        </div>
        <div>
          <label className={labelClass}>Require Approval</label>
          <input
            type="text"
            className={inputClass}
            value={(toolPolicies.require_approval ?? []).join(", ")}
            onChange={(e) => updateToolPolicies({ require_approval: parseList(e.target.value) })}
            placeholder="Comma-separated tool names requiring approval"
          />
        </div>
      </CollapsibleSection>

      {/* ═══════ Section G: Advanced Overrides ═══════ */}
      <CollapsibleSection title="Advanced Overrides">

        {/* Evaluator */}
        <CollapsibleSection title="Evaluator">
          <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              className="accent-brand-500"
              checked={!(overrides.evaluator?.disabled ?? false)}
              onChange={(e) => updateOverride("evaluator", { disabled: !e.target.checked })}
            />
            Enabled
          </label>
          <LLMOverrideFields
            config={overrides.evaluator as Record<string, unknown> | undefined}
            onChange={(patch) => updateOverride("evaluator", patch as Partial<EvaluatorOverrideConfig>)}
          />
          <div>
            <label className={labelClass}>Custom Prompt</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={3}
              value={overrides.evaluator?.prompt ?? ""}
              onChange={(e) => updateOverride("evaluator", { prompt: e.target.value || undefined })}
              placeholder="Custom evaluation prompt"
            />
          </div>
        </CollapsibleSection>

        {/* Reflection */}
        <CollapsibleSection title="Reflection">
          <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              className="accent-brand-500"
              checked={!(overrides.reflection?.disabled ?? false)}
              onChange={(e) => updateOverride("reflection", { disabled: !e.target.checked })}
            />
            Enabled
          </label>
          <LLMOverrideFields
            config={overrides.reflection as Record<string, unknown> | undefined}
            onChange={(patch) => updateOverride("reflection", patch as Partial<ReflectionOverrideConfig>)}
          />
          <div>
            <label className={labelClass}>Custom Prompt</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={3}
              value={overrides.reflection?.prompt ?? ""}
              onChange={(e) => updateOverride("reflection", { prompt: e.target.value || undefined })}
              placeholder="Custom reflection prompt"
            />
          </div>
          <div>
            <label className={labelClass}>Max Reflections</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.reflection?.max_reflections ?? ""}
              onChange={(e) =>
                updateOverride("reflection", {
                  max_reflections: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="3"
              min={1}
            />
          </div>
        </CollapsibleSection>

        {/* Context Compaction */}
        <CollapsibleSection title="Context Compaction">
          <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              className="accent-brand-500"
              checked={overrides.context_compaction?.enabled ?? true}
              onChange={(e) => updateOverride("context_compaction", { enabled: e.target.checked })}
            />
            Enabled
          </label>
          <LLMOverrideFields
            config={overrides.context_compaction as Record<string, unknown> | undefined}
            onChange={(patch) => updateOverride("context_compaction", patch as Partial<ContextCompactionOverrideConfig>)}
          />
          <div>
            <label className={labelClass}>Custom Prompt</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={3}
              value={overrides.context_compaction?.prompt ?? ""}
              onChange={(e) => updateOverride("context_compaction", { prompt: e.target.value || undefined })}
              placeholder="Custom compaction prompt"
            />
          </div>
          <div>
            <label className={labelClass}>Context Window Limit</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.context_compaction?.context_window_limit ?? ""}
              onChange={(e) =>
                updateOverride("context_compaction", {
                  context_window_limit: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="Auto-detect"
              min={1}
            />
          </div>
          <div>
            <label className={labelClass}>
              Compaction Threshold{" "}
              <span className="text-gray-500 ml-1">{overrides.context_compaction?.compaction_threshold ?? 0.75}</span>
            </label>
            <input
              type="range"
              className="w-full accent-brand-500"
              min={0.1}
              max={1}
              step={0.05}
              value={overrides.context_compaction?.compaction_threshold ?? 0.75}
              onChange={(e) =>
                updateOverride("context_compaction", { compaction_threshold: parseFloat(e.target.value) })
              }
            />
          </div>
          <div>
            <label className={labelClass}>Preserve Recent Messages</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.context_compaction?.preserve_recent_messages ?? ""}
              onChange={(e) =>
                updateOverride("context_compaction", {
                  preserve_recent_messages: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="Auto"
              min={0}
            />
          </div>
        </CollapsibleSection>

        {/* Stuck Detection */}
        <CollapsibleSection title="Stuck Detection">
          <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              className="accent-brand-500"
              checked={overrides.stuck_detection?.enabled ?? true}
              onChange={(e) => updateOverride("stuck_detection", { enabled: e.target.checked })}
            />
            Enabled
          </label>
          <div>
            <label className={labelClass}>Window Size</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.stuck_detection?.window_size ?? ""}
              onChange={(e) =>
                updateOverride("stuck_detection", {
                  window_size: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="Default"
              min={1}
            />
          </div>
          <div>
            <label className={labelClass}>Repeat Threshold</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.stuck_detection?.repeat_threshold ?? ""}
              onChange={(e) =>
                updateOverride("stuck_detection", {
                  repeat_threshold: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="Default"
              min={1}
            />
          </div>
        </CollapsibleSection>

        {/* Prompt Assembly */}
        <CollapsibleSection title="Prompt Assembly">
          <div>
            <label className={labelClass}>Template</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={4}
              value={overrides.prompt_assembly?.template ?? ""}
              onChange={(e) => updateOverride("prompt_assembly", { template: e.target.value || undefined })}
              placeholder="Custom system prompt template"
            />
          </div>
          <div>
            <label className={labelClass}>Tool Conventions</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={3}
              value={overrides.prompt_assembly?.tool_conventions ?? ""}
              onChange={(e) => updateOverride("prompt_assembly", { tool_conventions: e.target.value || undefined })}
              placeholder="Custom tool usage conventions"
            />
          </div>
          <div>
            <label className={labelClass}>Style</label>
            <input
              type="text"
              className={inputClass}
              value={overrides.prompt_assembly?.style ?? ""}
              onChange={(e) => updateOverride("prompt_assembly", { style: e.target.value || undefined })}
              placeholder="Prompt style"
            />
          </div>
        </CollapsibleSection>

        {/* Output Extraction */}
        <CollapsibleSection title="Output Extraction">
          <LLMOverrideFields
            config={overrides.output_extraction as Record<string, unknown> | undefined}
            onChange={(patch) => updateOverride("output_extraction", patch as Partial<OutputExtractionOverrideConfig>)}
          />
          <div>
            <label className={labelClass}>Custom Prompt</label>
            <textarea
              className={`${inputClass} font-mono resize-y`}
              rows={3}
              value={overrides.output_extraction?.prompt ?? ""}
              onChange={(e) => updateOverride("output_extraction", { prompt: e.target.value || undefined })}
              placeholder="Custom extraction prompt"
            />
          </div>
        </CollapsibleSection>

        {/* Task Tracking */}
        <CollapsibleSection title="Task Tracking">
          <label className="flex items-center gap-2 text-xs text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              className="accent-brand-500"
              checked={overrides.task_tracking?.enabled ?? true}
              onChange={(e) => updateOverride("task_tracking", { enabled: e.target.checked })}
            />
            Enabled
          </label>
          <div>
            <label className={labelClass}>Reminder Frequency</label>
            <input
              type="number"
              className={inputClass}
              value={overrides.task_tracking?.reminder_frequency ?? ""}
              onChange={(e) =>
                updateOverride("task_tracking", {
                  reminder_frequency: e.target.value ? parseInt(e.target.value, 10) : undefined,
                })
              }
              placeholder="Default"
              min={1}
            />
          </div>
        </CollapsibleSection>
      </CollapsibleSection>
    </div>
  );
};

export default SuperagentNodeForm;
