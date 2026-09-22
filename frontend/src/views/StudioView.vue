<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { useWorkspaceStore } from "../stores/workspace";
import { useTaskStore } from "../stores/tasks";
import type {
  Capability,
  Material,
  MaterialKind,
  Model,
  TaskEvent,
  TaskInput,
} from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
const workspace = useWorkspaceStore(),
  tasks = useTaskStore(),
  route = useRoute(),
  router = useRouter(),
  message = ref(""),
  pendingStoryResult = ref(""),
  providers = ref<Record<Capability, Model[]>>({
    story: [],
    image: [],
    video: [],
  }),
  materials = ref<Material[]>([]),
  videoSourceURL = ref("");
const storySnapshots = new Map<string, string>();
let unsubscribe = () => {};
const tabs: { id: MaterialKind; label: string }[] = [
  { id: "visual", label: "画面" },
  { id: "character", label: "角色" },
  { id: "frame", label: "首尾帧" },
  { id: "voice", label: "配音" },
  { id: "music", label: "音乐" },
];
const usesImage = computed(
  () =>
    workspace.activeResource === "character" ||
    workspace.activeResource === "frame",
);
const generatedImages = computed(() =>
  tasks.items.filter(
    (item) =>
      item.project_id === workspace.projectId &&
      item.draft_id === workspace.draft.id &&
      item.kind === "image" &&
      item.status === "succeeded" &&
      item.result_url,
  ),
);
const sourceImage = computed(
  () =>
    videoSourceURL.value ||
    (workspace.activeResource === "frame"
      ? materials.value.find(
          (item) => item.id === workspace.selectedMaterials.frame,
        )?.url
      : "") ||
    "",
);
onMounted(async () => {
  const projectID = String(route.params.projectId || "");
  const draftID = String(route.params.draftId || "");
  try {
    if (projectID && draftID) await workspace.load(projectID, draftID);
    else workspace.startNew(sessionStorage.getItem("frameflow-idea") || "");
  } catch (error) {
    message.value = (error as Error).message;
  }
  unsubscribe = tasks.subscribe((event) => void handleTaskEvent(event));
  for (const key of ["story", "image", "video"] as Capability[])
    providers.value[key] = (await api.models(key)).models.filter(
      (m) => m.enabled && m.supported,
    );
  workspace.storyProvider ||=
    providers.value.story.find((v) => v.default)?.id ||
    providers.value.story[0]?.id ||
    "";
  workspace.imageProvider ||=
    providers.value.image.find((v) => v.default)?.id ||
    providers.value.image[0]?.id ||
    "";
  workspace.videoProvider ||=
    providers.value.video.find((v) => v.default)?.id ||
    providers.value.video[0]?.id ||
    "";
  await loadMaterials();
  await tasks.load();
});
onUnmounted(() => unsubscribe());

async function handleTaskEvent(event: TaskEvent) {
  if (
    event.status !== "succeeded" ||
    event.project_id !== workspace.projectId ||
    event.draft_id !== workspace.draft.id
  )
    return;
  const item = tasks.items.find((task) => task.id === event.task_id);
  if (item?.kind !== "story") return;
  try {
    const project = await api.project(workspace.projectId);
    const remote = project.drafts.find(
      (draft) => draft.id === workspace.draft.id,
    );
    if (!remote) return;
    const submittedBody = storySnapshots.get(event.task_id);
    if (!workspace.mergeGeneratedDraft(remote, submittedBody)) {
      pendingStoryResult.value = "";
      message.value = "生成结果已填充到策划正文。";
    } else {
      pendingStoryResult.value = remote.story.body;
      message.value =
        "生成完成，但正文在生成期间已修改，请确认是否应用生成结果。";
    }
    storySnapshots.delete(event.task_id);
  } catch (error) {
    message.value = (error as Error).message;
  }
}

