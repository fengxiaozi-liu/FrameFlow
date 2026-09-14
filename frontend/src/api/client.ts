import type {
  Capability,
  Draft,
  Material,
  MaterialKind,
  Overview,
  Project,
  Provider,
  Task,
  TaskEvent,
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
  createTask: (kind: Task["kind"], provider_code: string, input: TaskInput) =>
    request<Task>("/api/tasks", {
      method: "POST",
      body: JSON.stringify({ kind, provider_code, ...input }),
    }),
  cancelTask: (id: string) =>
    request<Task>(`/api/tasks/${id}/cancel`, { method: "POST" }),
  retryTask: (id: string) =>
    request<Task>(`/api/tasks/${id}/retry`, { method: "POST" }),
  taskEvents: (id: string, after = 0) =>
    request<{ events: TaskEvent[] }>(
      `/api/tasks/${id}/event-log?after=${after}`,
    ),
  resultUrl: (id: string) => `${endpoint}/api/tasks/${id}/result`,
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
  socketUrl: (id: string) =>
    `${endpoint.replace(/^http/, "ws")}/api/tasks/${id}/events${token ? `?token=${encodeURIComponent(token)}` : ""}`,
};
