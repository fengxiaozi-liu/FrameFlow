<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { api } from "../api/client";
import type { Material, MaterialKind } from "../api/types";

const items = ref<Material[]>([]);
const category = ref<MaterialKind | "">("");
const uploadCategory = ref<MaterialKind>("scene");
const search = ref("");
const mediaType = ref("");
const sortOrder = ref<"newest" | "oldest" | "name">("newest");
const selected = ref<Material | null>(null);
const uses = ref<string[]>([]);
const input = ref<HTMLInputElement>();
const uploading = ref(false);
const progress = ref(0);
const error = ref("");
const message = ref("");
const loaded = ref(false);
const categories: { id: MaterialKind | ""; label: string }[] = [
  { id: "", label: "全部" }, { id: "scene", label: "场景" },
  { id: "character", label: "角色" }, { id: "prop", label: "道具" },
  { id: "voice", label: "配音" }, { id: "music", label: "音乐" },
];
const accept = computed(() => ["voice", "music"].includes(uploadCategory.value) ? "audio/*" : "image/*");
const hasFilters = computed(() => !!category.value || !!search.value.trim() || !!mediaType.value);
const sortedItems = computed(() => [...items.value].sort((a, b) => {
  if (sortOrder.value === "name") return a.name.localeCompare(b.name, "zh");
  const difference = new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  return sortOrder.value === "newest" ? difference : -difference;
}));
let timer: ReturnType<typeof setTimeout> | undefined;

async function load() {
  try {
    items.value = (await api.materials(category.value, search.value, mediaType.value)).materials || [];
    if (selected.value && !items.value.some((item) => item.id === selected.value?.id)) selected.value = null;
    error.value = "";
  }
  catch (cause) { error.value = (cause as Error).message; }
  finally { loaded.value = true; }
}
watch([category, mediaType], () => { if (category.value) uploadCategory.value = category.value; void load(); });
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
    category.value = uploadCategory.value; search.value = ""; mediaType.value = "";
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
    <header class="materials-header">
      <div><h1>素材管理</h1><p>让每个镜头，都有合适的素材</p></div>
      <div class="materials-upload-actions"><label>上传类别<select v-model="uploadCategory" aria-label="上传素材类别"><option v-for="tab in categories.filter((item) => item.id)" :key="tab.id" :value="tab.id">{{ tab.label }}</option></select></label>
        <button class="primary" :disabled="uploading" @click="input?.click()">{{ uploading ? `上传中 ${progress}%` : "＋ 上传素材" }}</button></div>
    </header>
    <input ref="input" class="file-input" type="file" :accept="accept" aria-label="选择素材文件" @change="upload" />
    <div v-if="uploading" class="progress" role="progressbar" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: progress + '%' }" /></div>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p><p v-if="message" class="notice" role="status">{{ message }}</p>
    <nav class="material-tabs" aria-label="素材类型"><button v-for="tab in categories" :key="tab.id" :class="{ active: category === tab.id }" :aria-current="category === tab.id ? 'page' : undefined" @click="category = tab.id">{{ tab.label }}</button></nav>
    <div class="material-tools"><label class="material-search"><span class="material-search-icon" aria-hidden="true">⌕</span><span class="sr-only">搜索名称或标签</span><input v-model="search" type="search" aria-label="搜索名称或标签" placeholder="搜索素材名称或标签" /></label><label><span class="sr-only">媒体类型</span><select v-model="mediaType" aria-label="媒体类型"><option value="">全部类型</option><option value="image">图片</option><option value="audio">音频</option></select></label><label><span class="sr-only">排序</span><select v-model="sortOrder" aria-label="排序"><option value="newest">最近添加</option><option value="oldest">最早添加</option><option value="name">按名称</option></select></label></div>
    <div class="material-workspace" :class="{ 'has-selection': !!selected }"><section class="materials-grid" aria-label="素材列表">
      <button v-for="item in sortedItems" :key="item.id" class="card material-card" :class="{ selected: selected?.id === item.id }" :aria-pressed="selected?.id === item.id" @click="inspect(item)">
        <span class="material-card-preview"><img v-if="item.kind !== 'voice' && item.kind !== 'music'" :src="api.mediaUrl(item.url)" :alt="item.name" loading="lazy" /><span v-else class="audio-tile"><span class="audio-play" aria-hidden="true">▶</span><span class="audio-wave" aria-hidden="true">▂▅▃▆▂▄▇▃▅</span></span></span>
        <span class="material-card-info"><strong>{{ item.name }}</strong><span class="material-kind" :class="item.kind">{{ categories.find((tab) => tab.id === item.kind)?.label }}</span></span>
        <small v-if="item.legacy_kind === 'frame'">原首尾帧</small>
      </button>
      <div v-if="loaded && !items.length && !error" class="material-empty">
        <div class="material-empty-icon" aria-hidden="true"><span>▧</span><span>♫</span></div>
        <h2>{{ hasFilters ? "没有找到符合条件的素材" : "从第一份素材开始" }}</h2>
        <p>{{ hasFilters ? "试试其他关键词或分类，也可以上传新素材。" : "上传场景、角色、道具图片或音频，创作时就能在镜头中使用。" }}</p>
        <div class="material-empty-actions"><button v-if="hasFilters" @click="category = ''; search = ''; mediaType = ''">清除筛选</button><button class="primary" :disabled="uploading" @click="input?.click()">＋ 上传素材</button></div>
        <div class="material-empty-types"><span>场景图片</span><span>角色形象</span><span>道具图片</span><span>配音</span><span>音乐</span></div>
      </div>
    </section><aside v-if="selected" class="panel material-detail" aria-label="素材详情"><div class="material-detail-head"><button class="mobile-panel-back" @click="selected = null">返回素材列表</button><h2>素材详情</h2><button class="material-detail-close" aria-label="关闭素材详情" @click="selected = null">×</button></div>
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
