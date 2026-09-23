<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { onBeforeRouteLeave } from "vue-router";
import { api } from "../api/client";
import { useWorkspaceStore } from "../stores/workspace";
import { useTaskStore } from "../stores/tasks";
import type { Candidate, Draft, Model, Scene, TaskEvent, Material, SceneBinding, VideoVersion, Composition, CreateVideoTaskInput, CreateCompositionInput } from "../api/types";

const workspace = useWorkspaceStore();
const tasks = useTaskStore();
const route = useRoute();
const router = useRouter();
const activeTab = ref<"body" | "scenes" | "video">("body");
const drawerOpen = ref(false);
const drawerElement = ref<HTMLElement>();
let drawerReturnFocus: HTMLElement | null = null;
const drawerMode = ref<"create" | "revise">("create");
const instruction = ref("");
const message = ref("");
const saveState = ref<"saved" | "saving" | "failed" | "unsaved">("saved");
const storyModels = ref<Model[]>([]);
const selectedSceneID = ref("");
const sceneForm = ref<Scene | null>(null);
const candidateNeedsReview = ref("");
const importInput = ref<HTMLInputElement>();
const importPreview = ref<string | null>(null);
const importError = ref("");
let lastSavedBody = "";
let lastSavedName = "";
let saveTimer: ReturnType<typeof setTimeout> | undefined;
let pendingSave: Promise<void> | undefined;
let internalRouteUpdate = false;
let unsubscribe = () => {};

const draft = computed(() => workspace.draft);
const selectedScene = computed(() => draft.value.story.scenes.find((scene) => scene.id === selectedSceneID.value));
const storyCandidates = computed(() => (draft.value.candidates || []).filter((item) => item.target === "story" && item.status === "preview"));
const storyboardCandidates = computed(() => (draft.value.candidates || []).filter((item) => item.target === "storyboard" && item.status === "preview"));
const latestStory = computed(() => storyCandidates.value.at(-1));
const latestStoryboard = computed(() => storyboardCandidates.value.at(-1));
const canGenerateStoryboard = computed(() => !!draft.value.story.body.trim() && saveState.value === "saved" && !!selectedStoryModel.value);
const selectedStoryModel = ref("");
const videoModels = ref<Model[]>([]);
const selectedVideoModel = ref("");
const videoCapability = ref<{supported:boolean;reason?:string;capability?:{min_duration:number;max_duration:number;resolutions:string[];allowed_media:Record<string,number>}} | null>(null);
const materials = ref<Material[]>([]);
const pickerUsage = ref<SceneBinding["usage"] | "">("");
const pickerChoice = ref("");
const bindingBusy = ref(false);
const pickerFile = ref<HTMLInputElement>();
const pickerUploading = ref(false);
const pickerProgress = ref(0);
const videoResolution = ref<NonNullable<CreateVideoTaskInput["resolution"]>>("1080P");
const validationIssues = ref<{field:string;message:string}[]>([]);
const selectedSceneVersions = ref<VideoVersion[]>([]);
const selectedSceneTasks = computed(() => tasks.items.filter((item) => item.scene_id === selectedSceneID.value && item.project_id === workspace.projectId && item.draft_id === draft.value.id));
const batchFailures = ref<{scene_id:string;field:string;message:string}[]>([]);
const musicMaterialID = ref("");
const musicVolume = ref(0.25);
const sourceVolume = ref(1);
const voiceVolume = ref(1);
const outputAspect = ref<CreateCompositionInput["aspect_ratio"]>("16:9");
const outputResolution = ref<CreateCompositionInput["resolution"]>("1080P");
const compositionIssues = ref<{scene_id:string;field:string;message:string}[]>([]);
const playingURL = ref("");
const musicMaterials = computed(() => materials.value.filter((item)=>item.kind==="music"));
const orderedClips = computed(() => draft.value.story.scenes.map((scene) => ({
  scene,
  version:(draft.value.video_versions || []).find((version)=>version.id===draft.value.selected_versions?.[scene.id] && version.scene_id===scene.id),
})));
const canCompose = computed(() => orderedClips.value.length>0 && orderedClips.value.every((entry)=>!!entry.version));
const compositionTasks = computed(() => tasks.items.filter((item)=>item.kind==="composition" && item.project_id===workspace.projectId && item.draft_id===draft.value.id));
function compositionIsOld(value: Composition) {
  return value.clip_version_ids.length !== orderedClips.value.length ||
    value.clip_version_ids.some((id,index)=>id!==orderedClips.value[index]?.version?.id) ||
    (value.music_material_id || "") !== musicMaterialID.value ||
    value.music_volume !== musicVolume.value || value.source_volume !== sourceVolume.value ||
    (value.voice_volume ?? 1) !== voiceVolume.value ||
    value.aspect_ratio !== outputAspect.value || value.resolution !== outputResolution.value;
}
const sceneBindings = computed(() => (draft.value.bindings || []).filter((binding) => binding.scene_id === selectedSceneID.value));
const usageLabels: Record<SceneBinding["usage"], string> = {
  character_reference:"角色参考", scene_reference:"场景参考", prop_reference:"道具参考",
  first_frame:"首帧", last_frame:"尾帧", driving_audio:"驱动音频", voiceover:"后期配音",
};
const pickerMaterials = computed(() => materials.value.filter((item) =>
  pickerUsage.value === "character_reference" ? item.kind === "character" :
  pickerUsage.value === "prop_reference" ? item.kind === "prop" :
  pickerUsage.value === "driving_audio" || pickerUsage.value === "voiceover" ? item.kind === "voice" :
  item.kind === "scene"));
