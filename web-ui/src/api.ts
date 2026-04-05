import type { Graph, Execution, ExecutionStep, APIToolDefinition, APIToolTestResult } from "./store";

class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  serverUrl: string,
  apiKey: string,
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const url = `${serverUrl.replace(/\/$/, "")}${path}`;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (apiKey) {
    headers["Authorization"] = `Bearer ${apiKey}`;
  }
  const res = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "Unknown error");
    throw new ApiError(res.status, `${res.status}: ${text}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json();
}

export async function checkHealth(
  serverUrl: string,
  apiKey: string
): Promise<boolean> {
  try {
    await request(serverUrl, apiKey, "GET", "/health");
    return true;
  } catch {
    return false;
  }
}

export async function fetchGraphs(
  serverUrl: string,
  apiKey: string
): Promise<Graph[]> {
  const res = await request<{ items: Graph[] }>(serverUrl, apiKey, "GET", "/api/v1/graphs");
  return res.items || [];
}

export async function createGraph(
  serverUrl: string,
  apiKey: string,
  graph: Partial<Graph>
): Promise<Graph> {
  return request<Graph>(serverUrl, apiKey, "POST", "/api/v1/graphs", graph);
}

export async function getGraph(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<Graph> {
  return request<Graph>(serverUrl, apiKey, "GET", `/api/v1/graphs/${id}`);
}

export async function deleteGraph(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<void> {
  return request<void>(serverUrl, apiKey, "DELETE", `/api/v1/graphs/${id}`);
}

export async function validateGraph(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<{ valid: boolean; errors: string[] }> {
  return request(
    serverUrl,
    apiKey,
    "POST",
    `/api/v1/graphs/${id}/validate`
  );
}

export async function invokeGraph(
  serverUrl: string,
  apiKey: string,
  graphId: string,
  input: Record<string, unknown>,
  mode: "sync" | "async" = "async"
): Promise<Execution> {
  return request<Execution>(
    serverUrl,
    apiKey,
    "POST",
    "/api/v1/executions",
    { graph_id: graphId, input, mode }
  );
}

export async function getExecution(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<Execution> {
  return request<Execution>(
    serverUrl,
    apiKey,
    "GET",
    `/api/v1/executions/${id}`
  );
}

export async function updateGraph(
  serverUrl: string,
  apiKey: string,
  id: string,
  body: Record<string, unknown>
): Promise<Graph> {
  return request<Graph>(serverUrl, apiKey, "PUT", `/api/v1/graphs/${id}`, body);
}

export async function getExecutionSteps(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<ExecutionStep[]> {
  const res = await request<{ items: ExecutionStep[] }>(
    serverUrl,
    apiKey,
    "GET",
    `/api/v1/executions/${id}/steps`
  );
  return res.items || [];
}

// ─── API Tools ───

export async function fetchAPITools(
  serverUrl: string,
  apiKey: string
): Promise<APIToolDefinition[]> {
  const res = await request<{ items: APIToolDefinition[] }>(
    serverUrl,
    apiKey,
    "GET",
    "/api/v1/api-tools"
  );
  return res.items || [];
}

export async function getAPITool(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<APIToolDefinition> {
  return request<APIToolDefinition>(
    serverUrl,
    apiKey,
    "GET",
    `/api/v1/api-tools/${id}`
  );
}

export async function createAPITool(
  serverUrl: string,
  apiKey: string,
  tool: Partial<APIToolDefinition>
): Promise<APIToolDefinition> {
  return request<APIToolDefinition>(
    serverUrl,
    apiKey,
    "POST",
    "/api/v1/api-tools",
    tool
  );
}

export async function updateAPITool(
  serverUrl: string,
  apiKey: string,
  id: string,
  body: Partial<APIToolDefinition>
): Promise<APIToolDefinition> {
  return request<APIToolDefinition>(
    serverUrl,
    apiKey,
    "PUT",
    `/api/v1/api-tools/${id}`,
    body
  );
}

export async function deleteAPITool(
  serverUrl: string,
  apiKey: string,
  id: string
): Promise<void> {
  return request<void>(
    serverUrl,
    apiKey,
    "DELETE",
    `/api/v1/api-tools/${id}`
  );
}

export async function testAPIToolEndpoint(
  serverUrl: string,
  apiKey: string,
  id: string,
  endpoint: string,
  input: Record<string, unknown>,
  baseUrlOverride?: string
): Promise<APIToolTestResult> {
  return request<APIToolTestResult>(
    serverUrl,
    apiKey,
    "POST",
    `/api/v1/api-tools/${id}/test`,
    {
      endpoint,
      input,
      ...(baseUrlOverride ? { base_url_override: baseUrlOverride } : {}),
    }
  );
}
