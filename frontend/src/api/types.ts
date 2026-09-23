import type { components } from "./schema";

type Schema = components["schemas"];
export type CreateVideoTaskInput = Schema["CreateVideoTask"];
export type CreateCompositionInput = Schema["CreateComposition"];
export type TaskStatus =
  "queued" | "running" | "succeeded" | "failed" | "cancelled";
export interface TaskInput {
  prompt: string;
  aspect_ratio?: string;
  source_image_url?: string;
  last_frame_url?: string;
  driving_audio_url?: string;
  duration?: number;
  resolution?: string;
}
export interface Task {
  id: string;
  project_id?: string;
  draft_id?: string;
  scene_id?: string;
  input_version?: number;
  input_hash?: string;
  kind: Schema["Task"]["kind"];
  status: TaskStatus;
  progress: number;
  stage: string;
  provider_code?: string;
  model_id?: string;
  remote_task_id?: string;
  result_text?: string;
  input: TaskInput;
  result_url?: string;
  result_duration_seconds?: number;
  retry_count: number;
  error?: string;
  created_at: string;
  updated_at: string;
}
export interface TaskEvent {
  task_id: string;
  project_id?: string;
  draft_id?: string;
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
  version?: number;
  story: { summary: string; body: string; scenes: Scene[]; updated_at: string };
  storyboard_source_body?: string;
  candidates?: Candidate[];
  storyboard_snapshots?: StoryboardSnapshot[];
  body_snapshots?: BodySnapshot[];
  bindings?: SceneBinding[];
  video_versions?: VideoVersion[];
  selected_versions?: Record<string,string>;
  compositions?: Composition[];
  outputs?: DraftOutput[];
  created_at?: string;
  updated_at?: string;
}
export interface Composition {
  id:string;
  task_id:string;
  clip_version_ids:string[];
  music_material_id?:string;
  music_volume:number;
  source_volume:number;
  voice_volume?:number;
  aspect_ratio:CreateCompositionInput["aspect_ratio"];
  resolution:CreateCompositionInput["resolution"];
  local_media_path:string;
  input_fingerprint:string;
  created_at:string;
}
export interface VideoVersion {
  id: string;
  scene_id: string;
  task_id: string;
  local_media_path: string;
  duration_seconds: number;
  input_fingerprint: string;
  created_at: string;
  based_on_old_settings?: boolean;
}
export interface SceneBinding {
  scene_id: string;
  material_id: string;
  usage: Schema["SceneBinding"]["usage"];
  position?: number;
}
export interface Candidate {
  id: string;
  task_id: string;
  target: Schema["Candidate"]["target"];
  mode?: string;
  source_version: number;
  source_body?: string;
  body?: string;
  scenes?: Scene[];
  status: string;
}
export interface StoryboardSnapshot {
  id: string;
  scenes: Scene[];
}
export interface BodySnapshot {
  id: string;
  body: string;
  applied_body: string;
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
export interface DraftOutput {
  task_id: string;
  kind: Task["kind"];
  text?: string;
  url?: string;
  created_at: string;
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
export type MaterialKind = "scene" | "character" | "prop" | "voice" | "music" | "visual" | "frame";
export interface Material {
  id: string;
  name: string;
  kind: MaterialKind;
  legacy_kind?: MaterialKind;
  tags?: string[];
  media?: { format?: string; size_bytes?: number; width?: number; height?: number; duration_seconds?: number };
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