const instructionKey = computed(() => `frameflow-story-instruction:${workspace.projectId}:${draft.value.id}`);

function acceptDraft(value: Draft, preserveBody?: string, preserveName?: string) {
  lastSavedBody = value.story.body;
  lastSavedName = value.name;
  workspace.applyDraft(value, preserveBody);
  if (preserveName !== undefined) draft.value.name = preserveName;
  if (selectedSceneID.value && !value.story.scenes.some((scene) => scene.id === selectedSceneID.value)) selectedSceneID.value = "";
  if (!selectedSceneID.value && value.story.scenes.length) selectedSceneID.value = value.story.scenes[0].id;
  selectScene(selectedSceneID.value);
}

function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer);
  saveState.value = "unsaved";
  saveTimer = setTimeout(() => void saveNow(), 800);
}

watch(() => [draft.value.story.body, draft.value.name], ([body, name]) => {
  if (body !== lastSavedBody || name !== lastSavedName) scheduleSave();
});
watch(instruction, (value) => sessionStorage.setItem(instructionKey.value, value));

async function saveNow() {
  if (saveTimer) clearTimeout(saveTimer);
  if (pendingSave) await pendingSave;
  if (draft.value.story.body === lastSavedBody && draft.value.name === lastSavedName && workspace.projectId) { saveState.value = "saved"; return; }
  const submitted = draft.value.story.body;
  const submittedName = draft.value.name;
  saveState.value = "saving";
  pendingSave = (async () => {
    try {
      if (!workspace.projectId) {
        await workspace.save();
        internalRouteUpdate = true;
        try { await router.replace(`/studio/${workspace.projectId}/${draft.value.id}`); }
        finally { internalRouteUpdate = false; }
        lastSavedBody = submitted;
        lastSavedName = submittedName;
      } else {
        const saved = await api.saveBody(workspace.projectId, draft.value.id, submitted, draft.value.version || 0, submittedName);
        acceptDraft(saved, draft.value.story.body !== submitted ? draft.value.story.body : undefined, draft.value.name !== submittedName ? draft.value.name : undefined);
      }
      if (draft.value.story.body !== submitted || draft.value.name !== submittedName) scheduleSave();
      else saveState.value = "saved";
    } catch (error) {
      saveState.value = "failed";
      message.value = (error as Error).message;
    } finally { pendingSave = undefined; }
  })();
  await pendingSave;
}

async function requireSaved() {
  await saveNow();
  if (saveState.value !== "saved") throw new Error("尚未保存，暂不能生成。请重试保存。");
}

async function refreshDraft() {
  if (!workspace.projectId) return;
  const body = draft.value.story.body;
  const preserve = body !== lastSavedBody ? body : undefined;
  const preserveName = draft.value.name !== lastSavedName ? draft.value.name : undefined;
  acceptDraft(await api.draft(workspace.projectId, draft.value.id), preserve, preserveName);
  await loadVersions();
}

async function handleTaskEvent(event: TaskEvent) {
  if (event.project_id !== workspace.projectId || event.draft_id !== draft.value.id) return;
  if (event.status === "succeeded" || event.status === "failed") {
    try { await refreshDraft(); await tasks.load(); }
    catch (error) { message.value = (error as Error).message; }
  }
}

