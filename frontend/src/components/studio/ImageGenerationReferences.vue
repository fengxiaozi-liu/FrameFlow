<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from "vue";
import { api } from "../../api/client";
import type { Material, OperationBinding } from "../../api/types";
const selected = defineModel<Material[]>({ required: true });
const props = defineProps<{
  projectId: string;
  kind: "scene" | "character";
  materials: Material[];
  binding?: OperationBinding;
  disabled?: boolean;
}>();
const emit = defineEmits<{ busy: [boolean]; material: [Material] }>();
const library = ref<Material[]>([]),
  query = ref(""),
  showLibrary = ref(false);
const error = ref(""),
  loading = ref(false),
  uploading = ref(false),
  progress = ref(0);
let alive = true;
const maximum = computed(() => props.binding?.max_references || 0);
const maxBytes = computed(() => props.binding?.max_image_bytes || 0);
const limitReached = computed(() => selected.value.length >= maximum.value);
const locked = computed(() => props.disabled || uploading.value);
function imageAllowed(m: Material) {
  return (
    ["scene", "character", "prop"].includes(m.kind) &&
    (!m.reference_only || m.project_id === props.projectId) &&
    /\.(png|jpe?g)$/i.test(m.url)
  );
}
const available = computed(() =>
  [
    ...new Map(
      [...props.materials, ...library.value].map((m) => [m.id, m]),
    ).values(),
  ].filter(
    (m) =>
      imageAllowed(m) &&
      (!query.value ||
        m.name.toLowerCase().includes(query.value.toLowerCase())),
  ),
);
async function refreshLibrary() {
  loading.value = true;
  error.value = "";
  try {
    const data = await api.materials("", "", "image", props.projectId);
    if (alive) library.value = data.materials || [];
  } catch (e) {
    if (alive) error.value = (e as Error).message;
  } finally {
    if (alive) loading.value = false;
  }
}
function select(m: Material) {
  if (
    locked.value ||
    limitReached.value ||
    selected.value.some((item) => item.id === m.id)
  )
    return;
  if ((m.media?.size_bytes || 0) > maxBytes.value) {
    error.value = `图片超过模型单张上限 ${maxBytes.value / 1048576}MB。`;
    return;
  }
  selected.value = [...selected.value, m];
  error.value = "";
}
function move(index: number, delta: number) {
  const next = [...selected.value];
  const target = index + delta;
  if (locked.value || !next[index] || !next[target]) return;
  [next[index], next[target]] = [next[target]!, next[index]!];
  selected.value = next;
}
async function upload(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  input.value = "";
  if (locked.value || !files.length) return;
  error.value = "";
  if (files.length + selected.value.length > maximum.value) {
    error.value = `最多选择 ${maximum.value} 张参考图片。`;
    return;
  }
  if (
    files.some(
      (file) =>
        !["image/png", "image/jpeg"].includes(file.type) ||
        file.size > maxBytes.value,
    )
  ) {
    error.value = `请选择不超过 ${maxBytes.value / 1048576}MB 的 PNG 或 JPEG 图片。`;
    return;
  }
  uploading.value = true;
  emit("busy", true);
  try {
    for (const file of files) {
      progress.value = 0;
      const m = await api.uploadMaterial(
        file,
        props.kind,
        (n) => (progress.value = n),
        { projectId: props.projectId, referenceOnly: true },
      );
      if (!alive) return;
      selected.value = [...selected.value, m];
      emit("material", m);
    }
  } catch (e) {
    if (alive) error.value = (e as Error).message;
  } finally {
    uploading.value = false;
    emit("busy", false);
  }
}
onMounted(() => void refreshLibrary());
onBeforeUnmount(() => {
  alive = false;
});
</script>