function applyGeneratedStory() {
  if (!pendingStoryResult.value) return;
  workspace.draft.story.body = pendingStoryResult.value;
  pendingStoryResult.value = "";
  message.value = "生成结果已填充到策划正文。";
}
async function loadMaterials() {
  materials.value = (
    await api.materials(workspace.activeResource as MaterialKind)
  ).materials;
}
function setTab(id: MaterialKind) {
  workspace.activeResource = id;
  loadMaterials();
}
async function enqueue(kind: "story" | "image" | "video", modelID: string) {
  try {
    const submittedBody = workspace.draft.story.body;
    await workspace.save();
    await router.replace(
      "/studio/" + workspace.projectId + "/" + workspace.draft.id,
    );
    const created = await tasks.create(
      workspace.projectId,
      workspace.draft.id,
      kind,
      modelID,
      generationInput(kind),
    );
    if (kind === "story") storySnapshots.set(created.id, submittedBody);
    message.value = "任务已进入后台队列，可以继续编辑。";
  } catch (e) {
    message.value = (e as Error).message;
  }
}

function generationInput(kind: "story" | "image" | "video"): TaskInput {
  const selectedID = workspace.selectedMaterials[workspace.activeResource];
  const selected = materials.value.find((item) => item.id === selectedID);
  const prompt =
    kind === "story"
      ? workspace.idea
      : workspace.draft.story.body || workspace.idea || workspace.draft.name;
  return {
    prompt,
    aspect_ratio: workspace.aspectRatio,
    source_image_url:
      kind === "video" ? sourceImage.value || selected?.url : undefined,
  };
}
async function saveDraft() {
  try {
    await workspace.save();
    await router.replace(
      "/studio/" + workspace.projectId + "/" + workspace.draft.id,
    );
    message.value = "草稿已保存。";
  } catch (e) {
    message.value = (e as Error).message;
  }
}
</script>
<template>
  <main class="studio-page">
    <div class="studio-head">
      <div>
        <input
          v-model="workspace.draft.name"
          class="title-input"
          aria-label="项目名称"
        />
        <div class="steps">
          <span>01 故事与脚本</span><b>02 分镜与素材</b><span>03 视频生成</span>
        </div>
      </div>
      <div class="actions">
        <button @click="saveDraft">保存草稿</button
        ><RouterLink class="button" to="/tasks"
          >后台任务
          <span v-if="tasks.running.length">{{
            tasks.running.length
          }}</span></RouterLink
        >
      </div>
    </div>
    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="pendingStoryResult" class="notice story-conflict">
      <span>已保护当前编辑内容。</span>
      <button class="primary" @click="applyGeneratedStory">应用生成结果</button>
    </div>
    <div class="studio-grid">
      <aside class="panel">
        <div class="panel-head">
          <h2><i>01</i> 故事与脚本</h2>
          <small>从创意生成可编辑脚本</small>
        </div>
        <div class="panel-body">
          <div class="assistant-message">
            接口切换只影响下一次生成，已有编辑不会被清空。
          </div>
          <label
            >视频想法 / 修改要求<textarea
              v-model="workspace.idea"
              rows="7"
              placeholder="描述主题、画面和节奏"
            />
          </label>
          <div class="provider-box">
            <div>
              <strong>故事 AI 接口</strong
              ><RouterLink to="/config">管理</RouterLink>
            </div>
            <select v-model="workspace.storyProvider" aria-label="故事 AI 接口">
              <option value="">请选择接口</option>
              <option v-for="p in providers.story" :key="p.id" :value="p.id">
                {{ p.name }} · {{ p.remote_id }}
              </option></select
            ><small>仅用于故事与分镜生成</small>
          </div>
          <button
            class="primary full"
            :disabled="!workspace.storyProvider"
            @click="enqueue('story', workspace.storyProvider)"
          >
            生成故事与分镜
          </button>
        </div>
      </aside>
      <section class="panel editor">
        <div class="panel-head split">
          <h2><i>02</i> 脚本与分镜</h2>
          <StatusBadge status="enabled" />
        </div>
        <div class="panel-body">
          <label
            >策划正文<textarea
              v-model="workspace.draft.story.body"
              rows="9"
              placeholder="【内容概览】&#10;【角色列表】&#10;【分镜脚本】"
            />
          </label>
          <div class="section-head">
            <h3>
              分镜列表
              <span class="count">{{
                workspace.draft.story.scenes.length
              }}</span>
            </h3>
            <button class="quiet" @click="workspace.addScene">新增分镜</button>
          </div>
          <article
            v-for="scene in workspace.draft.story.scenes"
            :key="scene.id"
            class="scene-card"
          >
            <div class="scene-title">
              <strong>分镜 {{ scene.order }}</strong
              ><input
                v-model.number="scene.duration_seconds"
                type="number"
                min="1"
              />
              秒
            </div>
            <input
              v-model="scene.title"
              aria-label="分镜标题"
              placeholder="分镜标题"
            /><textarea
              v-model="scene.visual_prompt"
              rows="2"
              placeholder="画面描述"
            /><textarea v-model="scene.narration" rows="2" placeholder="旁白" />
          </article>
          <div v-if="!workspace.draft.story.scenes.length" class="empty">
            还没有分镜，可以新增或让故事助手生成。
          </div>
        </div>
      </section>
      <section class="panel generation-panel">
        <div class="panel-head split">
          <h2><i>03</i> 生成设置</h2>
          <RouterLink class="text-link" to="/materials">管理素材</RouterLink>
        </div>
        <div class="panel-body generation-grid">
          <label
            >素材类型<select
              v-model="workspace.activeResource"
              aria-label="素材类型"
              @change="setTab(workspace.activeResource as MaterialKind)"
            >
              <option v-for="tab in tabs" :key="tab.id" :value="tab.id">
                {{ tab.label }}
              </option>
            </select></label
          >
          <label
            >选用素材<select
              v-model="workspace.selectedMaterials[workspace.activeResource]"
              aria-label="选用素材"
            >
              <option value="">暂不选择</option>
              <option v-for="item in materials" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select></label
          >
          <label v-if="workspace.activeResource === 'visual'"
            >视频比例<select v-model="workspace.aspectRatio">
              <option>9:16</option>
              <option>16:9</option>
              <option>1:1</option>
            </select></label
          >
          <label v-if="workspace.activeResource === 'visual'" class="toggle"
            ><input
              v-model="workspace.subtitles"
              type="checkbox"
            />匹配字幕</label
          >
          <label v-if="workspace.activeResource === 'music'"
            >背景音乐音量：{{ workspace.bgmVolume }}%<input
              v-model.number="workspace.bgmVolume"
              type="range"
              min="0"
              max="100"
          /></label>
          <div v-if="usesImage" class="provider-box compact-provider">
            <strong>生图 AI 接口</strong>
            <select v-model="workspace.imageProvider" aria-label="生图 AI 接口">
              <option value="">请选择接口</option>
              <option v-for="p in providers.image" :key="p.id" :value="p.id">
                {{ p.name }} · {{ p.remote_id }}
              </option>
            </select>
            <button
              class="full"
              :disabled="!workspace.imageProvider"
              @click="enqueue('image', workspace.imageProvider)"
            >
              生成{{
                workspace.activeResource === "character" ? "角色图片" : "首尾帧"
              }}
            </button>
          </div>
          <div class="provider-box compact-provider video-generation">
            <strong>视频生成 AI 接口</strong>
            <select
              v-model="workspace.videoProvider"
              aria-label="视频生成 AI 接口"
            >
              <option value="">请选择接口</option>
              <option v-for="p in providers.video" :key="p.id" :value="p.id">
                {{ p.name }} · {{ p.remote_id }}
              </option>
            </select>
            <label v-if="generatedImages.length"
              >已生成的首帧图片<select
                v-model="videoSourceURL"
                aria-label="已生成的首帧图片"
              >
                <option value="">选择图片或填写下方地址</option>
                <option
                  v-for="item in generatedImages"
                  :key="item.id"
                  :value="item.result_url"
                >
                  {{ new Date(item.created_at).toLocaleString() }} ·
                  {{ item.input.prompt.slice(0, 32) }}
                </option>
              </select></label
            >
            <label
              >首帧图片 URL<input
                v-model="videoSourceURL"
                placeholder="https://... 或选择上方图片"
            /></label>
            <button
              class="primary full"
              :disabled="
                !workspace.videoProvider ||
                !workspace.draft.story.scenes.length ||
                !sourceImage
              "
              @click="enqueue('video', workspace.videoProvider)"
            >
              检查并生成视频
            </button>
          </div>
        </div>
      </section>
    </div>
  </main>
</template>
