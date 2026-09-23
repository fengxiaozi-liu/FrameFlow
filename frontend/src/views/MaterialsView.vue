<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { api } from "../api/client";
import type { Material, MaterialKind } from "../api/types";

const items = ref<Material[]>([]);
const category = ref<MaterialKind | "">("");
const uploadCategory = ref<MaterialKind>("scene");
const search = ref("");
const mediaType = ref("");
const selected = ref<Material | null>(null);
const uses = ref<string[]>([]);
const input = ref<HTMLInputElement>();
const uploading = ref(false);
const progress = ref(0);
const error = ref("");
const message = ref("");
const categories: { id: MaterialKind | ""; label: string }[] = [
  { id: "", label: "全部" }, { id: "scene", label: "场景" },
  { id: "character", label: "角色" }, { id: "prop", label: "道具" },
  { id: "voice", label: "配音" }, { id: "music", label: "音乐" },
];
const accept = computed(() => ["voice", "music"].includes(uploadCategory.value) ? "audio/*" : "image/*");
let timer: ReturnType<typeof setTimeout> | undefined;

async function load() {
  try { items.value = (await api.materials(category.value, search.value, mediaType.value)).materials || []; error.value = ""; }
  catch (cause) { error.value = (cause as Error).message; }
}
watch([category, mediaType], () => void load());
watch(search, () => { if (timer) clearTimeout(timer); timer = setTimeout(() => void load(), 250); });
onMounted(() => void load());
async function inspect(item: Material) {
  selected.value = { ...item, tags: [...(item.tags || [])] };
  try { uses.value = (await api.materialUses(item.id)).uses || []; }
  catch (cause) { error.value = (cause as Error).message; uses.value = []; }
}
async function upload(event: Event) {
  const field = event.target as HTMLInputElement;
  const file = field.files?.[0];
  if (!file) return;
  uploading.value = true; progress.value = 0; error.value = ""; message.value = "";
  try {
    const item = await api.uploadMaterial(file, uploadCategory.value, (value) => { progress.value = value; });
    await load(); await inspect(item); message.value = `已上传“${item.name}”。`;
  } catch (cause) { error.value = (cause as Error).message; }
  finally { uploading.value = false; field.value = ""; }
}
async function save() {
  if (!selected.value) return;
  try { selected.value = await api.saveMaterial(selected.value); await load(); error.value = ""; message.value = "素材信息已保存。"; }
  catch (cause) { error.value = (cause as Error).message; }
}
async function remove() {
  if (!selected.value || !window.confirm(`删除素材“${selected.value.name}”？`)) return;
  try {
    uses.value = (await api.materialUses(selected.value.id)).uses || [];
    if (uses.value.length) { error.value = "该素材仍被引用，请先在对应位置解绑。"; return; }
    await api.deleteMaterial(selected.value.id);
    selected.value = null; await load(); error.value = ""; message.value = "素材已删除。";
  } catch (cause) { error.value = (cause as Error).message; }
}
function locationLink(use: string) {
  const [project, draft, scene] = use.split("/");
  return project && draft
    ? `/studio/${encodeURIComponent(project)}/${encodeURIComponent(draft)}${scene && scene !== "snapshot" && scene !== "composition" ? `?scene=${encodeURIComponent(scene)}` : ""}`
    : "/tasks";
}
</script>

<template>
  <main class="page materials-page">
    <header class="page-intro"><div><span class="eyebrow">ASSET LIBRARY</span><h1>素材库</h1><p>分类整理图片和音频，在镜头中选择用途。</p></div>
      <div class="actions"><select v-model="uploadCategory" aria-label="上传素材类别"><option v-for="tab in categories.filter((item) => item.id)" :key="tab.id" :value="tab.id">{{ tab.label }}</option></select>
        <button class="primary" :disabled="uploading" @click="input?.click()">{{ uploading ? `上传中 ${progress}%` : "上传素材" }}</button></div></header>
    <input ref="input" class="file-input" type="file" :accept="accept" aria-label="选择素材文件" @change="upload" />
    <div v-if="uploading" class="progress" role="progressbar" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: progress + '%' }" /></div>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p><p v-if="message" class="notice" role="status">{{ message }}</p>
    <nav class="filters" aria-label="素材类型"><button v-for="tab in categories" :key="tab.id" :class="{ active: category === tab.id }" @click="category = tab.id">{{ tab.label }}</button></nav>
    <div class="material-tools"><label>搜索名称或标签<input v-model="search" type="search" placeholder="搜索素材" /></label><label>媒体类型<select v-model="mediaType"><option value="">全部媒体</option><option value="image">图片</option><option value="audio">音频</option></select></label></div>
    <div class="material-workspace" :class="{ 'has-selection': !!selected }"><section class="materials-grid" aria-label="素材列表">
      <button v-for="item in items" :key="item.id" class="card material-card" :class="{ selected: selected?.id === item.id }" @click="inspect(item)">
        <img v-if="item.kind !== 'voice' && item.kind !== 'music'" :src="api.mediaUrl(item.url)" :alt="item.name" loading="lazy" />
        <span v-else class="audio-tile">♫</span><strong>{{ item.name }}</strong><small>{{ categories.find((tab) => tab.id === item.kind)?.label }} · {{ new Date(item.created_at).toLocaleString() }}</small>
        <small v-if="item.legacy_kind === 'frame'">原首尾帧</small>
      </button>
      <p v-if="!items.length" class="empty">没有符合条件的素材。可选择类别上传。</p>
    </section><aside v-if="selected" class="panel material-detail" aria-label="素材详情"><button class="mobile-panel-back" @click="selected = null">返回素材列表</button><h2>素材详情</h2>
      <img v-if="selected.kind !== 'voice' && selected.kind !== 'music'" :src="api.mediaUrl(selected.url)" :alt="selected.name" />
      <audio v-else :src="api.mediaUrl(selected.url)" controls preload="metadata" />
      <label>名称<input v-model="selected.name" /></label><label>分类<select v-model="selected.kind"><option v-for="tab in categories.filter((item) => item.id)" :key="tab.id" :value="tab.id">{{ tab.label }}</option></select></label>
      <label>标签（逗号分隔）<input :value="(selected.tags || []).join(', ')" @change="selected.tags = ($event.target as HTMLInputElement).value.split(',').map((tag) => tag.trim()).filter(Boolean)" /></label>
      <p>格式：{{ selected.media?.format || "未知" }} · 大小：{{ selected.media?.size_bytes || 0 }} 字节</p>
      <p v-if="selected.media?.width">尺寸：{{ selected.media.width }} × {{ selected.media.height }}</p>
      <p v-if="selected.kind === 'voice' || selected.kind === 'music'">时长：{{ selected.media?.duration_seconds?.toFixed(1) || "未知" }} 秒</p>
      <p v-if="selected.legacy_kind === 'frame'">来源：原首尾帧素材，现归入场景。</p>
      <div class="actions"><button class="primary" @click="save">保存信息</button><button @click="remove">删除</button></div>
      <h3>使用位置</h3><p v-if="!uses.length">尚未被引用</p><ul v-else><li v-for="use in uses" :key="use"><RouterLink :to="locationLink(use)">{{ use }}</RouterLink></li></ul>
    </aside></div>
  </main>
</template>
