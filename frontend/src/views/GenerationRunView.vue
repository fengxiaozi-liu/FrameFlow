<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useTaskStore } from "../stores/tasks";
import { api } from "../api/client";
import { bindingsApi, type GenerationRun } from "../api/bindings";
type RunStep = NonNullable<GenerationRun["steps"]>[number];
import { useShotWorkspace } from "../composables/useShotWorkspace";
import GenerationRunStages from "../components/studio/GenerationRunStages.vue";
import ShotPreview from "../components/studio/ShotPreview.vue";
const { projectId, draftId, sceneId, base, draft, error, reload } =
  useShotWorkspace();
const runId = String(useRoute().params.runId),
  run = ref<GenerationRun>(),
  busy = ref(false),
  message = ref("");
let alive = true,
  timer: ReturnType<typeof setInterval> | undefined,
  inFlight = false;
const router = useRouter(),
  tasks = useTaskStore();
async function remove() {
  if (busy.value || run.value?.state !== "failed") return;
  if (
    !window.confirm("删除这条失败的生成记录及其子任务？已经生成的素材会保留。")
  )
    return;
  busy.value = true;
  message.value = "";
  try {
    await bindingsApi.deleteRun(projectId, draftId, sceneId, runId);
    alive = false;
    clearInterval(timer);
    await tasks.load();
    await router.replace({
      path: `${base}/shots/${encodeURIComponent(sceneId)}`,
      query: { deleted: "1" },
    });
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
const steps = computed(() => run.value?.steps || []);
const takes = computed(
  () =>
    draft.value?.media_takes?.filter((t) =>
      run.value?.task_ids.includes(t.task_id),
    ) || [],
);
const versions = computed(
  () =>
    draft.value?.video_versions?.filter((v) =>
      run.value?.task_ids.includes(v.task_id),
    ) || [],
);
const snapshot = computed(() => run.value?.plan?.[0]);
const snapshotFrames = computed(() => {
  const value = snapshot.value;
  if (value?.video_input_mode !== "first_frame") return [];
  const references = value.entities.flatMap((entity) => entity.references);
  return [
    { label: "首帧", materialId: value.first_frame_material_id },
    { label: "尾帧", materialId: value.last_frame_material_id },
  ].flatMap((frame) => {
    const image = references.find(
      (reference) => reference.material_id === frame.materialId,
    );
    return image?.url
      ? [{ label: frame.label, url: api.mediaUrl(image.url) }]
      : [];
  });
});
const directVideo = computed(() => snapshot.value?.path === "reference_video");
const statusLabels: Record<string, string> = {
  queued: "排队中",
  running: "生成中",
  succeeded: "已完成",
  failed: "失败",
  cancelled: "已取消",
};
const kindLabels: Record<string, string> = {
  image: "分镜图",
  video: "视频",
  speech: "配音",
};
async function refresh() {
  if (inFlight) return;
  inFlight = true;
  try {
    const r = await bindingsApi.getRun(projectId, draftId, sceneId, runId);
    if (alive) {
      run.value = r;
      await reload();
    }
  } catch (e) {
    if (alive) error.value = (e as Error).message;
  } finally {
    inFlight = false;
  }
}
async function select(input: {
  take_id?: string;
  utterance_id?: string;
  video_version_id?: string;
}) {
  if (busy.value) return;
  busy.value = true;
  message.value = "";
  try {
    const latest = await api.draft(projectId, draftId);
    if (run.value?.selection) {
      run.value = await bindingsApi.selectRun(
        projectId,
        draftId,
        sceneId,
        runId,
        { expected_version: latest.version || 0, ...input },
      );
    } else if (input.video_version_id) {
      await api.selectSceneVersion(
        projectId,
        draftId,
        sceneId,
        latest.version || 0,
        input.video_version_id,
      );
      await refresh();
    } else if (input.take_id) {
      await bindingsApi.select(projectId, draftId, sceneId, {
        expected_version: latest.version || 0,
        take_id: input.take_id,
        utterance_id: input.utterance_id,
      });
      await refresh();
    }
    await reload();
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
function isSelected(t: RunStep) {
  const take = takes.value.find((x) => x.task_id === t.id);
  if (!run.value?.selection)
    return t.kind === "video"
      ? !!versions.value.find(
          (v) =>
            v.task_id === t.id &&
            draft.value?.selected_versions?.[sceneId] === v.id,
        )
      : !!take &&
          draft.value?.selected_media?.[
            t.kind === "image"
              ? `${sceneId}:image`
              : `${sceneId}:speech:${take.utterance_id}`
          ] === take.id;
  return t.kind === "image"
    ? !!take && run.value?.selection?.image_take_id === take.id
    : t.kind === "speech"
      ? !!take &&
        run.value?.selection?.speech_take_ids?.[take.utterance_id || ""] ===
          take.id
      : !!versions.value.find(
          (v) =>
            v.task_id === t.id &&
            v.id === run.value?.selection?.video_version_id,
        );
}
async function choose(t: RunStep) {
  if (t.kind === "video") {
    const v = versions.value.find((v) => v.task_id === t.id);
    if (v) await select({ video_version_id: v.id });
  } else {
    const take = takes.value.find((x) => x.task_id === t.id);
    if (take)
      await select({ take_id: take.id, utterance_id: take.utterance_id });
  }
}
async function retry(id: string) {
  busy.value = true;
  try {
    await api.retryTask(id);
    await refresh();
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
async function cancel() {
  busy.value = true;
  try {
    run.value = await bindingsApi.cancelRun(projectId, draftId, sceneId, runId);
    await refresh();
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
async function download(t: RunStep) {
  try {
    await api.downloadTask(
      t.id,
      t.kind === "video" ? "video" : t.kind === "speech" ? "speech" : "image",
    );
  } catch (e) {
    message.value = (e as Error).message;
  }
}
onMounted(() => {
  void refresh();
  timer = setInterval(() => {
    if (!["completed", "cancelled"].includes(run.value?.state || ""))
      void refresh();
  }, 4000);
});
onBeforeUnmount(() => {
  alive = false;
  clearInterval(timer);
});
</script>
<template>
  <main class="shot-workspace">
    <nav class="shot-breadcrumb">
      <RouterLink :to="`${base}/shots`">分镜列表</RouterLink><span>›</span
      ><RouterLink :to="`${base}/shots/${encodeURIComponent(sceneId)}`"
        >分镜详情</RouterLink
      ><span>› 任务详情</span>
    </nav>
    <p v-if="error || message" role="alert">{{ error || message }}</p>
    <template v-if="run"
      ><header class="shot-page-heading">
        <div>
          <h1>制作任务详情</h1>
          <small
            >{{ run.id }} ·
            {{
              run.created_at
                ? new Date(run.created_at).toLocaleString()
                : "历史记录"
            }}</small
          >
        </div>
        <div class="run-header-actions">
          <RouterLink
            class="button"
            :to="`${base}/shots/${encodeURIComponent(sceneId)}`"
            >返回分镜</RouterLink
          ><RouterLink class="button" :to="`${base}?tab=video`"
            >前往成片</RouterLink
          ><button
            v-if="
              ![
                'completed',
                'cancelled',
                'awaiting_composition',
              ].includes(run.state)
            "
            :disabled="busy"
            @click="cancel"
          >
            取消制作任务
          </button>
          <button
            v-if="run.state === 'failed'"
            :disabled="busy"
            @click="remove"
          >
            {{ busy ? "处理中…" : "删除任务" }}
          </button>
        </div>
      </header>
      <section v-if="!directVideo" class="shot-task-structure">
        <h2>{{ directVideo ? "视频状态" : "制作流程" }}</h2>
        <GenerationRunStages :run="run" />
        <p v-if="run.error" role="alert">{{ run.error }}</p>
        <p v-if="run.state === 'awaiting_selection'">
          请选用本任务的分镜图及每句配音，随后继续生成视频。
        </p>
        <p v-if="run.state === 'awaiting_video_selection'">
          视频已生成，请预览后选用。
        </p>
        <p v-if="run.state === 'awaiting_composition'">
          视频已选用，整片合成请前往成片页面。
        </p>
      </section>
      <div class="run-detail-layout">
        <section class="shot-task-structure" aria-label="生成记录">
          <h2>{{ directVideo ? "视频内容" : "生成记录与视频内容" }}</h2>
          <template v-if="directVideo"
            ><GenerationRunStages :run="run" />
            <p v-if="run.error" role="alert">{{ run.error }}</p>
            <p v-if="run.state === 'awaiting_composition'">
              视频已选用，可前往成片页面合成。
            </p></template
          >
          <p v-if="!steps.length">任务准备中…</p>
          <article v-for="t in steps" :key="t.id" class="run-step">
            <header>
              <h3>{{ kindLabels[t.kind] || t.kind }}</h3>
              <span
                >{{ statusLabels[t.status] || t.status }}
                {{ t.progress || 0 }}%</span
              >
            </header>
            <div
              v-if="
                t.kind === 'video' &&
                (t.status === 'queued' || t.status === 'running')
              "
              class="run-video-progress"
            >
              <strong>{{
                t.status === "queued" ? "视频等待生成" : "正在生成视频"
              }}</strong
              ><progress
                :value="t.progress || 0"
                max="100"
                aria-label="视频生成进度"
              /><span>完成后在这里预览和下载</span>
            </div>
            <p v-if="t.error" role="alert">{{ t.error }}</p>
            <template v-if="t.status === 'succeeded' && t.result_url"
              ><ShotPreview
                v-if="t.kind === 'video' || t.kind === 'image'"
                :title="kindLabels[t.kind]"
                hide-title
                :url="
                  t.kind === 'video' ? api.mediaUrl(t.result_url) : undefined
                "
                :image="
                  t.kind === 'image' ? api.mediaUrl(t.result_url) : undefined
                "
              /><audio
                v-if="t.kind === 'speech'"
                :src="api.mediaUrl(t.result_url)"
                controls
                preload="none"
              />
              <div class="shot-list-actions">
                <button
                  class="primary"
                  :disabled="busy || isSelected(t) || run.state === 'cancelled'"
                  @click="choose(t)"
                >
                  {{ isSelected(t) ? "已选用" : "选用此结果" }}</button
                ><button @click="download(t)">
                  下载{{ kindLabels[t.kind] }}
                </button>
              </div></template
            ><button
              v-if="t.status === 'failed' || t.status === 'cancelled'"
              :disabled="busy || run.state === 'cancelled'"
              @click="retry(t.id)"
            >
              按原输入重试
            </button>
          </article>
        </section>
        <section
          class="shot-task-structure run-settings"
          aria-label="本次生成设定"
        >
          <h2>本次生成设定</h2>
          <p>
            <span v-if="!directVideo"
              >分镜图模型：{{ snapshot?.model_id }} · </span
            >视频模型：{{ run.video_model_id }}
          </p>
          <p>
            画幅：{{ snapshot?.aspect_ratio }} · {{ run.duration_seconds }} 秒 ·
            {{ run.resolution }}
          </p>
          <p v-if="snapshot?.video_input_mode">
            生成模式：{{
              snapshot.video_input_mode === "reference_images"
                ? "智能参考"
                : snapshot.last_frame_material_id
                  ? "首尾帧"
                  : "首帧"
            }}
          </p>
          <div v-if="snapshotFrames.length" class="snapshot-frames">
            <figure v-for="frame in snapshotFrames" :key="frame.label">
              <figcaption>{{ frame.label }}</figcaption>
              <img :alt="'提交时的' + frame.label" :src="frame.url" />
            </figure>
          </div>
          <details>
            <summary>提交时的场景描述与引用素材</summary>
            <p class="snapshot-prompt">{{ snapshot?.prompt }}</p>
            <article v-for="e in snapshot?.entities" :key="e.binding_id">
              <strong>{{ e.entity_id }} · v{{ e.revision }}</strong>
              <p>{{ e.description }}</p>
              <span>{{ e.references.length }} 张参考图</span>
            </article>
          </details>
        </section>
      </div></template
    >
  </main>
</template>

<style scoped>
.run-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.run-detail-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 18px;
  align-items: start;
}
.run-detail-layout > section {
  min-width: 0;
  margin-top: 0;
}
.run-settings {
  overflow-wrap: anywhere;
}
.shot-page-heading small {
  overflow-wrap: anywhere;
}
@media (max-width: 1100px) {
  .run-detail-layout {
    grid-template-columns: minmax(0, 1fr);
  }
}

.snapshot-frames {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
.snapshot-frames figure {
  margin: 0;
}
.snapshot-frames img {
  width: 160px;
  height: 100px;
  object-fit: contain;
  background: #f3f6f8;
  border-radius: 6px;
}
</style>
