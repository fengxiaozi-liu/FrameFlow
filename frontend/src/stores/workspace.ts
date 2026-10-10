import { defineStore } from "pinia";
import { ref } from "vue";
import { projectDisplayName } from "../utils/projectDisplayName";
import { api } from "../api/client";
import type { Draft, Project, Scene } from "../api/types";

function blankDraft(): Draft {
  return {
    id: crypto.randomUUID(),
    name: "未命名视频",
    story: {
      summary: "",
      body: "",
      scenes: [],
      updated_at: new Date().toISOString(),
    },
    outputs: [],
  };
}

export const useWorkspaceStore = defineStore("workspace", () => {
  const idea = ref("");
  const projectId = ref("");
  const projectName = ref("");
  const projectDraftCount = ref(0);
  const storyProvider = ref("");
  const imageProvider = ref("");
  const videoProvider = ref("");
  const activeResource = ref("character");
  const aspectRatio = ref("9:16");
  const subtitles = ref(true);
  const bgmVolume = ref(25);
  const frameSource = ref("generated");
  const selectedMaterials = ref<Record<string, string>>({});
  const draft = ref<Draft>(blankDraft());

  function startNew(initialIdea = "") {
    projectId.value = "";
    projectName.value = "";
    projectDraftCount.value = 0;
    idea.value = initialIdea;
    draft.value = blankDraft();
  }

  function applyDraft(value: Draft, preserveBody?: string) {
    draft.value = structuredClone(value);
    if (projectDraftCount.value === 1) projectName.value = value.name;
    draft.value.outputs ||= [];
    if (preserveBody !== undefined) draft.value.story.body = preserveBody;
  }

  function mergeGeneratedDraft(value: Draft, submittedBody?: string) {
    const currentBody = draft.value.story.body;
    const conflicted =
      submittedBody !== undefined && currentBody !== submittedBody;
    applyDraft(value, conflicted ? currentBody : undefined);
    return conflicted;
  }

  async function load(project: string, draftID: string) {
    const value = await api.project(project);
    const selected = value.drafts.find((item) => item.id === draftID);
    if (!selected) throw new Error("项目中不存在指定草稿");
    projectId.value = value.id;
    projectDraftCount.value = value.drafts.length;
    projectName.value = projectDisplayName(value);
    applyDraft(selected);
    return value;
  }

  async function save(): Promise<Project> {
    if (!projectId.value) {
      const project = await api.createProject(draft.value.name);
      projectId.value = project.id;
      projectName.value = project.name;
    }
    draft.value.story.updated_at = new Date().toISOString();
    const project = await api.saveDraft(projectId.value, draft.value);
    projectDraftCount.value = project.drafts.length;
    projectName.value = projectDisplayName(project);
    const saved = project.drafts.find((item) => item.id === draft.value.id);
    if (saved) applyDraft(saved);
    return project;
  }

  function addScene() {
    const order = draft.value.story.scenes.length + 1;
    draft.value.story.scenes.push({
      id: "scene-" + Date.now(),
      order,
      title: "分镜 " + order,
      visual_prompt: "",
      narration: "",
      duration_seconds: 10,
    } as Scene);
  }

  return {
    idea,
    projectId,
    projectName,
    storyProvider,
    imageProvider,
    videoProvider,
    activeResource,
    aspectRatio,
    subtitles,
    bgmVolume,
    frameSource,
    selectedMaterials,
    draft,
    startNew,
    applyDraft,
    mergeGeneratedDraft,
    load,
    save,
    addScene,
  };
});
