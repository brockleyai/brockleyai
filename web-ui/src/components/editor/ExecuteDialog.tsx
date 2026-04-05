import { useState, useEffect } from "react";
import type { GraphNode, Execution, ExecutionStep } from "../../store";
import { getExecutionSteps } from "../../api";

interface ExecuteDialogProps {
  graphId: string;
  nodes: GraphNode[];
  serverUrl: string;
  apiKey: string;
  onClose: () => void;
  onExecute: (input: Record<string, unknown>, mode: string) => Promise<Execution | null>;
}

function buildDefaultInput(nodes: GraphNode[]): Record<string, unknown> {
  const inputNode = nodes.find((n) => n.type === "input");
  if (!inputNode) return {};

  const ports = inputNode.output_ports || inputNode.input_ports || [];
  const result: Record<string, unknown> = {};

  for (const port of ports) {
    const schema = port.schema || {};
    result[port.name] = buildDefaultFromSchema(schema);
  }

  return result;
}

function buildDefaultFromSchema(schema: Record<string, unknown>): unknown {
  const type = schema.type as string;
  switch (type) {
    case "string":
      return "";
    case "number":
    case "integer":
      return 0;
    case "boolean":
      return false;
    case "array":
      return [];
    case "object": {
      const props = schema.properties as Record<string, Record<string, unknown>> | undefined;
      if (!props) return {};
      const obj: Record<string, unknown> = {};
      for (const [key, propSchema] of Object.entries(props)) {
        obj[key] = buildDefaultFromSchema(propSchema);
      }
      return obj;
    }
    default:
      return null;
  }
}