<template>
  <section class="image-edit-references" aria-label="图生图参考图片">
    <header>
      <strong>参考图片</strong
      ><span>{{ selected.length }} / {{ maximum }} 张</span>
    </header>
    <p class="muted">
      需要 {{ binding?.min_references || 1 }}–{{ maximum }} 张 · 单张 ≤
      {{ maxBytes / 1048576 }}MB · 按图 1、图 2 的顺序描述修改要求。
    </p>
    <ol class="image-input-list">
      <li v-for="(m, index) in selected" :key="m.id">
        <img :src="api.mediaUrl(m.url)" :alt="m.name" />
        <div class="image-input-name">
          <strong>图 {{ index + 1 }}</strong
          ><span>{{ m.name }}</span>
        </div>
        <div class="image-input-actions">
          <button
            type="button"
            :disabled="locked || index === 0"
            :aria-label="`前移图 ${index + 1}`"
            @click="move(index, -1)"
          >
            ↑
          </button>
          <button
            type="button"
            :disabled="locked || index === selected.length - 1"
            :aria-label="`后移图 ${index + 1}`"
            @click="move(index, 1)"
          >
            ↓
          </button>
          <button
            type="button"
            :disabled="locked"
            :aria-label="`移除图 ${index + 1}`"
            @click="selected = selected.filter((item) => item.id !== m.id)"
          >
            移除
          </button>
        </div>
      </li>
    </ol>
    <p v-if="!selected.length" class="muted">
      选择要修改的原图，也可以加入风格或人物参考图片。
    </p>
    <div class="image-input-sources">
      <button
        type="button"
        :disabled="locked"
        :aria-expanded="showLibrary"
        @click="showLibrary = !showLibrary"
      >
        从素材库选择参考图片
      </button>
      <label
        >本地上传<input
          type="file"
          multiple
          accept="image/png,image/jpeg"
          aria-label="上传图生图参考图片"
          :disabled="locked || limitReached || !maximum"
          @change="upload"
      /></label>
    </div>
    <p class="muted">
      本地上传仅用于当前项目；原图保留，生成结果可另行上传到素材库。
    </p>
    <p v-if="uploading" role="status">上传中 {{ progress }}%</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <div v-if="showLibrary" class="image-input-library">
      <label
        >搜索图片<input v-model="query" type="search" placeholder="图片名称"
      /></label>
      <p v-if="loading">加载中…</p>
      <button
        v-if="error"
        type="button"
        :disabled="loading || locked"
        @click="refreshLibrary"
      >
        重新加载图片
      </button>
      <p v-if="!loading && !available.length">暂无图片，可以本地上传。</p>
      <div class="image-input-grid">
        <button
          v-for="m in available"
          :key="m.id"
          type="button"
          :disabled="
            locked || limitReached || selected.some((item) => item.id === m.id)
          "
          :aria-label="`添加编辑参考图 ${m.name}`"
          @click="select(m)"
        >
          <img :src="api.mediaUrl(m.url)" :alt="m.name" /><span>{{
            m.name
          }}</span>
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.image-edit-references {
  border: 1px solid var(--border, #dbe3ee);
  border-radius: 8px;
  padding: 12px;
  display: grid;
  gap: 10px;
  min-width: 0;
}
header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
p {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
}
.image-input-list {
  display: grid;
  gap: 8px;
  padding: 0;
  margin: 0;
  list-style: none;
}
.image-input-list li {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.image-input-list img {
  width: 48px;
  height: 48px;
  object-fit: cover;
  border-radius: 6px;
}
.image-input-name {
  display: grid;
  gap: 2px;
  min-width: 0;
  flex: 1;
  font-size: 12px;
}
.image-input-name span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.image-input-actions {
  display: flex;
  gap: 4px;
}
.image-input-actions button {
  padding: 4px 7px;
  min-height: 30px;
}
.image-input-sources {
  display: flex;
  align-items: end;
  gap: 10px;
  flex-wrap: wrap;
}
.image-input-sources label {
  display: grid;
  gap: 6px;
  min-width: 0;
}
input[type="file"] {
  max-width: 100%;
  width: 230px;
}
.image-input-library {
  display: grid;
  gap: 10px;
}
.image-input-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));
  gap: 8px;
  max-height: 240px;
  overflow-y: auto;
}
.image-input-grid button {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 6px;
  text-align: left;
}
.image-input-grid img {
  width: 100%;
  height: 64px;
  object-fit: cover;
  border-radius: 4px;
}
.image-input-grid span {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
