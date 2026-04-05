import { useState } from "react";
import { useAppStore } from "../store";
import { checkHealth } from "../api";

export default function ConnectionPage() {
  const setConnection = useAppStore((s) => s.setConnection);
  const [url, setUrl] = useState("");
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [testing, setTesting] = useState(false);

  const handleConnect = async () => {
    if (!url.trim()) {
      setError("Server URL is required");
      return;
    }
    setTesting(true);
    setError("");

    const healthy = await checkHealth(url.trim(), key.trim());
    setTesting(false);

    if (healthy) {
      setConnection(url.trim(), key.trim());
    } else {
      setError("Could not connect to server. Check the URL and API key (leave key empty for local dev).");
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-md rounded-xl border border-[var(--border-primary)] bg-[var(--bg-surface)] p-8 shadow-[0_8px_32px_-4px_rgba(0,0,0,0.6)]">
        <div className="mb-8 text-center">
          <h1 className="font-logo text-3xl font-bold tracking-tight text-white">
            Brockley
          </h1>
          <p className="mt-2 text-sm text-[var(--text-secondary)]">
            Connect to your Brockley server
          </p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-[var(--text-secondary)]">
              Server URL
            </label>
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://brockley.example.com"
              className="w-full rounded-lg border border-[var(--border-primary)] bg-[var(--bg-primary)] px-3 py-2 text-sm text-white placeholder-[var(--text-tertiary)] outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium text-[var(--text-secondary)]">
              API Key <span className="font-normal text-[var(--text-tertiary)]">(optional for local dev)</span>
            </label>
            <input
              type="password"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="Leave empty for local dev"
              className="w-full rounded-lg border border-[var(--border-primary)] bg-[var(--bg-primary)] px-3 py-2 text-sm text-white placeholder-[var(--text-tertiary)] outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
            />
          </div>

          {error && (
            <p className="text-sm text-red-400">{error}</p>
          )}

          <button
            onClick={handleConnect}
            disabled={testing}
            className="w-full rounded-lg bg-brand-500 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-brand-600 disabled:opacity-50"
          >
            {testing ? "Connecting..." : "Connect"}
          </button>
        </div>
      </div>
    </div>
  );
}
