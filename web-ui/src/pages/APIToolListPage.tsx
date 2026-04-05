import { useEffect, useState } from "react";
import { useAppStore } from "../store";
import type { APIToolDefinition } from "../store";
import { fetchAPITools, deleteAPITool, createAPITool } from "../api";

export default function APIToolListPage() {
  const { serverUrl, apiKey, navigate } = useAppStore();
  const [tools, setTools] = useState<APIToolDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState<string | null>(null);

  const loadTools = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await fetchAPITools(serverUrl, apiKey);
      setTools(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load API tools");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTools();
  }, [serverUrl, apiKey]);

  const handleDelete = async (id: string) => {
    if (!confirm("Are you sure you want to delete this API tool?")) return;
    setDeleting(id);
    try {
      await deleteAPITool(serverUrl, apiKey, id);
      setTools(tools.filter((t) => t.id !== id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to delete API tool");
    } finally {
      setDeleting(null);
    }
  };

  const handleCreate = async () => {
    try {
      const tool = await createAPITool(serverUrl, apiKey, {
        name: "Untitled API Tool",
        namespace: "default",
        base_url: "https://api.example.com",
        endpoints: [
          {
            name: "example",
            description: "Example endpoint",
            method: "GET",
            path: "/example",
          },
        ],
      });
      navigate("api-tool-editor", tool.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create API tool");
    }
  };

  return (
    <div className="px-6 py-4">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="font-heading text-lg font-bold text-white">API Tools</h1>
        <button
          onClick={handleCreate}
          className="rounded-lg bg-brand-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-600"
        >
          New API Tool
        </button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-400/20 bg-red-400/10 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-12 text-[var(--text-secondary)]">
          Loading API tools...
        </div>
      ) : tools.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-[var(--text-secondary)]">
          <p className="text-sm">No API tools yet.</p>
          <p className="mt-1 text-xs text-[var(--text-tertiary)]">
            Create an API tool to define REST endpoints for LLM tool calling.
          </p>
        </div>
      ) : (
        <div className="grid gap-3">
          {tools.map((tool) => (
            <div
              key={tool.id}
              onClick={() => navigate("api-tool-editor", tool.id)}
              className="group cursor-pointer rounded-lg border border-[var(--border-primary)] bg-[var(--bg-surface)] p-4 transition-colors hover:bg-[var(--bg-surface-hover)]"
            >
              <div className="flex items-start justify-between">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-semibold text-white">
                    {tool.name}
                  </h3>
                  <div className="mt-1 flex items-center gap-3 text-xs text-[var(--text-tertiary)]">
                    <span className="font-mono">{tool.namespace}</span>
                    <span className="truncate max-w-xs">{tool.base_url}</span>
                    <span>{tool.endpoints?.length || 0} endpoints</span>
                  </div>
                  {tool.description && (
                    <p className="mt-1 text-xs text-[var(--text-tertiary)]">
                      {tool.description}
                    </p>
                  )}
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDelete(tool.id);
                  }}
                  disabled={deleting === tool.id}
                  className="ml-4 rounded px-2 py-1 text-xs text-[var(--text-tertiary)] opacity-0 transition-all hover:bg-red-400/10 hover:text-red-400 group-hover:opacity-100"
                >
                  {deleting === tool.id ? "..." : "Delete"}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
