<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import { useWorkspaceStore } from "../stores/workspace";
import type { Material, MaterialKind } from "../api/types";

const workspace = useWorkspaceStore();
const materials = ref<Material[]>([]);
const activeKind = ref<MaterialKind>("visual");
const fileInput = ref<HTMLInputElement>();
const uploading = ref(false);
const message = ref("");
const messageIsError = ref(false);
const tabs: { id: MaterialKind; label: string }[] = [
  { id: "visual", label: "画面" },
  { id: "character", label: "角色" },
  { id: "frame", label: "首尾帧" },
  { id: "voice", label: "配音" },
  { id: "music", label: "音乐" },
];
const accept = computed(() =>
  activeKind.value === "voice" || activeKind.value === "music"
    ? "audio/*"
    : "image/*",
);

async function load() {
  materials.value = (await api.materials(activeKind.value)).materials;
}
async function selectKind(kind: MaterialKind) {
  activeKind.value = kind;
  message.value = "";
  await load();
}
function chooseFile() {
  fileInput.value?.click();
}
async function upload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  uploading.value = true;
  message.value = "";
  messageIsError.value = false;
  try {
    const saved = await api.uploadMaterial(file, activeKind.value);
    workspace.selectedMaterials[activeKind.value] = saved.id;
    await load();
    message.value = "素材“" + saved.name + "”已上传并选中。";
  } catch (error) {
    message.value = (error as Error).message;
    messageIsError.value = true;
  } finally {
    uploading.value = false;
    input.value = "";
  }
}
async function remove(item: Material) {
  if (!window.confirm("删除素材“" + item.name + "”？")) return;
  message.value = "";
  messageIsError.value = false;
  try {
    await api.deleteMaterial(item.id);
    if (workspace.selectedMaterials[item.kind] === item.id)
      delete workspace.selectedMaterials[item.kind];
    await load();
    message.value = "素材已删除。";
  } catch (error) {
    message.value = (error as Error).message;
    messageIsError.value = true;
  }
}
function preview(item: Material) {
  return api.mediaUrl(item.url);
}

onMounted(() => void load());
</script>

<template>
  <main class="page materials-page">
    <header class="page-intro split">
      <div>
        <span class="eyebrow">ASSET LIBRARY</span>
        <h1>素材管理</h1>
        <p>集中管理画面、角色、首尾帧、配音和音乐素材。</p>
      </div>
      <button class="primary" :disabled="uploading" @click="chooseFile">
        {{ uploading ? "正在上传…" : "上传素材" }}
      </button>
    </header>
    <p
      v-if="message"
      :class="['notice', { error: messageIsError }]"
      :role="messageIsError ? 'alert' : 'status'"
    >
      {{ message }}
    </p>
    <div class="filters material-kind-tabs" aria-label="素材类型">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="{ active: activeKind === tab.id }"
        @click="selectKind(tab.id)"
      >
        {{ tab.label }}
      </button>
    </div>
    <input
      ref="fileInput"
      class="file-input"
      type="file"
      :accept="accept"
      aria-label="选择要上传的素材"
      @change="upload"
    />
    <section class="materials-grid">
      <article
        v-for="item in materials"
        :key="item.id"
        :class="[
          'card',
          'material-card',
          { selected: workspace.selectedMaterials[activeKind] === item.id },
        ]"
      >
        <img
          v-if="accept === 'image/*'"
          :src="preview(item)"
          :alt="item.name"
          loading="lazy"
        />
        <audio v-else :src="preview(item)" controls preload="metadata" />
        <div class="split">
          <div>
            <strong>{{ item.name }}</strong>
            <small>{{ new Date(item.created_at).toLocaleString() }}</small>
          </div>
          <button
            class="quiet"
            :aria-label="'删除 ' + item.name"
            @click="remove(item)"
          >
            删除
          </button>
        </div>
        <button
          class="full"
          :class="{
            primary: workspace.selectedMaterials[activeKind] === item.id,
          }"
          @click="workspace.selectedMaterials[activeKind] = item.id"
        >
          {{
            workspace.selectedMaterials[activeKind] === item.id
              ? "当前选用"
              : "选用素材"
          }}
        </button>
      </article>
      <button
        v-if="!materials.length"
        class="empty material-library-empty"
        :disabled="uploading"
        @click="chooseFile"
      >
        <strong>当前素材库为空</strong>
        <span>点击上传{{ accept === "audio/*" ? "音频" : "图片" }}</span>
      </button>
    </section>
  </main>
</template>
