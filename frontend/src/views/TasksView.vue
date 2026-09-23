<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import { useTaskStore } from "../stores/tasks";
import type { Project, Task, TaskStatus } from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
import ProgressBar from "../components/ProgressBar.vue";
const store = useTaskStore(),
  filter = ref<"all" | TaskStatus>("all"),
  message = ref(""),
  projects = ref<Project[]>([]);
const visible = computed(() =>
  filter.value === "all"
    ? store.items
    : store.items.filter((v) => v.status === filter.value),
);
const kindLabel: Record<Task["kind"], string> = {
  story: "文本",
  image: "图片",
  video: "视频",
  composition: "成片",
};
const groups = computed(() => {
  const values = new Map<string, Task[]>();
  for (const item of visible.value) {
    const key = item.project_id || "";
    values.set(key, [...(values.get(key) || []), item]);
  }
  return [...values].map(([id, items]) => ({
    id: id || "unassigned",
    name:
      projects.value.find((project) => project.id === id)?.name ||
      (id ? "未知项目" : "未归属任务"),
    tasks: items,
  }));
});
function draftLabel(item: Task) {
  const project = projects.value.find((value) => value.id === item.project_id);
  return (
    project?.drafts.find((draft) => draft.id === item.draft_id)?.name ||
    item.draft_id ||
    "未归属草稿"
  );
}
function sceneLink(item: Task) {
  if (!item.project_id || !item.draft_id || !item.scene_id) return "";
  const draft = projects.value.find((value) => value.id === item.project_id)?.drafts.find((value) => value.id === item.draft_id);
  return draft?.story.scenes.some((scene) => scene.id === item.scene_id)
    ? `/studio/${encodeURIComponent(item.project_id)}/${encodeURIComponent(item.draft_id)}?scene=${encodeURIComponent(item.scene_id)}` : "";
}
function savedMedia(task: Task) {
  if (task.kind !== "image" && task.kind !== "video" && task.kind !== "composition") return false;
  const ext = task.kind === "image" ? "png" : "mp4";
  return task.result_url === `/media/${task.kind === "composition" ? "composition" : "generated"}-${task.id}.${ext}`;
}
function previewURL(task: Task) {
  const url = task.result_url || "";
  if (url.startsWith("/media/")) return api.mediaUrl(url);
  if (url.startsWith("https://")) return url;
  return "";
}
function sourcePoster(task: Task) {
  const url = task.input?.source_image_url || "";
  if (url.startsWith("/media/")) return api.mediaUrl(url);
  return url.startsWith("https://") ? url : undefined;
}
async function download(task: Task) {
  if (task.kind !== "image" && task.kind !== "video" && task.kind !== "composition") return;
  message.value = "";
  try {
    await api.downloadTask(task.id, task.kind);
  } catch (error) {
    message.value = (error as Error).message;
  }
}
onMounted(async () => {
  await Promise.all([
    store.load(),
    api.projects().then((value) => (projects.value = value.projects)),
  ]);
});
async function cancel(item: Task) {
  try {
    Object.assign(item, await api.cancelTask(item.id));
  } catch (e) {
    message.value = (e as Error).message;
  }
}
async function retry(item: Task) {
  try {
    Object.assign(item, await api.retryTask(item.id));
    await store.load();
  } catch (e) {
    message.value = (e as Error).message;
  }
}
</script>
<template>
  <main class="page">
    <div class="page-intro split">
      <div>
        <h1>任务中心</h1>
      </div>
      <button @click="store.load()">刷新状态</button>
    </div>
    <div v-if="message || store.error" class="notice error">
      {{ message || store.error }}
    </div>
    <div class="filters">
      <button
        v-for="item in [
          { id: 'all', label: '全部' },
          { id: 'running', label: '执行中' },
          { id: 'queued', label: '排队中' },
          { id: 'succeeded', label: '已完成' },
          { id: 'failed', label: '失败' },
          { id: 'cancelled', label: '已取消' },
        ]"
        :key="item.id"
        :class="{ active: filter === item.id }"
        @click="filter = item.id as typeof filter"
      >
        {{ item.label }}
      </button>
    </div>
    <div class="task-list">
      <section
        v-for="group in groups"
        :key="group.id"
        class="task-project-group"
      >
        <div class="section-head">
          <h2>{{ group.name }}</h2>
          <span>{{ group.tasks.length }} 个任务</span>
        </div>
        <article
          v-for="task in group.tasks"
          :key="task.id"
          class="card task-detail"
        >
          <div class="split task-heading">
            <div>
              <h2>
                {{ kindLabel[task.kind] }}生成
                <span class="task-short-id">#{{ task.id.slice(-9) }}</span>
              </h2>
              <small
                >{{ draftLabel(task) }} ·
                {{ new Date(task.created_at).toLocaleString() }} ·
                {{ task.id }}</small
              >
              <RouterLink v-if="sceneLink(task)" :to="sceneLink(task)" class="text-link">查看镜头</RouterLink><small v-else-if="task.scene_id">原镜头已删除，结果仅保留在任务中心</small>
            </div>
            <StatusBadge :status="task.status" />
          </div>
          <div class="task-meta">
            <span>阶段：{{ task.stage }}</span
            ><span
              >模型：{{ task.model_id || task.provider_code || "未指定" }}</span
            ><strong>{{ task.progress }}%</strong>
          </div>
          <ProgressBar :value="task.progress" />
          <p v-if="task.error" class="error-text">{{ task.error }}</p>
          <p v-if="task.result_text" class="task-result-text">
            {{ task.result_text }}
          </p>
          <div
            v-if="
              task.status === 'succeeded' &&
              task.kind === 'image' &&
              previewURL(task)
            "
            class="task-preview image-preview"
          >
            <img
              :src="previewURL(task)"
              :alt="`${kindLabel[task.kind]}生成结果`"
              loading="lazy"
            />
          </div>
          <div
            v-if="
              task.status === 'succeeded' &&
              (task.kind === 'video' || task.kind === 'composition') &&
              previewURL(task)
            "
            class="task-preview video-preview"
          >
            <video
              :src="previewURL(task)"
              :poster="sourcePoster(task)"
              controls
              preload="metadata"
              playsinline
              aria-label="生成的视频"
            />
          </div>
          <p
            v-if="
              task.status === 'succeeded' &&
              task.kind !== 'story' &&
              !previewURL(task)
            "
            class="task-unavailable"
          >
            此任务没有保存可预览的媒体文件。
          </p>
          <div class="actions">
            <button
              v-if="task.status === 'queued' || task.status === 'running'"
              @click="cancel(task)"
            >
              取消任务</button
            ><button
              v-if="task.status === 'failed' || task.status === 'cancelled'"
              @click="retry(task)"
            >
              重新执行</button
            ><button
              v-if="task.status === 'succeeded' && savedMedia(task)"
              class="primary"
              @click="download(task)"
            >
              下载{{ kindLabel[task.kind] }}</button
            ><a
              v-else-if="task.status === 'succeeded' && previewURL(task)"
              :href="previewURL(task)"
              target="_blank"
              rel="noopener noreferrer"
              >查看原始文件</a
            >
          </div>
        </article>
      </section>
      <div v-if="!visible.length" class="empty">当前筛选下没有任务。</div>
    </div>
  </main>
</template>