onMounted(async () => {
  try {
    const projectID = String(route.params.projectId || "");
    const draftID = String(route.params.draftId || "");
    if (projectID && draftID) await workspace.load(projectID, draftID);
    else workspace.startNew(sessionStorage.getItem("frameflow-idea") || "");
    lastSavedBody = draft.value.story.body;
    lastSavedName = draft.value.name;
    const latestComposition=draft.value.compositions?.at(-1);
    if (latestComposition) {
      musicMaterialID.value=latestComposition.music_material_id || "";
      musicVolume.value=latestComposition.music_volume;
      sourceVolume.value=latestComposition.source_volume;
      voiceVolume.value=latestComposition.voice_volume ?? 1;
      outputAspect.value=latestComposition.aspect_ratio;
      outputResolution.value=latestComposition.resolution;
    }
    instruction.value = sessionStorage.getItem(instructionKey.value) || workspace.idea;
    const models = await api.models("story");
    storyModels.value = models.models.filter((model) => model.enabled && model.supported);
    selectedStoryModel.value = storyModels.value.find((model) => model.default)?.id || storyModels.value[0]?.id || "";
    videoModels.value = (await api.models("video")).models.filter((model) => model.enabled);
    selectedVideoModel.value = videoModels.value.find((model) => model.default)?.id || videoModels.value[0]?.id || "";
    materials.value = (await api.materials()).materials || [];
    const requestedScene = String(route.query.scene || "");
    if (requestedScene && draft.value.story.scenes.some((scene) => scene.id === requestedScene)) {
      selectScene(requestedScene); activeTab.value = "scenes";
    } else if (draft.value.story.scenes.length) selectScene(draft.value.story.scenes[0].id);
    await tasks.load();
    unsubscribe = tasks.subscribe((event) => void handleTaskEvent(event));
  } catch (error) { message.value = (error as Error).message; }
});
watch(selectedVideoModel, async (id) => {
  videoCapability.value = id ? await api.videoCapability(id).catch(() => ({supported:false,reason:"无法读取模型能力"})) : null;
});
onUnmounted(() => { unsubscribe(); if (saveTimer) clearTimeout(saveTimer); });
onBeforeRouteLeave(async () => {
  if (internalRouteUpdate) return true;
  if (saveState.value === "saved") return true;
  await saveNow();
  return isSaved() || window.confirm("当前修改尚未保存，确定离开？");
});
function isSaved() { return saveState.value === "saved"; }
function beforeUnload(event: BeforeUnloadEvent) {
  if (saveState.value !== "saved") { event.preventDefault(); event.returnValue = ""; }
}
window.addEventListener("beforeunload", beforeUnload);
onUnmounted(() => window.removeEventListener("beforeunload", beforeUnload));

async function openDrawer() {
  drawerReturnFocus = document.activeElement as HTMLElement | null;
  drawerOpen.value = true;
  await nextTick();
  drawerElement.value?.querySelector<HTMLElement>("button")?.focus();
}
async function closeDrawer() {
  drawerOpen.value = false;
  await nextTick();
  drawerReturnFocus?.focus();
}
function drawerKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") { event.preventDefault(); void closeDrawer(); return; }
  if (event.key !== "Tab" || !drawerElement.value) return;
  const focusable = [...drawerElement.value.querySelectorAll<HTMLElement>('button:not([disabled]), textarea, select:not([disabled]), a[href]')];
  if (!focusable.length) return;
  if (event.shiftKey && document.activeElement === focusable[0]) { event.preventDefault(); focusable.at(-1)?.focus(); }
  else if (!event.shiftKey && document.activeElement === focusable.at(-1)) { event.preventDefault(); focusable[0].focus(); }
}

async function generate(target: "story" | "storyboard") {
  try {
    await requireSaved();
    const created = await api.createCandidate(workspace.projectId, draft.value.id, {
      expected_version: draft.value.version || 0,
      idempotency_key: crypto.randomUUID(), target,
      mode: target === "story" ? drawerMode.value : undefined,
      instruction: target === "story" ? instruction.value : "",
      model_id: selectedStoryModel.value,
    });
    await tasks.load();
    message.value = `已提交生成任务 ${created.id}，可继续编辑。`;
  } catch (error) { message.value = (error as Error).message; }
}

async function applyCandidate(candidate: Candidate, action: "append" | "replace") {
  try {
    await requireSaved();
    const applied = await api.applyCandidate(workspace.projectId, draft.value.id, candidate.id, draft.value.version || 0, action, candidateNeedsReview.value === candidate.id);
    candidateNeedsReview.value = "";
    acceptDraft(applied);
    message.value = candidate.target === "story" ? "正文已应用，可从恢复记录撤销。" : "分镜已应用，旧版本可恢复。";
  } catch (error) {
    if ((error as {status?: number}).status === 409) candidateNeedsReview.value = candidate.id;
    message.value = (error as Error).message;
  }
}

