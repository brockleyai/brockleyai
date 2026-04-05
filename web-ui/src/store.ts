import { create } from "zustand";

// ─── API Types matching actual Brockley schema ───

export interface Port {
  name: string;
  schema: Record<string, unknown>;
  required?: boolean;
  default?: unknown;
}

export interface StateBinding {
  state_field: string;
  port: string;
}

export interface StateField {
  name: string;
  schema: Record<string, unknown>;
  reducer: "replace" | "append" | "merge";
  initial?: unknown;
}

export interface GraphState {
  fields: StateField[];
}

export interface GraphNode {
  id: string;
  type: string;
  name: string;
  input_ports?: Port[];
  output_ports?: Port[];
  config: Record<string, unknown>;
  position?: { x: number; y: number };
  retry_policy?: Record<string, unknown>;
  timeout_seconds?: number;
  metadata?: Record<string, unknown>;
  state_reads?: StateBinding[];
  state_writes?: StateBinding[];
}

export interface GraphEdge {
  id: string;
  source_node_id: string;
  source_port: string;
  target_node_id: string;
  target_port: string;
  back_edge?: boolean;
  condition?: string;
  max_iterations?: number;
}

export interface Graph {
  id: string;
  tenant_id?: string;
  name: string;
  description?: string;
  namespace: string;
  version: number;
  status: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
  state?: GraphState;
  metadata?: unknown;
  created_at: string;
  updated_at: string;
}

export interface Execution {
  id: string;
  graph_id: string;
  graph_version?: number;
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  input: Record<string, unknown>;
  output: Record<string, unknown> | null;
  error: { code: string; message: string } | null;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export interface ExecutionStep {
  id: string;
  execution_id: string;
  node_id: string;
  node_type: string;
  iteration: number;
  status: "pending" | "running" | "completed" | "failed" | "skipped";
  input: Record<string, unknown> | null;
  output: Record<string, unknown> | null;
  error: Record<string, unknown> | null;
  attempt: number;
  duration_ms: number | null;
  created_at: string;
}

export interface ValidationResult {
  valid: boolean;
  errors: Array<{
    code: string;
    message: string;
    node_id?: string;
    edge_id?: string;
  }>;
  warnings?: Array<{
    code: string;
    message: string;
    node_id?: string;
  }>;
}

// ─── API Tool Types ───

export interface HeaderConfig {
  name: string;
  value?: string;
  from_input?: string;
  secret_ref?: string;
}

export interface RetryConfig {
  max_retries: number;
  backoff_ms: number;
  retry_on_status?: number[];
}

export interface RequestMapping {
  mode: string; // "json_body" | "form" | "query_params" | "path_and_body"
}

export interface ResponseMapping {
  mode: string; // "json_body" | "text" | "jq" | "headers_and_body"
  expression?: string;
}

export interface APIEndpoint {
  name: string;
  description: string;
  method: string;
  path: string;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  headers?: HeaderConfig[];
  request_mapping?: RequestMapping;
  response_mapping?: ResponseMapping;
  timeout_ms?: number;
}

export interface APIToolDefinition {
  id: string;
  tenant_id?: string;
  name: string;
  namespace: string;
  description?: string;
  base_url: string;
  default_headers?: HeaderConfig[];
  default_timeout_ms?: number;
  retry?: RetryConfig;
  endpoints: APIEndpoint[];
  metadata?: unknown;
  created_at: string;
  updated_at: string;
}

export interface APIToolTestResult {
  success: boolean;
  result?: unknown;
  error?: string;
  is_error?: boolean;
  duration_ms: number;
}

// ─── App State ───

interface AppState {
  // Connection
  serverUrl: string;
  apiKey: string;
  isConnected: boolean;
  setConnection: (url: string, key: string) => void;
  disconnect: () => void;

  // Navigation
  currentPage: "graphs" | "graph-editor" | "execution" | "api-tools" | "api-tool-editor";
  currentGraphId: string | null;
  currentExecutionId: string | null;
  currentAPIToolId: string | null;
  navigate: (
    page: "graphs" | "graph-editor" | "execution" | "api-tools" | "api-tool-editor",
    id?: string
  ) => void;

  // Graphs
  graphs: Graph[];
  setGraphs: (graphs: Graph[]) => void;

  // Executions
  executions: Execution[];
  setExecutions: (executions: Execution[]) => void;
}

const savedUrl = localStorage.getItem("brockley_server_url") || "";
const savedKey = localStorage.getItem("brockley_api_key") || "";

export const useAppStore = create<AppState>((set) => ({
  serverUrl: savedUrl,
  apiKey: savedKey,
  isConnected: !!savedUrl,

  setConnection: (url: string, key: string) => {
    localStorage.setItem("brockley_server_url", url);
    localStorage.setItem("brockley_api_key", key);
    set({ serverUrl: url, apiKey: key, isConnected: true });
  },

  disconnect: () => {
    localStorage.removeItem("brockley_server_url");
    localStorage.removeItem("brockley_api_key");
    set({
      serverUrl: "",
      apiKey: "",
      isConnected: false,
      currentPage: "graphs",
    });
  },

  currentPage: "graphs",
  currentGraphId: null,
  currentExecutionId: null,
  currentAPIToolId: null,
  navigate: (page, id) => {
    if (page === "graph-editor") {
      set({ currentPage: page, currentGraphId: id || null });
    } else if (page === "execution") {
      set({ currentPage: page, currentExecutionId: id || null });
    } else if (page === "api-tool-editor") {
      set({ currentPage: page, currentAPIToolId: id || null });
    } else {
      set({ currentPage: page });
    }
  },

  graphs: [],
  setGraphs: (graphs) => set({ graphs }),

  executions: [],
  setExecutions: (executions) => set({ executions }),
}));
