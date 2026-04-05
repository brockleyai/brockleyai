import type { ReactNode } from "react";
import { useAppStore } from "../store";
import logo from "../assets/logo.svg";

export default function Layout({ children }: { children: ReactNode }) {
  const { serverUrl, currentPage, navigate, disconnect } = useAppStore();

  const navItems: { label: string; page: "graphs" | "api-tools" }[] = [
    { label: "Graphs", page: "graphs" },
    { label: "API Tools", page: "api-tools" },
  ];

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="flex w-60 flex-col border-r border-[var(--border-primary)] bg-[var(--bg-surface)]">
        {/* Logo + disconnect */}
        <div className="flex h-14 items-center justify-between border-b border-[var(--border-primary)] pl-6 pr-1.5">
          <img src={logo} alt="Brockley" className="h-5 w-auto" />
          <button
            onClick={() => {
              if (window.confirm("Disconnect from server?")) disconnect();
            }}
            title="Disconnect"
            className="rounded p-1 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-surface-hover)] hover:text-red-400"
          >
            <svg className="h-6 w-6 opacity-60" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-3">
          {navItems.map((item) => {
            const active = currentPage === item.page;
            return (
              <button
                key={item.page}
                onClick={() => navigate(item.page)}
                className={`mb-0.5 flex w-full items-center rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? "bg-brand-500/10 text-brand-400"
                    : "text-[var(--text-secondary)] hover:bg-[var(--bg-surface-hover)] hover:text-white"
                }`}
              >
                <NavIcon page={item.page} />
                <span className="ml-2.5">{item.label}</span>
              </button>
            );
          })}
        </nav>

        {/* Connection info */}
        <div className="border-t border-[var(--border-primary)] px-4 py-3">
          <div className="flex items-center gap-2">
            <div className="h-2 w-2 rounded-full bg-emerald-400" />
            <span className="truncate text-xs text-[var(--text-secondary)]">
              {serverUrl.replace(/^https?:\/\//, "")}
            </span>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-[var(--bg-primary)]">
        {children}
      </main>
    </div>
  );
}

function NavIcon({ page }: { page: string }) {
  if (page === "graphs") {
    return (
      <svg
        className="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"
        />
      </svg>
    );
  }
  if (page === "api-tools") {
    return (
      <svg
        className="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M13 10V3L4 14h7v7l9-11h-7z"
        />
      </svg>
    );
  }
  return null;
}