async function restore(snapshotID: string) {
  try {
    await requireSaved();
    const value = await api.restoreDraft(workspace.projectId, draft.value.id, snapshotID, draft.value.version || 0, candidateNeedsReview.value === snapshotID);
    candidateNeedsReview.value = "";
    acceptDraft(value);
    message.value = "已恢复保存的版本。";
  } catch (error) {
    if ((error as {status?: number}).status === 409) candidateNeedsReview.value = snapshotID;
    message.value = (error as Error).message;
  }
}

function selectScene(id: string) {
  selectedSceneID.value = id;
  const scene = draft.value.story.scenes.find((item) => item.id === id);
  sceneForm.value = scene ? { ...scene } : null;
  void loadVersions();
}
async function loadVersions() {
  if (!workspace.projectId || !selectedSceneID.value) {selectedSceneVersions.value=[];return}
  try {selectedSceneVersions.value=(await api.sceneVersions(workspace.projectId,draft.value.id,selectedSceneID.value)).versions}
  catch {selectedSceneVersions.value=(draft.value.video_versions || []).filter((version)=>version.scene_id===selectedSceneID.value)}
}
function openPicker(usage: SceneBinding["usage"]) {
  pickerUsage.value = usage;
  pickerChoice.value = "";
}
async function changeBinding(usage: SceneBinding["usage"], materialID = "") {
  if (!selectedSceneID.value) return;
  bindingBusy.value = true;
  try {
    await requireSaved();
    const next = sceneBindings.value.filter((binding) => binding.usage !== usage);
    if (materialID) next.push({scene_id:selectedSceneID.value,material_id:materialID,usage});
    acceptDraft(await api.saveBindings(workspace.projectId,draft.value.id,selectedSceneID.value,draft.value.version || 0,next));
    pickerUsage.value = "";
    message.value = materialID ? "镜头素材绑定已保存。" : "已解绑镜头素材，素材文件仍保留。";
  } catch (cause) { message.value = (cause as Error).message; }
  finally { bindingBusy.value = false; }
}
async function uploadForScene(event: Event) {
  const field = event.target as HTMLInputElement;
  const file = field.files?.[0];
  if (!file || !pickerUsage.value) return;
  const kind = pickerUsage.value === "character_reference" ? "character"
    : pickerUsage.value === "prop_reference" ? "prop"
    : pickerUsage.value === "driving_audio" || pickerUsage.value === "voiceover" ? "voice" : "scene";
  pickerUploading.value = true; pickerProgress.value = 0;
  try {
    const item = await api.uploadMaterial(file, kind, (value) => { pickerProgress.value = value; });
    materials.value = (await api.materials()).materials || [];
    pickerChoice.value = item.id;
    message.value = "素材已上传，请确认绑定到当前镜头。";
  } catch (cause) { message.value = (cause as Error).message; }
  finally { pickerUploading.value = false; field.value = ""; }
}
async function checkVideoScene() {
  try {
    await requireSaved();
    const result = await api.validateSceneVideo(workspace.projectId,draft.value.id,selectedSceneID.value,selectedVideoModel.value,videoResolution.value);
    validationIssues.value = result.issues;
    message.value = result.valid ? "当前镜头参数可提交生成。" : "请按镜头检查结果修正参数。";
  } catch (cause) { message.value = (cause as Error).message; }
}
async function generateSceneVideo() {
  if (!selectedSceneID.value) return;
  try {
    await requireSaved();
    const check = await api.validateSceneVideo(workspace.projectId,draft.value.id,selectedSceneID.value,selectedVideoModel.value,videoResolution.value);
    validationIssues.value = check.issues;
    if (!check.valid) { message.value = "请先修正镜头输入。"; return; }
    const created = await api.createSceneVideo(workspace.projectId,draft.value.id,selectedSceneID.value,{
      expected_version:draft.value.version || 0,idempotency_key:crypto.randomUUID(),
      model_id:selectedVideoModel.value,resolution:videoResolution.value,
    });
    await tasks.load(workspace.projectId,draft.value.id);
    message.value = `镜头任务 ${created.id} 已提交，完成后可选择新版本。`;
  } catch (cause) {
    const issuePayload = (cause as {payload?:{issues?:{field:string;message:string}[]}}).payload;
    if (issuePayload?.issues) validationIssues.value = issuePayload.issues;
    message.value = (cause as Error).message;
  }
}
async function generateBatchVideo() {
  try {
    await requireSaved();
    const result = await api.createBatchVideo(workspace.projectId,draft.value.id,{
      expected_version:draft.value.version || 0,idempotency_key:crypto.randomUUID(),
      model_id:selectedVideoModel.value,resolution:videoResolution.value,
    });
    batchFailures.value = result.failed;
    await tasks.load(workspace.projectId,draft.value.id);
    message.value = `已提交 ${result.created.length} 个镜头，${result.failed.length} 项需要修正。`;
  } catch (cause) { message.value = (cause as Error).message; }
}
async function chooseVersion(versionID: string) {
  try {
    await requireSaved();
    acceptDraft(await api.selectSceneVersion(workspace.projectId,draft.value.id,selectedSceneID.value,draft.value.version || 0,versionID));
    message.value = versionID ? "已选用视频版本。" : "已取消选用。";
  } catch (cause) { message.value = (cause as Error).message; }
}
async function retryVideoTask(id: string) {
  try { await api.retryTask(id); await tasks.load(workspace.projectId,draft.value.id); message.value = "已重试失败任务。"; }
  catch (cause) { message.value = (cause as Error).message; }
}
async function submitComposition() {
  try {
    await requireSaved();
    const result = await api.createComposition(workspace.projectId,draft.value.id,{
      expected_version:draft.value.version || 0,idempotency_key:crypto.randomUUID(),
      music_material_id:musicMaterialID.value || undefined,music_volume:musicVolume.value,
      source_volume:sourceVolume.value,voice_volume:voiceVolume.value,
      aspect_ratio:outputAspect.value,resolution:outputResolution.value,
    });
    compositionIssues.value=[];
    await tasks.load(workspace.projectId,draft.value.id);
    message.value = `成片任务 ${result.id} 已提交。`;
  } catch (cause) {
    compositionIssues.value=(cause as {payload?:{issues?:{scene_id:string;field:string;message:string}[]}}).payload?.issues || [];
    message.value=(cause as Error).message;
  }
}