export default function ExecuteDialog({ graphId, nodes, serverUrl, apiKey, onClose, onExecute }: ExecuteDialogProps) {
  const defaultInput = buildDefaultInput(nodes);
  const [jsonInput, setJsonInput] = useState(JSON.stringify(defaultInput, null, 2));
  const [mode, setMode] = useState<"sync" | "async">("sync");
  const [parseError, setParseError] = useState<string | null>(null);
  const [executing, setExecuting] = useState(false);
  const [result, setResult] = useState<Execution | null>(null);
  const [steps, setSteps] = useState<ExecutionStep[]>([]);
  const [error, setError] = useState<string | null>(null);

  // Reset when opened
  useEffect(() => {
    setResult(null);
    setSteps([]);
    setError(null);
    setExecuting(false);
  }, [graphId]);

  // Fetch steps when execution completes
  useEffect(() => {
    if (!result || (result.status !== "completed" && result.status !== "failed")) return;
    getExecutionSteps(serverUrl, apiKey, result.id)
      .then(setSteps)
      .catch(() => {});
  }, [result, serverUrl, apiKey]);

  async function handleExecute() {
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(jsonInput);
      setParseError(null);
    } catch (e) {
      setParseError(e instanceof SyntaxError ? e.message : "Invalid JSON");
      return;
    }

    setExecuting(true);
    setError(null);
    setResult(null);

    try {
      const exec = await onExecute(parsed, mode);
      if (exec) {
        setResult(exec);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Execution failed");
    } finally {
      setExecuting(false);
    }
  }

  const isComplete = result && (result.status === "completed" || result.status === "failed");

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#111318] shadow-2xl">
        {/* Header */}
        <div className="sticky top-0 z-10 flex items-center justify-between border-b border-[rgba(255,255,255,0.08)] bg-[#111318] px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold text-white">Execute Graph</h2>
            <p className="mt-0.5 text-xs text-gray-500">{graphId}</p>
          </div>
          <button
            onClick={onClose}
            className="flex h-7 w-7 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-white/5 hover:text-white"
          >
            &times;
          </button>
        </div>

        {/* Body */}
        <div className="space-y-4 px-5 py-4">
          {/* Input JSON */}
          <div>
            <label className="mb-1.5 block text-xs font-medium text-gray-400">
              Input JSON
              <span className="ml-2 font-normal text-gray-600">
                (pre-populated from input node ports)
              </span>
            </label>
            <textarea
              value={jsonInput}
              onChange={(e) => { setJsonInput(e.target.value); setParseError(null); }}
              rows={Math.min(12, Math.max(4, jsonInput.split("\n").length + 1))}
              spellCheck={false}
              disabled={executing}
              className="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0a0a0a] p-3 font-mono text-xs text-gray-200 outline-none transition-colors focus:border-brand-400/50 disabled:opacity-50"
            />
            {parseError && (
              <p className="mt-1.5 text-[11px] text-red-400">{parseError}</p>
            )}
          </div>

          {/* Mode selector */}
          <div>
            <label className="mb-1.5 block text-xs font-medium text-gray-400">
              Execution Mode
            </label>
            <div className="flex gap-2">
              {(["sync", "async"] as const).map((m) => (
                <button
                  key={m}
                  onClick={() => setMode(m)}
                  disabled={executing}
                  className={`rounded-lg border px-4 py-2 text-xs font-medium transition-colors ${
                    mode === m
                      ? "border-brand-400 bg-brand-500/20 text-brand-400"
                      : "border-[rgba(255,255,255,0.08)] text-gray-400 hover:text-gray-200"
                  } disabled:opacity-50`}
                >
                  {m === "sync" ? "Sync (wait for result)" : "Async (fire & forget)"}
                </button>
              ))}
            </div>
          </div>

          {/* Error */}
          {error && (
            <div className="rounded-lg border border-red-400/20 bg-red-400/10 px-4 py-3">
              <p className="text-xs font-medium text-red-400">Execution failed</p>
              <p className="mt-1 text-xs text-red-300">{error}</p>
            </div>
          )}

          {/* Result */}
          {result && (
            <div className="space-y-3">
              {/* Status */}
              <div className="flex items-center gap-3">
                <span className="text-xs font-medium text-gray-400">Status:</span>
                <span className={`rounded-md px-2 py-0.5 text-[11px] font-semibold uppercase ${
                  result.status === "completed" ? "bg-emerald-400/20 text-emerald-400" :
                  result.status === "failed" ? "bg-red-400/20 text-red-400" :
                  result.status === "running" ? "bg-brand-400/20 text-brand-400 animate-pulse" :
                  "bg-gray-400/20 text-gray-400"
                }`}>
                  {result.status}
                </span>
                {result.started_at && result.completed_at && (
                  <span className="text-[11px] text-gray-600">
                    {((new Date(result.completed_at).getTime() - new Date(result.started_at).getTime()) / 1000).toFixed(1)}s
                  </span>
                )}
              </div>

              {/* Output */}
              {result.output && Object.keys(result.output).length > 0 && (
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-emerald-400">
                    Output
                  </label>
                  <pre className="max-h-[300px] overflow-auto rounded-lg border border-emerald-400/20 bg-emerald-400/5 p-3 font-mono text-xs text-gray-200">
                    {JSON.stringify(result.output, null, 2)}
                  </pre>
                </div>
              )}

              {/* Error details */}
              {result.error && (
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-red-400">
                    Error
                  </label>
                  <pre className="rounded-lg border border-red-400/20 bg-red-400/5 p-3 font-mono text-xs text-red-300">
                    {typeof result.error === "object" ? JSON.stringify(result.error, null, 2) : String(result.error)}
                  </pre>
                </div>
              )}

              {/* Step outputs */}
              {steps.length > 0 && (
                <div className="space-y-2">
                  <label className="block text-xs font-medium text-gray-400">
                    Node Results
                  </label>
                  {steps
                    .filter((s) => s.status === "completed" || s.status === "failed")
                    .map((step) => {
                    const nodeName = nodes.find((n) => n.id === step.node_id)?.name || step.node_id;
                    return (
                    <div
                      key={step.id}
                      className="rounded-lg border border-[rgba(255,255,255,0.06)] bg-[#0d0f14] p-3"
                    >
                      <div className="mb-1.5 flex items-center gap-2">
                        <span className="text-xs font-medium text-gray-300">
                          {nodeName}
                        </span>
                        <span className="text-[10px] text-gray-600">{step.node_type}</span>
                        <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${
                          step.status === "completed" ? "bg-emerald-400/10 text-emerald-400" :
                          step.status === "failed" ? "bg-red-400/10 text-red-400" :
                          "bg-gray-400/10 text-gray-400"
                        }`}>
                          {step.status}
                        </span>
                        {step.duration_ms != null && (
                          <span className="text-[10px] text-gray-600">{step.duration_ms}ms</span>
                        )}
                      </div>
                      {step.output && Object.keys(step.output).length > 0 && (
                        <pre className="max-h-[200px] overflow-auto rounded border border-[rgba(255,255,255,0.04)] bg-[#080a0e] p-2 font-mono text-[11px] text-gray-300">
                          {JSON.stringify(step.output, null, 2)}
                        </pre>
                      )}
                      {step.error && (
                        <pre className="mt-1 rounded border border-red-400/10 bg-red-400/5 p-2 font-mono text-[11px] text-red-300">
                          {typeof step.error === "object" ? JSON.stringify(step.error, null, 2) : String(step.error)}
                        </pre>
                      )}
                    </div>
                  );
                  })}
                </div>
              )}

              {/* Execution ID */}
              <p className="text-[11px] text-gray-600">
                Execution ID: {result.id}
              </p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="sticky bottom-0 flex items-center justify-end gap-2 border-t border-[rgba(255,255,255,0.08)] bg-[#111318] px-5 py-3">
          <button
            onClick={onClose}
            className="rounded-lg border border-[rgba(255,255,255,0.08)] px-4 py-2 text-xs font-medium text-gray-400 transition-colors hover:text-white"
          >
            {isComplete ? "Close" : "Cancel"}
          </button>
          {!isComplete && (
            <button
              onClick={handleExecute}
              disabled={executing}
              className="rounded-lg bg-brand-500 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-brand-600 disabled:opacity-50"
            >
              {executing ? (
                <span className="flex items-center gap-2">
                  <span className="h-3 w-3 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                  Executing...
                </span>
              ) : "Execute"}
            </button>
          )}
          {isComplete && (
            <button
              onClick={() => { setResult(null); setError(null); }}
              className="rounded-lg bg-brand-500 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-brand-600"
            >
              Run Again
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
