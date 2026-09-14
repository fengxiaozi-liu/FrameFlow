<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import { useWorkspaceStore } from "../stores/workspace";
import { useTaskStore } from "../stores/tasks";
import type {
  Capability,
  Material,
  MaterialKind,
  Provider,
  TaskInput,
} from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
const workspace = useWorkspaceStore(),
  tasks = useTaskStore(),
  message = ref(""),
  providers = ref<Record<Capability, Provider[]>>({
    story: [],
    image: [],
    video: [],
  }),
  materials = ref<Material[]>([]),
  fileInput = ref<HTMLInputElement>(),
  uploading = ref(false);
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
const uploadAccept = computed(() =>
  workspace.activeResource === "voice" || workspace.activeResource === "music"
    ? "audio/*"
    : "image/*",
);
onMounted(async () => {
  workspace.idea = sessionStorage.getItem("frameflow-idea") || workspace.idea;
  for (const key of ["story", "image", "video"] as Capability[])
    providers.value[key] = (await api.providers(key)).providers;
  workspace.storyProvider ||=
    providers.value.story.find((v) => v.status === "healthy")?.code || "";
  workspace.imageProvider ||=
    providers.value.image.find((v) => v.status === "healthy")?.code || "";
  workspace.videoProvider ||=
    providers.value.video.find((v) => v.status === "healthy")?.code || "";
  await loadMaterials();
});
async function loadMaterials() {
  materials.value = (
    await api.materials(workspace.activeResource as MaterialKind)
  ).materials;
}
function setTab(id: MaterialKind) {
  workspace.activeResource = id;
  loadMaterials();
}
function chooseMaterial() {
  fileInput.value?.click();
}
async function uploadMaterial(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  const kind = workspace.activeResource as MaterialKind;
  uploading.value = true;
  message.value = "";
  try {
    const material = await api.uploadMaterial(file, kind);
    await loadMaterials();
    workspace.selectedMaterials[kind] = material.id;
    message.value = `素材“${material.name}”已上传并选中。`;
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    uploading.value = false;
    input.value = "";
  }
}
async function enqueue(kind: "story" | "image" | "video", provider: string) {
  try {
    await tasks.create(kind, provider, generationInput(kind));
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
    source_image_url: kind === "video" ? selected?.url : undefined,
  };
}
async function saveDraft() {
  try {
    if (!workspace.projectId) {
      const project = await api.createProject(workspace.draft.name);
      workspace.projectId = project.id;
    }
    workspace.draft.story.updated_at = new Date().toISOString();
    await api.saveDraft(workspace.projectId, workspace.draft);
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
              <option
                v-for="p in providers.story"
                :key="p.code"
                :value="p.code"
              >
                {{ p.name }} · {{ p.model }}
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
      <aside class="panel resources">
        <div class="panel-head">
          <h2>素材与生成设置</h2>
          <div class="tabs">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              :class="{ active: workspace.activeResource === tab.id }"
              @click="setTab(tab.id)"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>
        <div class="panel-body">
          <h3>
            {{ tabs.find((v) => v.id === workspace.activeResource)?.label }}设置
          </h3>
          <div v-if="usesImage" class="provider-box">
            <strong>生图 AI 接口</strong
            ><select
              v-model="workspace.imageProvider"
              aria-label="生图 AI 接口"
            >
              <option value="">请选择接口</option>
              <option
                v-for="p in providers.image"
                :key="p.code"
                :value="p.code"
              >
                {{ p.name }} · {{ p.model }}
              </option></select
            ><small>角色和首尾帧共用此选择</small>
          </div>
          <label v-if="workspace.activeResource === 'visual'"
            >视频比例<select v-model="workspace.aspectRatio">
              <option>9:16</option>
              <option>16:9</option>
              <option>1:1</option>
            </select></label
          ><label v-if="workspace.activeResource === 'visual'" class="toggle"
            ><input
              v-model="workspace.subtitles"
              type="checkbox"
            />匹配字幕</label
          ><label v-if="workspace.activeResource === 'frame'"
            >图片来源<select v-model="workspace.frameSource">
              <option value="generated">AI 生成</option>
              <option value="upload">上传图片</option>
              <option value="library">素材库</option>
            </select></label
          ><label v-if="workspace.activeResource === 'music'"
            >背景音乐音量：{{ workspace.bgmVolume }}%<input
              v-model.number="workspace.bgmVolume"
              type="range"
              min="0"
              max="100"
          /></label>
          <div class="material-list">
            <button
              v-for="item in materials"
              :key="item.id"
              :class="[
                'material-item',
                {
                  selected:
                    workspace.selectedMaterials[workspace.activeResource] ===
                    item.id,
                },
              ]"
              @click="
                workspace.selectedMaterials[workspace.activeResource] = item.id
              "
            >
              <span class="material-preview">{{ item.name.slice(0, 1) }}</span
              ><strong>{{ item.name }}</strong>
            </button>
          </div>
          <button
            v-if="!materials.length"
            type="button"
            class="empty material-upload-empty"
            :disabled="uploading"
            @click="chooseMaterial"
          >
            <strong>当前素材库为空</strong>
            <span
              >点击上传{{ uploadAccept === "audio/*" ? "音频" : "图片" }}</span
            >
          </button>
          <input
            ref="fileInput"
            class="file-input"
            type="file"
            :accept="uploadAccept"
            aria-label="选择要上传的素材"
            @change="uploadMaterial"
          />
          <button
            type="button"
            class="full"
            :disabled="uploading"
            @click="chooseMaterial"
          >
            {{ uploading ? "正在上传…" : "上传素材" }}
          </button>
          <button
            v-if="usesImage && workspace.frameSource !== 'upload'"
            class="full"
            :disabled="!workspace.imageProvider"
            @click="enqueue('image', workspace.imageProvider)"
          >
            生成{{
              workspace.activeResource === "character" ? "角色图片" : "首尾帧"
            }}
          </button>
        </div>
        <footer>
          <div class="provider-box">
            <strong>视频生成 AI 接口</strong
            ><select
              v-model="workspace.videoProvider"
              aria-label="视频生成 AI 接口"
            >
              <option value="">请选择接口</option>
              <option
                v-for="p in providers.video"
                :key="p.code"
                :value="p.code"
              >
                {{ p.name }} · {{ p.model }}
              </option>
            </select>
          </div>
          <button
            class="primary full"
            :disabled="
              !workspace.videoProvider || !workspace.draft.story.scenes.length
            "
            @click="enqueue('video', workspace.videoProvider)"
          >
            检查并生成视频
          </button>
        </footer>
      </aside>
    </div>
  </main>
</template>
