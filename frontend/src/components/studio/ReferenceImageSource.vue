<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import ImageGenerationReferences from "./ImageGenerationReferences.vue";
import { api } from "../../api/client";
import { bindingsApi, type EntityVersion } from "../../api/bindings";
import type { Material, Model, Task } from "../../api/types";
import type { components } from "../../api/schema";
type Request = components["schemas"]["ReferenceImageRequest"];
const props = defineProps<{
  projectId: string;
  draftId: string;
  entity: EntityVersion;
  materials: Material[];
  models: Model[];
  disabled?: boolean;
}>();
const emit = defineEmits<{
  add: [Material];
  material: [Material];
  busy: [boolean];
}>();
const mode = ref<"library" | "generate">("library"),
  query = ref(""),
  library = ref<Material[]>([]),
  libraryError = ref(""),
  loading = ref(false);
const uploads = ref<{ file: File; preview: string; uploaded?: Material }[]>([]),
  uploadError = ref(""),
  uploadProgress = ref(0);
const uploadAction = ref<"use" | "library">("use");
const uploadsInLibrary = computed(
  () =>
    uploads.value.length > 0 &&
    uploads.value.every(
      (item) => item.uploaded && !item.uploaded.reference_only,
    ),
);
defineExpose({ hasPendingUpload: () => uploads.value.length > 0 });
const generationMode = ref<NonNullable<Request["mode"]>>("text_to_image");
const editReferences = ref<Material[]>([]);
const imageModeAvailable = (m: Model, requested: string) =>
  m.availability?.image?.available === true &&
  !!m.availability.image.binding.image_modes?.includes(
    requested as "text_to_image" | "image_to_image",
  );
const canEdit = computed(() =>
  props.models.some((m) => imageModeAvailable(m, "image_to_image")),
);
const canText = computed(() =>
  props.models.some((m) => imageModeAvailable(m, "text_to_image")),
);
if (!canText.value && canEdit.value) generationMode.value = "image_to_image";
const models = computed(() =>
  props.models.filter((m) => imageModeAvailable(m, generationMode.value)),
);
const imageModelOptions = computed(() =>
  props.models.filter((m) => m.capabilities.includes("image")),
);
function modelReason(m: Model) {
  if (!m.availability?.image?.available)
    return m.availability?.image?.reason || "配置未就绪";
  return generationMode.value === "image_to_image"
    ? "未启用图生图"
    : "未启用文生图";
}
const model = ref(
  models.value.find((m) => m.default)?.id || models.value[0]?.id || "",
);
const prompt = ref(""),
  aspect = ref<Request["aspect_ratio"]>(
    props.entity.kind === "character" ? "1:1" : "16:9",
  );
const imageBinding = computed(
  () =>
    models.value.find((m) => m.id === model.value)?.availability?.image
      ?.binding,
);
const imageRatios = computed(() => imageBinding.value?.ratios || []);
watch(generationMode, () => {
  if (!models.value.some((m) => m.id === model.value)) model.value = "";
});
watch(
  [model, imageRatios],
  () => {
    if (!imageRatios.value.includes(aspect.value))
      aspect.value = (imageRatios.value[0] || "") as Request["aspect_ratio"];
  },
  { immediate: true },
);
const busy = ref(false),
  error = ref(""),
  current = ref<Task>(),
  result = ref<Material>(),
  pending = ref<Request>();
const active = computed(
  () => !!current.value && ["queued", "running"].includes(current.value.status),
);
const locked = computed(
  () => !!props.disabled || busy.value || active.value || !!pending.value,
);
const kind = computed(() =>
  props.entity.kind === "character" ? "character" : "scene",
);
const chosen = (id: string) =>
  props.entity.references.some((r) => r.material_id === id);
const available = computed(() => {
  const map = new Map(
    [...props.materials, ...library.value].map((m) => [m.id, m]),
  );
  return [...map.values()].filter(
    (m) =>
      !m.reference_only &&
      m.kind === kind.value &&
      (!query.value ||
        m.name.toLowerCase().includes(query.value.toLowerCase())),
  );
});
const status: Record<string, string> = {
  queued: "排队中",
  running: "生成中",
  succeeded: "生成完成",
  failed: "生成失败",
  cancelled: "已取消",
};
const cacheKey = `frameflow-reference-image:${props.projectId}:${props.draftId}:${props.entity.id}`;
let alive = true,
  timer: ReturnType<typeof setTimeout> | undefined,
  epoch = 0;
