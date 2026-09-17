export type TaskStatus =
  "queued" | "running" | "succeeded" | "failed" | "cancelled";
export interface TaskInput {
  prompt: string;
  aspect_ratio?: string;
  source_image_url?: string;
}
export interface Task {
  id: string;
  kind: "story" | "image" | "video";
  status: TaskStatus;
  progress: number;
  stage: string;
  provider_code?: string;
  model_id?: string;
  remote_task_id?: string;
  result_text?: string;
  input: TaskInput;
  result_url?: string;
  retry_count: number;
  error?: string;
  created_at: string;
  updated_at: string;
}
export interface TaskEvent {
  task_id: string;
  status: TaskStatus;
  progress: number;
  stage: string;
  at: string;
}
export interface Scene {
  id: string;
  order: number;
  title: string;
  visual_prompt: string;
  narration: string;
  duration_seconds: number;
}
export interface Draft {
  id: string;
  project_id?: string;
  name: string;
  story: { summary: string; body: string; scenes: Scene[]; updated_at: string };
  created_at?: string;
  updated_at?: string;
}
export interface Project {
  id: string;
  name: string;
  drafts: Draft[];
  created_at: string;
}
export type Capability = "story" | "image" | "video";
export interface Provider {
  code: string;
  name: string;
  capability: Capability;
  model: string;
  base_url: string;
  enabled: boolean;
  status: "disabled" | "enabled" | "healthy";
  last_error?: string;
}
export interface ProviderConnection {
  id: string;
  name: string;
  vendor: "bailian";
  region: string;
  workspace_id?: string;
  base_url: string;
  credential_set: boolean;
}
export interface Model {
  id: string;
  connection_id: string;
  remote_id: string;
  name: string;
  capabilities: Capability[];
  source: "manual" | "discovered";
  supported: boolean;
  enabled: boolean;
  default: boolean;
  last_seen?: string;
}
export type MaterialKind = "visual" | "frame" | "character" | "voice" | "music";
export interface Material {
  id: string;
  name: string;
  kind: MaterialKind;
  url: string;
  created_at: string;
}
export interface Overview {
  projects: number;
  tasks: number;
  running_tasks: number;
  providers: number;
  materials: number;
}
