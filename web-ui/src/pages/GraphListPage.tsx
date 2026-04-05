import { useEffect, useState } from "react";
import { useAppStore } from "../store";
import { fetchGraphs, deleteGraph, createGraph, updateGraph } from "../api";

export default function GraphListPage() {
  const { serverUrl, apiKey, graphs, setGraphs, navigate } = useAppStore();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState<string | null>(null);

  const loadGraphs = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await fetchGraphs(serverUrl, apiKey);
      setGraphs(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load graphs");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadGraphs();
  }, [serverUrl, apiKey]);

  const handleDelete = async (id: string) => {
    if (!confirm("Are you sure you want to delete this graph?")) return;
    setDeleting(id);
    try {
      await deleteGraph(serverUrl, apiKey, id);
      setGraphs(graphs.filter((g) => g.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to delete graph");
    } finally {
      setDeleting(null);
    }
  };

  const handleToggleStatus = async (graphId: string, currentStatus: string) => {
    const newStatus = currentStatus === "active" ? "draft" : "active";
    try {
      const updated = await updateGraph(serverUrl, apiKey, graphId, { status: newStatus });
      setGraphs(graphs.map((g) => (g.id === graphId ? updated : g)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update status");
    }
  };

  const handleCreate = async () => {
    try {
      const graph = await createGraph(serverUrl, apiKey, {
        name: "Untitled Graph",
        namespace: "default",
        version: "0.1.0",
        nodes: [],
        edges: [],
      });
      navigate("graph-editor", graph.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create graph");
    }
  };

  return (
    <div className="px-6 py-4">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="font-heading text-lg font-bold text-white">Graphs</h1>
        <button
          onClick={handleCreate}
          className="rounded-lg bg-brand-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-600"
        >
          New Graph
        </button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-400/20 bg-red-400/10 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-12 text-[var(--text-secondary)]">
          Loading graphs...
        </div>
      ) : graphs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-[var(--text-secondary)]">
          <p className="text-sm">No graphs yet.</p>
          <p className="mt-1 text-xs text-[var(--text-tertiary)]">
            Create your first graph to get started.
          </p>
        </div>
      ) : (
        <div className="grid gap-3">
          {graphs.map((graph) => (
            <div
              key={graph.id}
              onClick={() => navigate("graph-editor", graph.id)}
              className="group cursor-pointer rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface)] p-4 transition-colors hover:bg-[var(--bg-surface-hover)]"
            >
              <div className="flex items-start justify-between">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-semibold text-white">
                    {graph.name}
                  </h3>
                  <div className="mt-1 flex items-center gap-3 text-xs text-[var(--text-tertiary)]">
                    <span className="font-mono">{graph.namespace}</span>
                    <span>v{graph.version}</span>
                    <span>{graph.nodes?.length || 0} nodes</span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleToggleStatus(graph.id, graph.status);
                      }}
                      title={`Click to mark as ${graph.status === "active" ? "draft" : "active"}`}
                      className="cursor-pointer transition-opacity hover:opacity-80"
                    >
                      <StatusBadge status={graph.status} />
                    </button>
                  </div>
                  <p className="mt-1 text-xs text-[var(--text-tertiary)]">
                    Created{" "}
                    {new Date(graph.created_at).toLocaleDateString()}
                  </p>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDelete(graph.id);
                  }}
                  disabled={deleting === graph.id}
                  className="ml-4 rounded px-2 py-1 text-xs text-[var(--text-tertiary)] opacity-0 transition-all hover:bg-red-400/10 hover:text-red-400 group-hover:opacity-100"
                >
                  {deleting === graph.id ? "..." : "Delete"}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    draft: "bg-gray-500/20 text-gray-400",
    active: "bg-emerald-400/20 text-emerald-400",
    archived: "bg-gray-600/20 text-gray-500",
  };

  return (
    <span
      className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${colors[status] || colors.draft}`}
    >
      {status}
    </span>
  );
}