function remember() {
  try {
    localStorage.setItem(
      cacheKey,
      JSON.stringify({ pending: pending.value, task: current.value?.id }),
    );
  } catch {
    /* Task remains available in the task center. */
  }
}
function setBusy(value: boolean) {
  busy.value = value;
  emit("busy", value);
}
async function showLibrary() {
  mode.value = "library";
  await refreshLibrary();
}
async function refreshLibrary() {
  loading.value = true;
  libraryError.value = "";
  try {
    const data = await api.materials(kind.value);
    if (alive) library.value = data.materials || [];
  } catch (e) {
    if (alive) libraryError.value = (e as Error).message;
  } finally {
    if (alive) loading.value = false;
  }
}
function add(material: Material) {
  if (!props.disabled && !chosen(material.id)) {
    emit("material", material);
    emit("add", material);
  }
}
function clearUpload() {
  for (const item of uploads.value) URL.revokeObjectURL(item.preview);
  uploads.value = [];
  uploadError.value = "";
}
function chooseUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  input.value = "";
  uploadError.value = "";
  for (const file of files) {
    if (
      !["image/png", "image/jpeg", "image/gif"].includes(file.type) ||
      file.size > 10 * 1024 * 1024
    ) {
      uploadError.value = "请选择不超过 10MB 的 PNG、JPEG 或 GIF 图片。";
      continue;
    }
    uploads.value.push({ file, preview: URL.createObjectURL(file) });
  }
}
async function useUpload(publish = false) {
  if (!uploads.value.length || busy.value || props.disabled) return;
  uploadAction.value = publish ? "library" : "use";
  setBusy(true);
  uploadError.value = "";
  for (const item of [...uploads.value]) {
    uploadProgress.value = 0;
    try {
      let material =
        item.uploaded ||
        (await api.uploadMaterial(
          item.file,
          kind.value,
          (n) => (uploadProgress.value = n),
          { projectId: props.projectId, referenceOnly: true },
        ));
      if (!alive) return;
      item.uploaded = material;
      if (publish && material.reference_only)
        material = await api.addMaterialToLibrary(material.id);
      if (!alive) return;
      item.uploaded = material;
      if (publish) {
        emit("material", material);
      } else {
        add(material);
        URL.revokeObjectURL(item.preview);
        uploads.value = uploads.value.filter((u) => u !== item);
      }
      if (!material.reference_only) {
        library.value = [
          ...library.value.filter((m) => m.id !== material.id),
          material,
        ];
      }
    } catch (e) {
      if (alive) uploadError.value = (e as Error).message;
    }
  }
  if (alive) {
    if (!uploads.value.length) clearUpload();
    setBusy(false);
  }
}
function useGenerated() {
  if (!result.value || busy.value || props.disabled) return;
  add(result.value);
}
async function publishGenerated() {
  if (!result.value?.reference_only || busy.value || props.disabled) return;
  setBusy(true);
  error.value = "";
  try {
    const material = await api.addMaterialToLibrary(result.value.id);
    if (!alive) return;
    result.value = material;
    emit("material", material);
    library.value = [
      ...library.value.filter((m) => m.id !== material.id),
      material,
    ];
  } catch (e) {
    if (alive) error.value = (e as Error).message;
  } finally {
    if (alive) setBusy(false);
  }
}
async function refresh() {
  if (!current.value || !alive) return;
  clearTimeout(timer);
  const at = ++epoch;
  const id = current.value.id;
  try {
    const task = await bindingsApi.task(id);
    if (!alive || at !== epoch) return;
    current.value = task;
    error.value = "";
    if (task.status === "succeeded") {
      const material = await api.material(`result-${id}`);
      if (!alive || at !== epoch) return;
      result.value = material;
      emit("material", material);
    }
  } catch (e) {
    if (alive && at === epoch) error.value = (e as Error).message;
  } finally {
    if (alive && at === epoch && active.value)
      timer = setTimeout(() => void refresh(), 2000);
  }
}
async function generate() {
  if (busy.value || active.value || props.disabled) return;
  error.value = "";
  if (!pending.value) {
    const text = prompt.value.trim() || props.entity.description.trim();
    if (
      !props.entity.name.trim() ||
      !text ||
      !models.value.some((m) => m.id === model.value)
    ) {
      error.value = "请填写名称、提示词并选择可用的生图模型。";
      return;
    }
    if (generationMode.value === "image_to_image") {
      const minimum = imageBinding.value?.min_references || 1;
      const maximum = imageBinding.value?.max_references || 0;
      if (
        editReferences.value.length < minimum ||
        editReferences.value.length > maximum
      ) {
        error.value = `请选择 ${minimum}–${maximum} 张参考图片。`;
        return;
      }
    }
    pending.value = {
      mode: generationMode.value,
      reference_material_ids:
        generationMode.value === "image_to_image"
          ? editReferences.value.map((m) => m.id)
          : [],
      idempotency_key: crypto.randomUUID(),
      kind: kind.value,
      name: props.entity.name.trim(),
      prompt: text,
      model_id: model.value,
      aspect_ratio: aspect.value,
    };
    remember();
  }
  setBusy(true);
  try {
    const task = await bindingsApi.referenceImage(
      props.projectId,
      props.draftId,
      pending.value,
    );
    if (!alive) return;
    current.value = task;
    result.value = undefined;
    pending.value = undefined;
    remember();
    void refresh();
  } catch (e) {
    if (!alive) return;
    error.value = (e as Error).message;
    const code = (e as { status?: number }).status;
    if (code && code >= 400 && code < 500 && code !== 429) {
      pending.value = undefined;
      remember();
    }
  } finally {
    if (alive) setBusy(false);
  }
}
async function taskAction(action: "retry" | "cancel") {
  if (!current.value || busy.value || props.disabled) return;
  setBusy(true);
  error.value = "";
  clearTimeout(timer);
  epoch++;
  try {
    current.value = await (action === "retry"
      ? api.retryTask(current.value.id)
      : api.cancelTask(current.value.id));
    if (alive) {
      remember();
      void refresh();
    }
  } catch (e) {
    if (alive) {
      error.value = (e as Error).message;
      void refresh();
    }
  } finally {
    if (alive) setBusy(false);
  }
}
async function restoreReferences(ids: string[]) {
  const items = await Promise.all(
    ids.map(async (id) => {
      const known = props.materials.find((m) => m.id === id);
      if (known) return known;
      try {
        return await api.material(id);
      } catch {
        const frozen = current.value?.input?.reference_images?.find(
          (r) => r.material_id === id,
        );
        return {
          id,
          name: frozen?.label || "参考图片暂时无法读取",
          kind: kind.value,
          url: frozen?.url || "",
          reference_only: true,
          project_id: props.projectId,
        } as Material;
      }
    }),
  );
  if (alive) editReferences.value = items;
}
onMounted(async () => {
  try {
    const cached = JSON.parse(localStorage.getItem(cacheKey) || "null") as {
      pending?: Request;
      task?: string;
    } | null;
    if (cached?.pending) {
      pending.value = cached.pending;
      generationMode.value = cached.pending.mode || "text_to_image";
      await restoreReferences(cached.pending.reference_material_ids || []);
      model.value = cached.pending.model_id;
      prompt.value = cached.pending.prompt;
      aspect.value = cached.pending.aspect_ratio;
      mode.value = "generate";
    }
    if (cached?.task) {
      const task = await bindingsApi.task(cached.task);
      if (!alive) return;
      current.value = task;
      generationMode.value =
        task.input?.mode === "image_to_image"
          ? "image_to_image"
          : "text_to_image";
      await restoreReferences(
        task.input?.reference_images?.map((r) => r.material_id) || [],
      );
      model.value = task.model_id || model.value;
      prompt.value = task.input?.prompt || "";
      if (["16:9", "9:16", "1:1"].includes(task.input?.aspect_ratio || ""))
        aspect.value = task.input.aspect_ratio as Request["aspect_ratio"];
      mode.value = "generate";
      void refresh();
    }
  } catch {
    /* The user can recover saved tasks in the task center. */
  }
});
onMounted(() => void refreshLibrary());
onBeforeUnmount(() => {
  alive = false;
  epoch++;
  clearTimeout(timer);
  for (const item of uploads.value) URL.revokeObjectURL(item.preview);
  emit("busy", false);
});
</script>

