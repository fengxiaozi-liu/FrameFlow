import type {
  Capability,
  Draft,
  Material,
  MaterialKind,
  Model,
  Overview,
  Project,
  Provider,
  ProviderConnection,
  Task,
  TaskInput,
} from "./types";
const base = import.meta.env.VITE_API_BASE_URL || "";
const endpoint = base || location.origin;
const token = import.meta.env.VITE_API_TOKEN || "";
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData))
    headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetch(endpoint + path, { ...init, headers });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error?.message || `请求失败 (${response.status})`);
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}
export const api = {
  overview: () => request<Overview>("/api/overview"),
  shutdown: () =>
    request<{ status: string }>("/api/system/shutdown", { method: "POST" }),
  projects: () => request<{ projects: Project[] }>("/api/projects"),
  createProject: (name: string) =>
    request<Project>("/api/projects", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  saveDraft: (projectId: string, draft: Draft) =>
    request<Project>(`/api/projects/${projectId}/drafts`, {
      method: "POST",
      body: JSON.stringify(draft),
    }),
  tasks: () =>
    request<{ tasks: Task[]; total: number }>("/api/tasks?limit=100"),
  createTask: (kind: Task["kind"], model_id: string, input: TaskInput) =>
    request<Task>("/api/tasks", {
      method: "POST",
      body: JSON.stringify({
        kind,
        model_id,
        ...input,
      }),
    }),
  cancelTask: (id: string) =>
    request<Task>(`/api/tasks/${id}/cancel`, { method: "POST" }),
  retryTask: (id: string) =>
    request<Task>(`/api/tasks/${id}/retry`, { method: "POST" }),
  resultUrl: (id: string) => `${endpoint}/api/tasks/${id}/result`,
  downloadUrl: (id: string) => `${endpoint}/api/tasks/${encodeURIComponent(id)}/download`,
  mediaUrl: (url: string) => url.startsWith("/media/") ? `${endpoint}${url}` : url,
  downloadTask: async (id: string, kind: "image" | "video") => {
    const destination = `${endpoint}/api/tasks/${encodeURIComponent(id)}/download`;
    if (!token) {
      const link = document.createElement("a");
      link.href = destination;
      link.download = `frameflow-${id}.${kind === "image" ? "png" : "mp4"}`;
      document.body.append(link);
      link.click();
      link.remove();
      return;
    }
    const headers = new Headers();
    headers.set("Authorization", `Bearer ${token}`);
    const response = await fetch(destination, { headers });
    if (!response.ok) {
      const payload = await response.json().catch(() => null);
      throw new Error(payload?.error?.message || `下载失败 (${response.status})`);
    }
    const url = URL.createObjectURL(await response.blob());
    const link = document.createElement("a");
    link.href = url;
    link.download = `frameflow-${id}.${kind === "image" ? "png" : "mp4"}`;
    document.body.append(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
  },
  providers: (capability: Capability) =>
    request<{ providers: Provider[] }>(
      `/api/providers?capability=${capability}`,
    ),
  saveProvider: (item: Provider, api_key = "") =>
    request<Provider>("/api/providers", {
      method: "POST",
      body: JSON.stringify({ ...item, api_key }),
    }),
  deleteProvider: (code: string) =>
    request<void>(`/api/providers/${code}`, { method: "DELETE" }),
  testProvider: (code: string) =>
    request<Provider>(`/api/providers/${code}/test`, { method: "POST" }),
  connections: () =>
    request<{ connections: ProviderConnection[] }>("/api/connections"),
  saveConnection: (item: ProviderConnection, api_key: string, create = false) =>
    request<ProviderConnection>(create ? "/api/connections" : `/api/connections/${encodeURIComponent(item.id)}`, {
      method: create ? "POST" : "PUT",
      body: JSON.stringify({ ...item, api_key }),
    }),
  deleteConnection: (id: string) =>
    request<void>(`/api/connections/${encodeURIComponent(id)}`, { method: "DELETE" }),
  testConnection: (id: string) =>
    request<{ reachable: boolean; model_count: number }>(`/api/connections/${encodeURIComponent(id)}/test`, { method: "POST" }),
  models: (capability?: Capability, connectionID?: string) => {
    const params = new URLSearchParams();
    if (capability) params.set("capability", capability);
    if (connectionID) params.set("connection_id", connectionID);
    return request<{ models: Model[] }>(`/api/models?${params}`);
  },
  syncModels: (connectionID: string) =>
    request<{ synced: number }>(`/api/connections/${encodeURIComponent(connectionID)}/models/sync`, { method: "POST" }),
  addModel: (connectionID: string, model_id: string, name: string, capabilities: Capability[]) =>
    request<Model>(`/api/connections/${encodeURIComponent(connectionID)}/models`, {
      method: "POST", body: JSON.stringify({ model_id, name, capabilities }),
    }),
  updateModel: (item: Model, values: { enabled?: boolean; default?: boolean; capabilities?: Capability[] }) =>
    request<Model>(`/api/connections/${encodeURIComponent(item.connection_id)}/models/${encodeURIComponent(item.id)}`, {
      method: "PATCH", body: JSON.stringify(values),
    }),
  deleteModel: (item: Model) =>
    request<void>(`/api/connections/${encodeURIComponent(item.connection_id)}/models/${encodeURIComponent(item.id)}`, { method: "DELETE" }),
  materials: (kind: MaterialKind) =>
    request<{ materials: Material[] }>(`/api/materials?kind=${kind}`),
  uploadMaterial: (file: File, kind: MaterialKind) => {
    const body = new FormData();
    body.append("file", file);
    body.append("kind", kind);
    body.append("name", file.name);
    return request<Material>("/api/materials/upload", {
      method: "POST",
      body,
    });
  },
  saveMaterial: (item: Material) =>
    request<Material>("/api/materials", {
      method: "POST",
      body: JSON.stringify(item),
    }),
  deleteMaterial: (id: string) =>
    request<void>(`/api/materials/${id}`, { method: "DELETE" }),
  socketUrl: () =>
    `${endpoint.replace(/^http/, "ws")}/ws${token ? `?token=${encodeURIComponent(token)}` : ""}`,
};
