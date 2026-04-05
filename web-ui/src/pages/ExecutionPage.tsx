import { useEffect, useState, useRef } from "react";
import { useAppStore, type Execution, type ExecutionStep } from "../store";
import { getExecution, getExecutionSteps } from "../api";

const STATUS_STYLES: Record<string, string> = {
  pending: "bg-gray-500/20 text-gray-400",
  running: "bg-brand-400/20 text-brand-400 animate-pulse",
  completed: "bg-emerald-400/20 text-emerald-400",
  failed: "bg-red-400/20 text-red-400",
  cancelled: "bg-gray-600/20 text-gray-500",
  skipped: "bg-gray-600/20 text-gray-500",
};

export default function ExecutionPage() {
  const { serverUrl, apiKey, currentExecutionId, navigate } = useAppStore();
  const [execution, setExecution] = useState<Execution | null>(null);
  const [steps, setSteps] = useState<ExecutionStep[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadExecution = async () => {
    if (!currentExecutionId) return;
    try {
      const [exec, stepData] = await Promise.all([
        getExecution(serverUrl, apiKey, currentExecutionId),
        getExecutionSteps(serverUrl, apiKey, currentExecutionId),
      ]);
      setExecution(exec);
      setSteps(stepData);

      if (exec.status !== "pending" && exec.status !== "running") {
        if (pollRef.current) {
          clearInterval(pollRef.current);
          pollRef.current = null;
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load execution");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadExecution();
    pollRef.current = setInterval(loadExecution, 2000);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [currentExecutionId]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-[var(--text-secondary)]">
        Loading execution...
      </div>
    );
  }

  if (error || !execution) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-red-400">{error || "Execution not found"}</p>
        <button
          onClick={() => navigate("graphs")}
          className="text-sm text-brand-400 hover:underline"
        >
          Back to graphs
        </button>
      </div>
    );
  }

  return (
    <div className="px-6 py-4">
      {/* Header */}
      <div className="mb-6">
        <button
          onClick={() => navigate("graphs")}
          className="mb-3 text-sm text-[var(--text-secondary)] hover:text-white"
        >
          &larr; Back to graphs
        </button>
        <div className="flex items-center gap-3">
          <h1 className="font-heading text-lg font-bold text-white">
            Execution
          </h1>
          <StatusBadge status={execution.status} />
        </div>
        <p className="mt-1 font-mono text-xs text-[var(--text-tertiary)]">
          {execution.id}
        </p>
      </div>

      {/* Input / Output */}
      <div className="mb-6 grid gap-4 md:grid-cols-2">
        <JsonPanel title="Input" data={execution.input} />
        <JsonPanel
          title="Output"
          data={execution.output}
          error={execution.error}
        />
      </div>

      {/* Steps timeline */}
      <div>
        <h2 className="mb-3 text-[11px] font-semibold uppercase tracking-widest text-[var(--text-tertiary)]">
          Steps
        </h2>
        {steps.length === 0 ? (
          <p className="text-sm text-[var(--text-tertiary)]">
            No steps recorded yet.
          </p>
        ) : (
          <div className="space-y-2">
            {steps.map((step) => (
              <StepCard key={step.id} step={step} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={`rounded-full px-2.5 py-0.5 text-[10px] font-semibold ${STATUS_STYLES[status] || STATUS_STYLES.pending}`}
    >
      {status}
    </span>
  );
}

function JsonPanel({
  title,
  data,
  error,
}: {
  title: string;
  data: Record<string, unknown> | null;
  error?: string | null;
}) {
  return (
    <div className="rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface)] p-4">
      <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-widest text-[var(--text-tertiary)]">
        {title}
      </h3>
      {error ? (
        <pre className="whitespace-pre-wrap font-mono text-xs text-red-400">
          {error}
        </pre>
      ) : data ? (
        <pre className="whitespace-pre-wrap font-mono text-xs text-[var(--text-secondary)]">
          {JSON.stringify(data, null, 2)}
        </pre>
      ) : (
        <p className="text-xs text-[var(--text-tertiary)]">--</p>
      )}
    </div>
  );
}

function StepCard({ step }: { step: ExecutionStep }) {
  return (
    <div className="rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface)] p-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-white">
            {step.node_name}
          </span>
          <StatusBadge status={step.status} />
        </div>
        {step.duration_ms != null && (
          <span className="font-mono text-xs text-[var(--text-tertiary)]">
            {step.duration_ms}ms
          </span>
        )}
      </div>

      {(step.input || step.output || step.error) && (
        <div className="mt-2 grid gap-2 md:grid-cols-2">
          {step.input && (
            <div>
              <span className="text-[10px] font-medium uppercase text-[var(--text-tertiary)]">
                Input
              </span>
              <pre className="mt-0.5 whitespace-pre-wrap font-mono text-[11px] text-[var(--text-secondary)]">
                {JSON.stringify(step.input, null, 2)}
              </pre>
            </div>
          )}
          {step.output && (
            <div>
              <span className="text-[10px] font-medium uppercase text-[var(--text-tertiary)]">
                Output
              </span>
              <pre className="mt-0.5 whitespace-pre-wrap font-mono text-[11px] text-[var(--text-secondary)]">
                {JSON.stringify(step.output, null, 2)}
              </pre>
            </div>
          )}
          {step.error && (
            <div className="md:col-span-2">
              <span className="text-[10px] font-medium uppercase text-red-400">
                Error
              </span>
              <pre className="mt-0.5 whitespace-pre-wrap font-mono text-[11px] text-red-400">
                {step.error}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