<template>
  <section class="reference-source" aria-label="参考图来源">
    <div class="reference-source-actions">
      <button
        type="button"
        :disabled="disabled || busy"
        :aria-pressed="mode === 'library'"
        @click="showLibrary"
      >
        从素材库选择</button
      ><button
        type="button"
        :disabled="disabled || busy"
        :aria-pressed="mode === 'generate'"
        @click="mode = 'generate'"
      >
        图片生成
      </button>
    </div>
    <section
      v-show="mode === 'library'"
      class="reference-source-panel"
      aria-label="素材库图片选择"
    >
      <div class="reference-library-toolbar">
        <label
          >搜索参考图<input
            v-model="query"
            type="search"
            placeholder="搜索名称" /></label
        ><label class="reference-local-upload"
          >上传图片<input
            type="file"
            multiple
            accept="image/png,image/jpeg,image/gif"
            aria-label="上传参考图片"
            :disabled="disabled || busy"
            @change="chooseUpload"
        /></label>
      </div>
      <p v-if="loading">加载中…</p>
      <p v-if="libraryError" role="alert">{{ libraryError }}</p>
      <button v-if="libraryError" type="button" @click="refreshLibrary">
        重新加载素材
      </button>
      <p
        v-if="!loading && !libraryError && !available.length"
        class="reference-empty"
      >
        暂无可用{{
          kind === "scene" ? "场景" : "人物"
        }}图片，可以上传图片或切换到图片生成。
      </p>
      <div class="reference-library-grid">
        <button
          v-for="item in available"
          :key="item.id"
          type="button"
          :disabled="disabled || busy || chosen(item.id)"
          :aria-label="`选择参考图 ${item.name}`"
          :class="{ selected: chosen(item.id) }"
          @click="add(item)"
        >
          <img :src="api.mediaUrl(item.url)" :alt="item.name" /><span>{{
            item.name
          }}</span
          ><small>{{ chosen(item.id) ? "✓ 已选" : "选择使用" }}</small>
        </button>
      </div>
      <div v-if="uploads.length" class="reference-pending-upload">
        <header><strong>本次上传</strong><span>待确认</span></header>
        <div
          v-for="item in uploads"
          :key="item.preview"
          class="reference-upload-preview"
        >
          <img :src="item.preview" :alt="item.file.name" /><span>{{
            item.file.name
          }}</span>
        </div>
        <button
          type="button"
          :disabled="disabled || busy || uploadsInLibrary"
          @click="useUpload(true)"
        >
          {{
            busy && uploadAction === "library"
              ? `上传中 ${uploadProgress}%`
              : uploadsInLibrary
                ? "已上传到素材库"
                : "上传到素材库"
          }}
        </button>
        <p class="reference-option-note">
          {{
            uploadsInLibrary
              ? "已保存，可在素材库中复用。"
              : "直接使用仅作为当前人物或场景的参考图；需要复用时，再上传到素材库。"
          }}
        </p>
        <div class="reference-use-actions">
          <button
            type="button"
            :disabled="disabled || busy"
            @click="clearUpload"
          >
            取消上传</button
          ><button
            type="button"
            class="primary"
            :disabled="disabled || busy"
            @click="useUpload()"
          >
            {{
              busy && uploadAction === "use"
                ? `处理中 ${uploadProgress}%`
                : uploadError && uploadAction === "use"
                  ? "重试失败图片"
                  : uploads.length > 1
                    ? `使用这 ${uploads.length} 张图片`
                    : "使用这张图片"
            }}
          </button>
        </div>
      </div>
      <p v-if="uploadError" role="alert">{{ uploadError }}</p>
    </section>
    <section
      v-show="mode === 'generate'"
      class="reference-source-panel"
      aria-label="图片生成设置"
    >
      <h4>生成{{ kind === "scene" ? "场景" : "人物" }}参考图</h4>
      <div
        class="image-generation-modes"
        role="group"
        aria-label="图片生成模式"
      >
        <button
          type="button"
          :aria-pressed="generationMode === 'text_to_image'"
          :disabled="locked || !canText"
          @click="generationMode = 'text_to_image'"
        >
          文生图
        </button>
        <button
          type="button"
          :aria-pressed="generationMode === 'image_to_image'"
          :disabled="locked || !canEdit"
          @click="generationMode = 'image_to_image'"
        >
          图生图
        </button>
      </div>
      <p v-if="!canEdit" class="muted">
        图生图尚未配置：请在<RouterLink to="/config"
          >模型管理 → 协议与参数 → 图片生成</RouterLink
        >中启用图生图并填写参考图片限制。
      </p>
      <p v-if="!models.length" class="muted">
        暂无配置就绪的生图模型，<RouterLink to="/config"
          >前往模型管理配置</RouterLink
        >。
      </p>
      <div class="reference-image-settings">
        <label
          >生图模型<select v-model="model" :disabled="locked">
            <option value="">请选择生图模型</option>
            <option
              v-for="m in imageModelOptions"
              :key="m.id"
              :value="m.id"
              :disabled="!imageModeAvailable(m, generationMode)"
            >
              {{ m.name
              }}{{
                imageModeAvailable(m, generationMode)
                  ? ""
                  : `（${modelReason(m)}）`
              }}
            </option>
          </select></label
        ><label
          >图片画幅<select v-model="aspect" :disabled="locked">
            <option v-for="ratio in imageRatios" :key="ratio" :value="ratio">
              {{ ratio }}
            </option>
          </select></label
        >
      </div>
      <ImageGenerationReferences
        v-if="generationMode === 'image_to_image'"
        v-model="editReferences"
        :project-id="projectId"
        :kind="kind"
        :materials="materials"
        :binding="imageBinding"
        :disabled="locked || !imageBinding"
        @busy="setBusy"
        @material="emit('material', $event)"
      />
      <label
        >生图提示词<textarea
          v-model="prompt"
          :disabled="locked"
          rows="4"
          maxlength="4000"
          :placeholder="
            generationMode === 'image_to_image'
              ? '例如：保留图 1 的人物，将服装改为图 2 的配色；请写明保留与修改的内容'
              : '描述环境、外观、服装或画风；留空时使用参考描述'
          "
        /></label
      ><button
        type="button"
        :disabled="locked || !entity.description.trim()"
        @click="prompt = entity.description"
      >
        使用参考描述
      </button>
      <p class="muted">
        默认仅作项目参考图。关闭侧栏后任务继续，可在任务中心查看。
      </p>
      <p v-if="error" role="alert">{{ error }}</p>
      <button
        type="button"
        class="primary"
        :disabled="disabled || busy || active || (!pending && !models.length)"
        @click="generate"
      >
        {{
          busy
            ? "处理中…"
            : pending
              ? "恢复提交"
              : active
                ? "正在生成…"
                : "生成图片"
        }}
      </button>
      <div v-if="current" class="reference-generation-result">
        <p role="status">
          {{ status[current.status] || current.status
          }}<span v-if="active"> · {{ current.progress }}%</span>
        </p>
        <p v-if="current.status === 'failed'" role="alert">
          {{ current.error || "生成失败，请重试。" }}
        </p>
        <button
          v-if="active"
          type="button"
          :disabled="busy || disabled"
          @click="taskAction('cancel')"
        >
          取消生成</button
        ><button
          v-if="current.status === 'failed' || current.status === 'cancelled'"
          type="button"
          :disabled="busy || disabled"
          @click="taskAction('retry')"
        >
          按原设置重试</button
        ><button
          v-if="error && !pending"
          type="button"
          :disabled="busy"
          @click="refresh"
        >
          刷新生成状态
        </button>
        <template v-if="result">
          <details
            v-if="current?.input?.reference_images?.length"
            class="original-image-comparison"
          >
            <summary>查看生成时使用的原图（原图保留）</summary>
            <div>
              <figure
                v-for="(reference, index) in current.input.reference_images"
                :key="reference.material_id"
              >
                <img
                  :src="api.mediaUrl(reference.url || '')"
                  :alt="`原图 ${index + 1}`"
                />
                <figcaption>图 {{ index + 1 }}</figcaption>
              </figure>
            </div>
          </details>
          <strong>生成结果</strong>
          <img :src="api.mediaUrl(result.url)" :alt="result.name" />
          <button
            type="button"
            :disabled="disabled || busy || !result.reference_only"
            @click="publishGenerated"
          >
            {{
              !result.reference_only
                ? "已上传到素材库"
                : busy
                  ? "上传中…"
                  : "上传到素材库"
            }}
          </button>
          <p class="reference-option-note">
            {{
              result.reference_only
                ? "直接使用仅作为当前人物或场景的参考图；需要复用时，再上传到素材库。"
                : "已保存，可在素材库中复用。"
            }}
          </p>
          <button
            type="button"
            class="primary reference-use-generated"
            :disabled="disabled || busy || chosen(result.id)"
            @click="useGenerated"
          >
            {{ chosen(result.id) ? "已加入参考图" : "使用这张图片" }}
          </button></template
        >
      </div>
    </section>
  </section>
</template>

<style scoped>
.image-generation-modes {
  display: flex;
  gap: 8px;
}
.image-generation-modes button[aria-pressed="true"] {
  color: #2357b9;
  background: #edf4ff;
  border-color: #7fa3e2;
}
.original-image-comparison > div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 8px;
}
.original-image-comparison figure {
  width: 90px;
  margin: 0;
}
.original-image-comparison img {
  width: 90px;
  height: 70px;
  object-fit: cover;
  border-radius: 6px;
}
.original-image-comparison figcaption {
  font-size: 12px;
}
</style>