async function sceneAction(action: "create" | "update" | "copy" | "move", order?: number) {
  try {
    await requireSaved();
    const before = draft.value.story.scenes.length;
    const value = await api.writeScene(workspace.projectId, draft.value.id, action === "create" ? null : selectedSceneID.value, {
      expected_version: draft.value.version || 0, action,
      title: sceneForm.value?.title || "新分镜", visual_prompt: sceneForm.value?.visual_prompt || "",
      narration: sceneForm.value?.narration || "", duration_seconds: sceneForm.value?.duration_seconds || 5, order,
    });
    acceptDraft(value);
    if (action === "create" || action === "copy") selectScene(value.story.scenes[before]?.id || value.story.scenes.at(-1)?.id || "");
    message.value = "分镜已保存。";
  } catch (error) { message.value = (error as Error).message; }
}

async function deleteSelectedScene() {
  if (!selectedSceneID.value || !window.confirm("删除当前分镜？删除前会保留可恢复版本。")) return;
  try {
    await requireSaved();
    acceptDraft(await api.deleteScene(workspace.projectId, draft.value.id, selectedSceneID.value, draft.value.version || 0));
    message.value = "分镜已删除，可从历史版本恢复。";
  } catch (error) { message.value = (error as Error).message; }
}

async function importText(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  try {
    importError.value = "";
    const bytes = await file.arrayBuffer();
    importPreview.value = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch { importError.value = "无法读取 UTF-8 文本，正文未更改。"; }
  (event.target as HTMLInputElement).value = "";
}
function applyImport(action: "append" | "replace") {
  if (importPreview.value === null) return;
  draft.value.story.body = action === "append" && draft.value.story.body.trim()
    ? `${draft.value.story.body}\n\n${importPreview.value}` : importPreview.value;
  importPreview.value = null;
}
</script>

<template>
  <main class="studio-page creative-workspace">
    <header class="studio-head">
      <div><input v-model="draft.name" class="title-input" aria-label="草稿名称" /><p>{{ workspace.projectName || "新项目" }} · {{ saveState === "saved" ? "已保存" : saveState === "saving" ? "保存中" : saveState === "failed" ? "保存失败" : "尚未保存" }}</p></div>
      <div class="actions"><button v-if="saveState === 'failed'" @click="saveNow">重试保存</button><RouterLink class="button" to="/tasks">任务中心</RouterLink></div>
    </header>
    <nav class="workspace-tabs" aria-label="创作阶段">
      <button :class="{ active: activeTab === 'body' }" @click="activeTab = 'body'">正文</button>
      <button :class="{ active: activeTab === 'scenes' }" @click="activeTab = 'scenes'">分镜 <span>{{ draft.story.scenes.length }}</span></button>
      <button :class="{ active: activeTab === 'video' }" @click="activeTab = 'video'">视频</button>
    </nav>
    <p v-if="saveState === 'failed'" class="notice error" role="alert">保存失败。请重试保存，当前编辑内容仍在页面中。</p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>

    <section v-if="activeTab === 'body'" class="workspace-body">
      <div class="section-head"><h2>正文</h2><div class="actions"><button @click="importInput?.click()">导入文本</button><button @click="openDrawer">故事联想</button><button class="primary" :disabled="!canGenerateStoryboard" @click="generate('storyboard')">生成分镜</button></div></div>
      <input ref="importInput" class="file-input" type="file" accept=".txt,text/plain" aria-label="导入 UTF-8 文本" @change="importText" />
      <p v-if="importError" role="alert">{{ importError }}</p>
      <div v-if="importPreview !== null" class="notice"><strong>导入预览</strong><p>{{ importPreview.slice(0, 500) }}</p><button @click="applyImport('append')">追加到末尾</button><button @click="applyImport('replace')">替换正文</button><button @click="importPreview = null">取消</button></div>
      <textarea v-model="draft.story.body" class="body-editor" aria-label="故事正文" placeholder="写下或粘贴你的故事，也可以从一个想法开始。" />
      <div class="editor-footer">{{ draft.story.body.length }} 字 · {{ saveState === "saved" ? "已保存" : saveState === "failed" ? "保存失败" : "自动保存" }}</div>
      <div v-if="latestStoryboard" class="candidate-card"><h3>分镜候选 · {{ latestStoryboard.scenes?.length || 0 }} 个镜头</h3><p v-for="scene in latestStoryboard.scenes" :key="scene.order">{{ scene.order }}. {{ scene.title }} · {{ scene.duration_seconds }} 秒</p><button class="primary" @click="applyCandidate(latestStoryboard, 'replace')">{{ candidateNeedsReview === latestStoryboard.id ? "核对后确认替换" : "应用分镜" }}</button></div>
    </section>

    <section v-else-if="activeTab === 'scenes'" class="workspace-scenes">
      <div class="section-head"><h2>分镜 · {{ draft.story.scenes.length }}</h2><div class="actions"><button @click="activeTab = 'body'">返回正文</button><button @click="sceneAction('create')">新增分镜</button><button class="primary" :disabled="!draft.story.scenes.length || !videoCapability?.supported" @click="generateBatchVideo">批量生成视频</button></div></div>
      <p v-if="draft.storyboard_source_body && draft.storyboard_source_body !== draft.story.body" class="notice">正文已修改，当前分镜需要复核；可保留并编辑镜头，或重新生成分镜候选。</p>
      <ul v-if="batchFailures.length" class="notice error"><li v-for="(failure,index) in batchFailures" :key="index">{{ failure.scene_id }} · {{ failure.field }}：{{ failure.message }}</li></ul>
      <div class="scene-layout" :class="{ 'has-selection': !!sceneForm }"><div class="scene-list"><p v-if="!draft.story.scenes.length" class="empty">先从正文生成分镜，也可以手动新增。</p><button v-for="scene in draft.story.scenes" :key="scene.id" class="scene-row" :class="{ active: scene.id === selectedSceneID }" @click="selectScene(scene.id)"><strong>{{ scene.order }}. {{ scene.title }}</strong><span>{{ scene.duration_seconds }} 秒</span><small>{{ scene.visual_prompt }}</small></button></div>
        <aside v-if="sceneForm" class="panel scene-inspector"><button class="mobile-panel-back" @click="selectedSceneID = ''; sceneForm = null">返回镜头列表</button><h3>镜头 {{ selectedScene?.order }}</h3><label>标题<input v-model="sceneForm.title" /></label><label>画面描述<textarea v-model="sceneForm.visual_prompt" rows="4" /></label><label>旁白<textarea v-model="sceneForm.narration" rows="3" /></label><label>时长（秒）<input v-model.number="sceneForm.duration_seconds" type="number" min="1" /></label><div class="actions"><button @click="sceneAction('move', Math.max(1, (selectedScene?.order || 1) - 1))">上移</button><button @click="sceneAction('move', Math.min(draft.story.scenes.length, (selectedScene?.order || 1) + 1))">下移</button><button @click="sceneAction('copy')">复制</button><button @click="deleteSelectedScene">删除</button></div><button class="primary" @click="sceneAction('update')">保存镜头</button>
          <h3>镜头素材</h3><p>选择确认后才绑定到当前镜头。</p>
          <div v-for="usage in (Object.keys(usageLabels) as SceneBinding['usage'][])" :key="usage" class="binding-row"><span>{{ usageLabels[usage] }}</span><strong>{{ materials.find((item) => item.id === sceneBindings.find((binding) => binding.usage === usage)?.material_id)?.name || "未绑定" }}</strong><button @click="openPicker(usage)">选择</button><button v-if="sceneBindings.some((binding) => binding.usage === usage)" :disabled="bindingBusy" @click="changeBinding(usage)">解绑</button></div>
          <div v-if="pickerUsage" class="candidate-card"><h3>选择{{ usageLabels[pickerUsage] }}</h3><p v-if="!pickerMaterials.length">当前类别没有素材，可在这里上传。</p><select v-else v-model="pickerChoice" aria-label="待绑定素材"><option value="">请选择</option><option v-for="item in pickerMaterials" :key="item.id" :value="item.id">{{ item.name }}</option></select><input ref="pickerFile" class="file-input" type="file" :accept="pickerUsage === 'driving_audio' || pickerUsage === 'voiceover' ? 'audio/*' : 'image/*'" @change="uploadForScene" /><div class="actions"><button :disabled="pickerUploading" @click="pickerFile?.click()">{{ pickerUploading ? `上传中 ${pickerProgress}%` : "上传新素材" }}</button><button @click="pickerUsage = ''">取消</button><button class="primary" :disabled="!pickerChoice || bindingBusy || pickerUploading" @click="changeBinding(pickerUsage as SceneBinding['usage'], pickerChoice)">确认绑定</button></div><RouterLink to="/materials">打开素材库</RouterLink></div>
          <h3>视频模型</h3><label>模型<select v-model="selectedVideoModel"><option v-for="model in videoModels" :key="model.id" :value="model.id">{{ model.name }}</option></select></label><label>清晰度<select v-model="videoResolution"><option value="720P">720P</option><option value="1080P">1080P</option></select></label><p v-if="videoCapability && !videoCapability.supported" class="notice error">{{ videoCapability.reason }}</p><p v-else-if="videoCapability?.capability">支持首帧、可选尾帧和驱动音频；{{ videoCapability.capability.min_duration }}–{{ videoCapability.capability.max_duration }} 秒。</p><button :disabled="!selectedVideoModel" @click="checkVideoScene">检查生成条件</button><ul v-if="validationIssues.length" class="notice error"><li v-for="issue in validationIssues" :key="issue.field + issue.message">{{ issue.field }}：{{ issue.message }}</li></ul>
          <p v-if="sceneBindings.some((binding) => ['character_reference','scene_reference','prop_reference'].includes(binding.usage))" class="notice">当前模型不支持参考图参与视频生成；绑定会保留，提交时需处理。</p>
          <button class="primary" :disabled="!videoCapability?.supported" @click="generateSceneVideo">生成当前镜头视频</button>
          <h3>生成任务</h3><p v-if="!selectedSceneTasks.length">暂无镜头任务。</p><div v-for="item in selectedSceneTasks" :key="item.id" class="task-row"><span>{{ item.status }} · {{ item.progress }}%</span><button v-if="item.status === 'failed'" @click="retryVideoTask(item.id)">重试</button><small v-if="item.error">{{ item.error }}</small></div>
          <h3>历史版本</h3><p v-if="!selectedSceneVersions.length">新版本完成后会显示在这里；到达时不会自动选用。</p><div v-for="version in selectedSceneVersions" :key="version.id" class="candidate-card"><video :src="api.mediaUrl(version.local_media_path)" controls preload="metadata" /><small>{{ new Date(version.created_at).toLocaleString() }} · {{ version.duration_seconds }} 秒</small><p v-if="version.based_on_old_settings" class="notice">基于旧设置生成</p><button class="primary" :disabled="draft.selected_versions?.[selectedSceneID] === version.id" @click="chooseVersion(version.id)">{{ draft.selected_versions?.[selectedSceneID] === version.id ? "已选用" : "选用此版本" }}</button><button v-if="draft.selected_versions?.[selectedSceneID] === version.id" @click="chooseVersion('')">取消选用</button></div>
        </aside>
      </div>
      <div v-if="draft.storyboard_snapshots?.length" class="history-strip"><h3>分镜历史</h3><button v-for="snapshot in draft.storyboard_snapshots" :key="snapshot.id" @click="restore(snapshot.id)">{{ snapshot.scenes.length }} 个镜头 · 恢复</button></div>
    </section>

    <section v-else class="workspace-video"><div class="section-head"><h2>视频</h2><RouterLink to="/tasks">查看生成任务</RouterLink></div>
      <video v-if="playingURL" class="composition-player" :src="playingURL" controls playsinline preload="metadata" aria-label="视频预览" /><p v-else class="empty">选择一个镜头视频或历史成片预览。</p>
      <h3>镜头顺序</h3><div class="video-sequence"><div v-for="entry in orderedClips" :key="entry.scene.id" class="scene-row"><strong>{{ entry.scene.order }}. {{ entry.scene.title }}</strong><span>{{ entry.version ? "已选版本" : "缺少已选视频" }}</span><button v-if="entry.version" @click="playingURL = api.mediaUrl(entry.version.local_media_path)">播放片段</button><button v-else @click="selectScene(entry.scene.id); activeTab = 'scenes'">前往镜头</button></div></div>
      <div class="panel composition-settings"><h3>成片设置</h3><label>背景音乐<select v-model="musicMaterialID"><option value="">不使用音乐</option><option v-for="item in musicMaterials" :key="item.id" :value="item.id">{{ item.name }}</option></select></label><label>背景音乐音量 {{ Math.round(musicVolume * 100) }}%<input v-model.number="musicVolume" type="range" min="0" max="1" step="0.05" /></label><label>原片音量 {{ Math.round(sourceVolume * 100) }}%<input v-model.number="sourceVolume" type="range" min="0" max="1" step="0.05" /></label><label>后期配音音量 {{ Math.round(voiceVolume * 100) }}%<input v-model.number="voiceVolume" type="range" min="0" max="1" step="0.05" /></label><label>画幅<select v-model="outputAspect"><option value="16:9">横屏 16:9</option><option value="9:16">竖屏 9:16</option></select></label><label>输出清晰度<select v-model="outputResolution"><option value="720P">720P</option><option value="1080P">1080P</option></select></label><button class="primary" :disabled="!canCompose" @click="submitComposition">导出完整成片</button><p v-if="!canCompose">每个镜头都要先选用一个可播放的视频版本。</p></div>
      <ul v-if="compositionIssues.length" class="notice error"><li v-for="(issue,index) in compositionIssues" :key="index"><button v-if="issue.scene_id" @click="selectScene(issue.scene_id);activeTab='scenes'">镜头 {{ issue.scene_id }}</button>{{ issue.field }}：{{ issue.message }}</li></ul>
      <h3>成片历史</h3><p v-if="!draft.compositions?.length">完成的成片将保留在这里。</p><div v-for="composition in draft.compositions" :key="composition.id" class="candidate-card"><strong>{{ new Date(composition.created_at).toLocaleString() }} · {{ composition.resolution }}</strong><p v-if="compositionIsOld(composition)" class="notice">有未合成修改</p><button @click="playingURL = api.mediaUrl(composition.local_media_path)">播放成片</button><button @click="api.downloadTask(composition.task_id, 'composition')">下载成片</button><small>使用 {{ composition.clip_version_ids.length }} 个镜头版本</small></div>
      <div v-for="item in compositionTasks" :key="item.id" class="task-row"><span>成片任务 {{ item.status }} · {{ item.progress }}%</span><button v-if="item.status === 'failed'" @click="retryVideoTask(item.id)">重试</button><small v-if="item.error">{{ item.error }}</small></div>
    </section>

    <aside v-if="drawerOpen" ref="drawerElement" class="story-drawer" role="dialog" aria-modal="true" aria-label="故事联想" @keydown="drawerKeydown"><div class="section-head"><h2>故事联想</h2><button aria-label="关闭故事联想" @click="closeDrawer">关闭</button></div><p>从一个想法，展开一个故事。</p><div class="actions"><button :class="{ active: drawerMode === 'create' }" @click="drawerMode = 'create'">新建故事</button><button :class="{ active: drawerMode === 'revise' }" :disabled="!draft.story.body.trim()" @click="drawerMode = 'revise'">修改正文</button></div><label>你的想法或修改要求<textarea v-model="instruction" rows="5" /></label><label>故事模型<select v-model="selectedStoryModel"><option v-for="model in storyModels" :key="model.id" :value="model.id">{{ model.name }}</option></select></label><button class="primary" :disabled="!instruction.trim() || !selectedStoryModel || saveState === 'failed'" @click="generate('story')">生成正文</button><div v-if="latestStory" class="candidate-card"><h3>生成预览</h3><p>{{ latestStory.body }}</p><button @click="applyCandidate(latestStory, 'append')">追加到正文</button><button class="primary" @click="applyCandidate(latestStory, 'replace')">{{ candidateNeedsReview === latestStory.id ? "核对后确认替换" : "替换正文" }}</button></div><div v-if="draft.body_snapshots?.length" class="history-strip"><h3>应用记录</h3><button v-for="snapshot in draft.body_snapshots" :key="snapshot.id" @click="restore(snapshot.id)">恢复应用前正文</button></div></aside>
  </main>
</template>
