import { useEffect, useState } from "react";
import { useAppStore } from "../store";
import type { APIToolDefinition, APIEndpoint, HeaderConfig, APIToolTestResult } from "../store";
import { getAPITool, updateAPITool, testAPIToolEndpoint } from "../api";
import { useToast } from "../components/Toast";

const inputClass =
  "w-full bg-[#0a0a0a] border border-[rgba(255,255,255,0.08)] rounded-lg px-3 py-2 text-sm text-gray-200 focus:border-brand-500/50 focus:outline-none focus:ring-1 focus:ring-brand-500/50";
const labelClass = "text-xs font-medium text-gray-400 mb-1.5 block";
const sectionClass = "border-t border-[rgba(255,255,255,0.06)] pt-4 mt-4";

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];
const REQUEST_MAPPING_MODES = ["json_body", "form", "query_params", "path_and_body"];
const RESPONSE_MAPPING_MODES = ["json_body", "text", "jq", "headers_and_body"];

export default function APIToolEditorPage() {
  const { serverUrl, apiKey, currentAPIToolId, navigate } = useAppStore();
  const { showToast } = useToast();

  const [tool, setTool] = useState<APIToolDefinition | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [isDirty, setIsDirty] = useState(false);
  const [error, setError] = useState("");

  // Endpoint editing
  const [selectedEndpointIdx, setSelectedEndpointIdx] = useState<number | null>(null);

  // Test panel
  const [testEndpointName, setTestEndpointName] = useState("");
  const [testInput, setTestInput] = useState("{}");
  const [testResult, setTestResult] = useState<APIToolTestResult | null>(null);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (!currentAPIToolId) return;
    setLoading(true);
    getAPITool(serverUrl, apiKey, currentAPIToolId)
      .then((data) => {
        setTool(data);
        if (data.endpoints?.length > 0) {
          setSelectedEndpointIdx(0);
          setTestEndpointName(data.endpoints[0].name);
        }
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setLoading(false));
  }, [currentAPIToolId]);

  const updateTool = (patch: Partial<APIToolDefinition>) => {
    if (!tool) return;
    setTool({ ...tool, ...patch });
    setIsDirty(true);
  };

  const handleSave = async () => {
    if (!tool) return;
    setSaving(true);
    try {
      const updated = await updateAPITool(serverUrl, apiKey, tool.id, {
        name: tool.name,
        description: tool.description,
        base_url: tool.base_url,
        default_headers: tool.default_headers,
        default_timeout_ms: tool.default_timeout_ms,
        retry: tool.retry,
        endpoints: tool.endpoints,
      });
      setTool(updated);
      setIsDirty(false);
      showToast("Saved", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "Save failed", "error");
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!tool || !testEndpointName) return;
    setTesting(true);
    setTestResult(null);
    try {
      let parsedInput: Record<string, unknown> = {};
      if (testInput.trim()) {
        parsedInput = JSON.parse(testInput);
      }
      const result = await testAPIToolEndpoint(
        serverUrl,
        apiKey,
        tool.id,
        testEndpointName,
        parsedInput
      );
      setTestResult(result);
    } catch (e) {
      setTestResult({
        success: false,
        error: e instanceof Error ? e.message : "Test failed",
        duration_ms: 0,
      });
    } finally {
      setTesting(false);
    }
  };

  // Keyboard shortcut for save
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        if (isDirty && tool) handleSave();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isDirty, tool]);

  // --- Endpoint CRUD helpers ---
  const addEndpoint = () => {
    if (!tool) return;
    const name = `endpoint_${tool.endpoints.length + 1}`;
    const ep: APIEndpoint = {
      name,
      description: "New endpoint",
      method: "GET",
      path: "/",
    };
    const endpoints = [...tool.endpoints, ep];
    updateTool({ endpoints });
    setSelectedEndpointIdx(endpoints.length - 1);
  };

  const removeEndpoint = (idx: number) => {
    if (!tool || tool.endpoints.length <= 1) return;
    const endpoints = tool.endpoints.filter((_, i) => i !== idx);
    updateTool({ endpoints });
    if (selectedEndpointIdx !== null) {
      if (selectedEndpointIdx >= endpoints.length) {
        setSelectedEndpointIdx(endpoints.length - 1);
      } else if (selectedEndpointIdx === idx) {
        setSelectedEndpointIdx(Math.max(0, idx - 1));
      }
    }
  };

  const updateEndpoint = (idx: number, patch: Partial<APIEndpoint>) => {
    if (!tool) return;
    const endpoints = tool.endpoints.map((ep, i) =>
      i === idx ? { ...ep, ...patch } : ep
    );
    updateTool({ endpoints });
  };

  // --- Header helpers ---
  const updateDefaultHeaders = (headers: HeaderConfig[]) => {
    updateTool({ default_headers: headers });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12 text-[var(--text-secondary)]">
        Loading API tool...
      </div>
    );
  }

  if (error || !tool) {
    return (
      <div className="px-6 py-4">
        <div className="mb-4 rounded-lg border border-red-400/20 bg-red-400/10 px-4 py-3 text-sm text-red-400">
          {error || "API tool not found"}
        </div>
        <button
          onClick={() => navigate("api-tools")}
          className="text-sm text-brand-400 hover:text-brand-300"
        >
          Back to API Tools
        </button>
      </div>
    );
  }

  const selectedEndpoint =
    selectedEndpointIdx !== null ? tool.endpoints[selectedEndpointIdx] : null;

  return (
    <div className="flex h-full">
      {/* Left sidebar: endpoint list */}
      <aside className="flex w-56 shrink-0 flex-col border-r border-[rgba(255,255,255,0.08)] bg-[#111318]">
        {/* Back + title */}
        <div className="p-3 border-b border-[rgba(255,255,255,0.08)]">
          <button
            onClick={() => navigate("api-tools")}
            className="mb-2 text-xs text-[var(--text-tertiary)] hover:text-brand-400 transition-colors flex items-center gap-1"
          >
            <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
            </svg>
            API Tools
          </button>
          <input
            type="text"
            className="w-full bg-transparent text-sm font-semibold text-white outline-none border-b border-transparent focus:border-brand-500/30 pb-1"
            value={tool.name}
            onChange={(e) => updateTool({ name: e.target.value })}
          />
        </div>

        {/* Endpoint list */}
        <div className="flex-1 overflow-y-auto p-3">
          <div className="mb-2 text-xs font-medium uppercase tracking-wider text-gray-500">
            Endpoints ({tool.endpoints.length})
          </div>
          <div className="space-y-1">
            {tool.endpoints.map((ep, i) => (
              <button
                key={i}
                onClick={() => {
                  setSelectedEndpointIdx(i);
                  setTestEndpointName(ep.name);
                }}
                className={`w-full text-left rounded-lg px-2.5 py-2 text-xs transition-colors ${
                  selectedEndpointIdx === i
                    ? "bg-brand-500/10 text-brand-400"
                    : "text-gray-400 hover:bg-[var(--bg-surface-hover)] hover:text-white"
                }`}
              >
                <span className={`inline-block w-10 font-mono text-[10px] font-bold ${methodColor(ep.method)}`}>
                  {ep.method}
                </span>
                <span className="ml-1">{ep.name}</span>
              </button>
            ))}
          </div>
          <button
            onClick={addEndpoint}
            className="mt-2 text-xs text-brand-400 hover:text-brand-300 transition-colors"
          >
            + Add Endpoint
          </button>
        </div>
      </aside>

      {/* Main content area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <div className="flex items-center justify-between border-b border-[rgba(255,255,255,0.08)] px-4 py-2.5">
          <div className="text-xs text-[var(--text-tertiary)]">
            <span className="font-mono">{tool.id}</span>
          </div>
          <button
            onClick={handleSave}
            disabled={!isDirty || saving}
            className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-colors ${
              isDirty
                ? "bg-brand-500 text-white hover:bg-brand-600"
                : "bg-gray-700 text-gray-500 cursor-not-allowed"
            }`}
          >
            {saving ? "Saving..." : isDirty ? "Save" : "Saved"}
          </button>
        </div>

        {/* Scrollable content: split into definition + endpoint detail + test */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">
          {/* Definition-level fields */}
          <section>
            <h2 className="text-xs font-medium uppercase tracking-wider text-gray-500 mb-3">
              Definition
            </h2>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className={labelClass}>Base URL</label>
                <input
                  type="text"
                  className={inputClass}
                  value={tool.base_url}
                  onChange={(e) => updateTool({ base_url: e.target.value })}
                  placeholder="https://api.example.com/v1"
                />
              </div>
              <div>
                <label className={labelClass}>Description</label>
                <input
                  type="text"
                  className={inputClass}
                  value={tool.description ?? ""}
                  onChange={(e) => updateTool({ description: e.target.value })}
                  placeholder="What this API does..."
                />
              </div>
              <div>
                <label className={labelClass}>Default Timeout (ms)</label>
                <input
                  type="number"
                  className={inputClass}
                  value={tool.default_timeout_ms ?? ""}
                  onChange={(e) =>
                    updateTool({
                      default_timeout_ms: e.target.value
                        ? parseInt(e.target.value, 10)
                        : undefined,
                    })
                  }
                  placeholder="30000"
                />
              </div>
              <div>
                <label className={labelClass}>Namespace</label>
                <input
                  type="text"
                  className={inputClass}
                  value={tool.namespace}
                  onChange={(e) => updateTool({ namespace: e.target.value })}
                />
              </div>
            </div>

            {/* Default headers */}
            <div className={sectionClass}>
              <HeaderEditor
                label="Default Headers"
                headers={tool.default_headers ?? []}
                onChange={updateDefaultHeaders}
              />
            </div>

            {/* Retry config */}
            <div className={sectionClass}>
              <label className="text-xs font-medium text-gray-400 mb-2 block">
                Retry Config
              </label>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="text-[10px] text-gray-500 block mb-1">
                    Max Retries
                  </label>
                  <input
                    type="number"
                    className={inputClass}
                    value={tool.retry?.max_retries ?? ""}
                    onChange={(e) =>
                      updateTool({
                        retry: {
                          max_retries: parseInt(e.target.value, 10) || 0,
                          backoff_ms: tool.retry?.backoff_ms ?? 1000,
                          retry_on_status: tool.retry?.retry_on_status,
                        },
                      })
                    }
                    placeholder="3"
                    min={0}
                  />
                </div>
                <div>
                  <label className="text-[10px] text-gray-500 block mb-1">
                    Backoff (ms)
                  </label>
                  <input
                    type="number"
                    className={inputClass}
                    value={tool.retry?.backoff_ms ?? ""}
                    onChange={(e) =>
                      updateTool({
                        retry: {
                          max_retries: tool.retry?.max_retries ?? 0,
                          backoff_ms: parseInt(e.target.value, 10) || 1000,
                          retry_on_status: tool.retry?.retry_on_status,
                        },
                      })
                    }
                    placeholder="1000"
                    min={0}
                  />
                </div>
                <div>
                  <label className="text-[10px] text-gray-500 block mb-1">
                    Retry On (comma-sep)
                  </label>
                  <input
                    type="text"
                    className={inputClass}
                    value={tool.retry?.retry_on_status?.join(", ") ?? ""}
                    onChange={(e) =>
                      updateTool({
                        retry: {
                          max_retries: tool.retry?.max_retries ?? 0,
                          backoff_ms: tool.retry?.backoff_ms ?? 1000,
                          retry_on_status: e.target.value
                            .split(",")
                            .map((s) => parseInt(s.trim(), 10))
                            .filter((n) => !isNaN(n)),
                        },
                      })
                    }
                    placeholder="429, 500, 502, 503"
                  />
                </div>
              </div>
            </div>
          </section>

          {/* Selected endpoint detail */}
          {selectedEndpoint && selectedEndpointIdx !== null && (
            <section className={sectionClass}>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-xs font-medium uppercase tracking-wider text-gray-500">
                  Endpoint: {selectedEndpoint.name}
                </h2>
                <button
                  onClick={() => removeEndpoint(selectedEndpointIdx)}
                  disabled={tool.endpoints.length <= 1}
                  className="text-[10px] text-red-400 hover:text-red-300 disabled:opacity-30 transition-colors"
                >
                  Remove
                </button>
              </div>

              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className={labelClass}>Name</label>
                    <input
                      type="text"
                      className={inputClass}
                      value={selectedEndpoint.name}
                      onChange={(e) =>
                        updateEndpoint(selectedEndpointIdx, {
                          name: e.target.value,
                        })
                      }
                    />
                  </div>
                  <div>
                    <label className={labelClass}>Method</label>
                    <select
                      className={inputClass}
                      value={selectedEndpoint.method}
                      onChange={(e) =>
                        updateEndpoint(selectedEndpointIdx, {
                          method: e.target.value,
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
                </div>

                <div>
                  <label className={labelClass}>Path</label>
                  <input
                    type="text"
                    className={inputClass}
                    value={selectedEndpoint.path}
                    onChange={(e) =>
                      updateEndpoint(selectedEndpointIdx, {
                        path: e.target.value,
                      })
                    }
                    placeholder="/users/{{input.user_id}}"
                  />
                  <p className="mt-1 text-[10px] text-gray-600">
                    {"Use {{input.field}} for path parameters"}
                  </p>
                </div>

                <div>
                  <label className={labelClass}>Description</label>
                  <textarea
                    className={`${inputClass} resize-y`}
                    rows={2}
                    value={selectedEndpoint.description}
                    onChange={(e) =>
                      updateEndpoint(selectedEndpointIdx, {
                        description: e.target.value,
                      })
                    }
                    placeholder="What this endpoint does (shown to LLMs)..."
                  />
                </div>

                {/* Input schema */}
                <div>
                  <label className={labelClass}>Input Schema (JSON)</label>
                  <textarea
                    className={`${inputClass} font-mono text-xs resize-y`}
                    rows={4}
                    value={
                      selectedEndpoint.input_schema
                        ? JSON.stringify(selectedEndpoint.input_schema, null, 2)
                        : ""
                    }
                    onChange={(e) => {
                      try {
                        const schema = e.target.value
                          ? JSON.parse(e.target.value)
                          : undefined;
                        updateEndpoint(selectedEndpointIdx, {
                          input_schema: schema,
                        });
                      } catch {
                        // Let user keep typing
                      }
                    }}
                    placeholder='{"type":"object","properties":{...}}'
                  />
                </div>

                {/* Output schema */}
                <div>
                  <label className={labelClass}>Output Schema (JSON)</label>
                  <textarea
                    className={`${inputClass} font-mono text-xs resize-y`}
                    rows={3}
                    value={
                      selectedEndpoint.output_schema
                        ? JSON.stringify(selectedEndpoint.output_schema, null, 2)
                        : ""
                    }
                    onChange={(e) => {
                      try {
                        const schema = e.target.value
                          ? JSON.parse(e.target.value)
                          : undefined;
                        updateEndpoint(selectedEndpointIdx, {
                          output_schema: schema,
                        });
                      } catch {
                        // Let user keep typing
                      }
                    }}
                    placeholder='{"type":"object","properties":{...}}'
                  />
                </div>

                {/* Request/Response mapping */}
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className={labelClass}>Request Mapping</label>
                    <select
                      className={inputClass}
                      value={selectedEndpoint.request_mapping?.mode ?? "json_body"}
                      onChange={(e) =>
                        updateEndpoint(selectedEndpointIdx, {
                          request_mapping: { mode: e.target.value },
                        })
                      }
                    >
                      {REQUEST_MAPPING_MODES.map((m) => (
                        <option key={m} value={m}>
                          {m}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className={labelClass}>Response Mapping</label>
                    <select
                      className={inputClass}
                      value={selectedEndpoint.response_mapping?.mode ?? "json_body"}
                      onChange={(e) =>
                        updateEndpoint(selectedEndpointIdx, {
                          response_mapping: {
                            mode: e.target.value,
                            expression:
                              selectedEndpoint.response_mapping?.expression,
                          },
                        })
                      }
                    >
                      {RESPONSE_MAPPING_MODES.map((m) => (
                        <option key={m} value={m}>
                          {m}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                {selectedEndpoint.response_mapping?.mode === "jq" && (
                  <div>
                    <label className={labelClass}>JQ Expression</label>
                    <input
                      type="text"
                      className={inputClass}
                      value={selectedEndpoint.response_mapping?.expression ?? ""}
                      onChange={(e) =>
                        updateEndpoint(selectedEndpointIdx, {
                          response_mapping: {
                            mode: "jq",
                            expression: e.target.value,
                          },
                        })
                      }
                      placeholder=".data.items"
                    />
                  </div>
                )}

                {/* Endpoint-level headers */}
                <HeaderEditor
                  label="Endpoint Headers"
                  headers={selectedEndpoint.headers ?? []}
                  onChange={(headers) =>
                    updateEndpoint(selectedEndpointIdx, { headers })
                  }
                />
              </div>
            </section>
          )}

          {/* Test panel */}
          <section className={sectionClass}>
            <h2 className="text-xs font-medium uppercase tracking-wider text-gray-500 mb-3">
              Test Endpoint
            </h2>
            <div className="rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0a0a0a] p-4 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className={labelClass}>Endpoint</label>
                  <select
                    className={inputClass}
                    value={testEndpointName}
                    onChange={(e) => setTestEndpointName(e.target.value)}
                  >
                    {tool.endpoints.map((ep) => (
                      <option key={ep.name} value={ep.name}>
                        {ep.method} {ep.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="flex items-end">
                  <button
                    onClick={handleTest}
                    disabled={testing || !testEndpointName}
                    className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700 disabled:opacity-50"
                  >
                    {testing ? "Testing..." : "Run Test"}
                  </button>
                </div>
              </div>
              <div>
                <label className={labelClass}>Input (JSON)</label>
                <textarea
                  className={`${inputClass} font-mono text-xs resize-y`}
                  rows={3}
                  value={testInput}
                  onChange={(e) => setTestInput(e.target.value)}
                  placeholder='{"user_id": "123"}'
                />
              </div>
              {testResult && (
                <div
                  className={`rounded-lg border p-3 text-xs ${
                    testResult.success
                      ? "border-emerald-500/20 bg-emerald-500/5"
                      : "border-red-500/20 bg-red-500/5"
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <span
                      className={`font-medium ${
                        testResult.success
                          ? "text-emerald-400"
                          : "text-red-400"
                      }`}
                    >
                      {testResult.success ? "Success" : "Error"}
                    </span>
                    <span className="text-gray-500">
                      {testResult.duration_ms}ms
                    </span>
                  </div>
                  {testResult.error && (
                    <pre className="text-red-300 whitespace-pre-wrap break-all mb-2">
                      {testResult.error}
                    </pre>
                  )}
                  {testResult.result !== undefined && (
                    <pre className="text-gray-300 whitespace-pre-wrap break-all max-h-64 overflow-y-auto font-mono">
                      {typeof testResult.result === "string"
                        ? testResult.result
                        : JSON.stringify(testResult.result, null, 2)}
                    </pre>
                  )}
                </div>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

// --- Header Editor Component ---

function HeaderEditor({
  label,
  headers,
  onChange,
}: {
  label: string;
  headers: HeaderConfig[];
  onChange: (headers: HeaderConfig[]) => void;
}) {
  const addHeader = () => {
    onChange([...headers, { name: "", value: "" }]);
  };

  const removeHeader = (idx: number) => {
    onChange(headers.filter((_, i) => i !== idx));
  };

  const updateHeader = (idx: number, patch: Partial<HeaderConfig>) => {
    onChange(headers.map((h, i) => (i === idx ? { ...h, ...patch } : h)));
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <label className="text-xs font-medium text-gray-400">{label}</label>
        <button
          onClick={addHeader}
          className="text-[10px] text-brand-400 hover:text-brand-300 transition-colors"
        >
          + Add
        </button>
      </div>
      {headers.length === 0 ? (
        <p className="text-[10px] text-gray-600 italic">No headers configured</p>
      ) : (
        <div className="space-y-2">
          {headers.map((h, i) => (
            <div key={i} className="flex items-center gap-2">
              <input
                type="text"
                className={`${inputClass} flex-1`}
                value={h.name}
                onChange={(e) => updateHeader(i, { name: e.target.value })}
                placeholder="Header-Name"
              />
              <input
                type="text"
                className={`${inputClass} flex-1`}
                value={h.value ?? h.secret_ref ?? ""}
                onChange={(e) => updateHeader(i, { value: e.target.value })}
                placeholder="Value or {{secret.ref}}"
              />
              <button
                onClick={() => removeHeader(i)}
                className="text-gray-600 hover:text-red-400 transition-colors shrink-0"
              >
                <svg
                  className="h-3.5 w-3.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function methodColor(method: string): string {
  switch (method) {
    case "GET":
      return "text-emerald-400";
    case "POST":
      return "text-blue-400";
    case "PUT":
      return "text-amber-400";
    case "PATCH":
      return "text-orange-400";
    case "DELETE":
      return "text-red-400";
    default:
      return "text-gray-400";
  }
}
