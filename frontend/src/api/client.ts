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
  SceneBinding,
  VideoVersion,
  Composition,
  CreateVideoTaskInput,
  CreateCompositionInput,
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
    throw Object.assign(new Error(payload?.error?.message || payload?.message || `请求失败 (${response.status})`), { status: response.status, payload });
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}
export const api = {
  overview: () => request<Overview>("/api/overview"),
  shutdown: () =>
    request<{ status: string }>("/api/system/shutdown", { method: "POST" }),
  projects: () => request<{ projects: Project[] }>("/api/projects"),
  project: (id: string) =>
    request<Project>(`/api/projects/${encodeURIComponent(id)}`),
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
  draft: (projectId: string, draftId: string) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}`),
  saveBody: (projectId: string, draftId: string, body: string, expected_version: number, name?: string) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}`, { method: "PUT", body: JSON.stringify({ body, expected_version, name }) }),
  createCandidate: (projectId: string, draftId: string, input: { expected_version: number; target: "story" | "storyboard"; mode?: string; instruction?: string; model_id: string; idempotency_key: string }) =>
    request<Task>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/candidates`, { method: "POST", body: JSON.stringify(input) }),
  applyCandidate: (projectId: string, draftId: string, candidateId: string, expected_version: number, action: "append" | "replace", reconfirm = false) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/candidates/${encodeURIComponent(candidateId)}/apply`, { method: "POST", body: JSON.stringify({ expected_version, action, reconfirm }) }),
  restoreDraft: (projectId: string, draftId: string, snapshot_id: string, expected_version: number, reconfirm = false) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/restore`, { method: "POST", body: JSON.stringify({ snapshot_id, expected_version, reconfirm }) }),
  writeScene: (projectId: string, draftId: string, sceneId: string | null, input: { expected_version: number; action: "create" | "update" | "copy" | "move"; title?: string; visual_prompt?: string; narration?: string; duration_seconds?: number; order?: number }) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes${sceneId ? `/${encodeURIComponent(sceneId)}` : ""}`, { method: sceneId ? "PATCH" : "POST", body: JSON.stringify(input) }),
  deleteScene: (projectId: string, draftId: string, sceneId: string, expected_version: number) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}?expected_version=${expected_version}`, { method: "DELETE" }),
  saveBindings: (projectId: string, draftId: string, sceneId: string, expected_version: number, bindings: SceneBinding[]) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}/bindings`, { method:"PUT", body:JSON.stringify({expected_version,bindings}) }),
  videoCapability: (modelId: string) =>
    request<{supported:boolean; reason?:string; capability?:{min_duration:number; max_duration:number; resolutions:string[]; allowed_media:Record<string,number>}}>(`/api/models/${encodeURIComponent(modelId)}/video-capability`),
  validateSceneVideo: (projectId:string,draftId:string,sceneId:string,model_id:string,resolution:string) =>
    request<{valid:boolean;issues:{scene_id:string;field:string;message:string}[]}>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}/video-validation`,{method:"POST",body:JSON.stringify({model_id,resolution})}),
  createSceneVideo: (projectId:string,draftId:string,sceneId:string,input:CreateVideoTaskInput) =>
    request<Task>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}/video-tasks`,{method:"POST",body:JSON.stringify(input)}),
  createBatchVideo: (projectId:string,draftId:string,input:CreateVideoTaskInput) =>
    request<{created:Task[];failed:{scene_id:string;field:string;message:string}[]}>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/video-tasks/batch`,{method:"POST",body:JSON.stringify(input)}),
  sceneVersions: (projectId:string,draftId:string,sceneId:string) =>
    request<{versions:VideoVersion[];selected_version_id:string}>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}/versions`),
  selectSceneVersion: (projectId:string,draftId:string,sceneId:string,expected_version:number,version_id:string) =>
    request<Draft>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/scenes/${encodeURIComponent(sceneId)}/selected-version`,{method:"PUT",body:JSON.stringify({expected_version,version_id})}),
  createComposition: (projectId:string,draftId:string,input:CreateCompositionInput) =>
    request<Task>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/compositions`,{method:"POST",body:JSON.stringify(input)}),
  compositions: (projectId:string,draftId:string) =>
    request<{compositions:Composition[]}>(`/api/projects/${encodeURIComponent(projectId)}/drafts/${encodeURIComponent(draftId)}/compositions`),
  tasks: (projectId = "", draftId = "") => {
    const params = new URLSearchParams({ limit: "100" });
    if (projectId) params.set("project_id", projectId);
    if (draftId) params.set("draft_id", draftId);
    return request<{ tasks: Task[]; total: number }>(`/api/tasks?${params}`);
  },
  createTask: (
    project_id: string,
    draft_id: string,
    kind: Task["kind"],
    model_id: string,
    input: TaskInput,
  ) =>
    request<Task>("/api/tasks", {
      method: "POST",
      body: JSON.stringify({
        project_id,
        draft_id,
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
  downloadUrl: (id: string) =>
    `${endpoint}/api/tasks/${encodeURIComponent(id)}/download`,
  mediaUrl: (url: string) =>
    url.startsWith("/media/") ? `${endpoint}${url}` : url,
  downloadTask: async (id: string, kind: "image" | "video" | "composition") => {
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
      throw new Error(
        payload?.error?.message || `下载失败 (${response.status})`,
      );
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
    request<ProviderConnection>(
      create
        ? "/api/connections"
        : `/api/connections/${encodeURIComponent(item.id)}`,
      {
        method: create ? "POST" : "PUT",
        body: JSON.stringify({ ...item, api_key }),
      },
    ),
  deleteConnection: (id: string) =>
    request<void>(`/api/connections/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  testConnection: (id: string) =>
    request<{ reachable: boolean; model_count: number }>(
      `/api/connections/${encodeURIComponent(id)}/test`,
      { method: "POST" },
    ),
  models: (capability?: Capability, connectionID?: string) => {
    const params = new URLSearchParams();
    if (capability) params.set("capability", capability);
    if (connectionID) params.set("connection_id", connectionID);
    return request<{ models: Model[] }>(`/api/models?${params}`);
  },
  syncModels: (connectionID: string) =>
    request<{ synced: number }>(
      `/api/connections/${encodeURIComponent(connectionID)}/models/sync`,
      { method: "POST" },
    ),
  addModel: (
    connectionID: string,
    model_id: string,
    name: string,
    capabilities: Capability[],
  ) =>
    request<Model>(
      `/api/connections/${encodeURIComponent(connectionID)}/models`,
      {
        method: "POST",
        body: JSON.stringify({ model_id, name, capabilities }),
      },
    ),
  updateModel: (
    item: Model,
    values: {
      enabled?: boolean;
      default?: boolean;
      capabilities?: Capability[];
    },
  ) =>
    request<Model>(
      `/api/connections/${encodeURIComponent(item.connection_id)}/models/${encodeURIComponent(item.id)}`,
      {
        method: "PATCH",
        body: JSON.stringify(values),
      },
    ),
  deleteModel: (item: Model) =>
    request<void>(
      `/api/connections/${encodeURIComponent(item.connection_id)}/models/${encodeURIComponent(item.id)}`,
      { method: "DELETE" },
    ),
  materials: (kind: MaterialKind | "" = "", q = "", mediaType = "") => {
    const params = new URLSearchParams();
    if (kind) params.set("kind", kind);
    if (q) params.set("q", q);
    if (mediaType) params.set("media_type", mediaType);
    return request<{ materials: Material[] }>(`/api/materials?${params}`);
  },
  materialUses: (id: string) => request<{ uses: string[] }>(`/api/materials/${encodeURIComponent(id)}/uses`),
  uploadMaterial: (file: File, kind: MaterialKind, onProgress?: (value: number) => void) => {
    const body = new FormData();
    body.append("file", file);
    body.append("kind", kind);
    body.append("name", file.name);
    return new Promise<Material>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${endpoint}/api/materials/upload`);
      if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) onProgress?.(Math.round(100 * event.loaded / event.total));
      };
      xhr.onload = () => {
        let payload: unknown;
        try { payload = JSON.parse(xhr.responseText); } catch { payload = null; }
        if (xhr.status >= 200 && xhr.status < 300) resolve(payload as Material);
        else reject(new Error((payload as { error?: { message?: string } })?.error?.message || `上传失败 (${xhr.status})`));
      };
      xhr.onerror = () => reject(new Error("上传连接失败，请重试。"));
      xhr.send(body);
    });
  },
  saveMaterial: (item: Material) =>
    request<Material>(`/api/materials/${encodeURIComponent(item.id)}`, {
      method: "PUT",
      body: JSON.stringify(item),
    }),
  deleteMaterial: (id: string) =>
    request<void>(`/api/materials/${id}`, { method: "DELETE" }),
  socketUrl: () =>
    `${endpoint.replace(/^http/, "ws")}/ws${token ? `?token=${encodeURIComponent(token)}` : ""}`,
};
