<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import type { Capability, Model, ProviderConnection } from "../api/types";

const connections = ref<ProviderConnection[]>([]);
const models = ref<Model[]>([]);
const selectedID = ref("");
const draft = ref<ProviderConnection>(blankConnection());
const creating = ref(false);
const apiKey = ref("");
const modelID = ref("");
const modelName = ref("");
const modelCapabilities = ref<Capability[]>([]);
const addingModel = ref(false);
const busy = ref(false);
const message = ref("");
const messageIsError = ref(false);
const filter = ref<Capability | "all">("all");
const modelQuery = ref("");
const shownCount = ref(50);
const active = computed(() => connections.value.find((item) => item.id === selectedID.value));
const filteredModels = computed(() => models.value.filter((item) =>
  (filter.value === "all" || item.capabilities.includes(filter.value)) &&
  `${item.name} ${item.remote_id}`.toLowerCase().includes(modelQuery.value.toLowerCase().trim()),
));
const visibleModels = computed(() => filteredModels.value.slice(0, shownCount.value));
const labels: Record<Capability, string> = { story: "文本生成", image: "文生图", video: "图生视频" };

function showError(error: unknown) {
  const detail = (error as Error).message;
  message.value = detail.includes("base_url must be an HTTPS URL")
    ? "API 地址必须使用 HTTPS，且不能包含账号、查询参数或 # 片段。"
    : detail.includes("base_url must be a complete API endpoint")
    ? "请输入完整的 API 接口地址，不能只填域名。"
    : detail;
  messageIsError.value = true;
}

function blankConnection(): ProviderConnection {
  return { id: crypto.randomUUID(), name: "", vendor: "bailian", region: "", workspace_id: "", base_url: "", credential_set: false };
}
async function loadConnections(select?: string) {
  connections.value = (await api.connections()).connections;
  selectConnection(select || selectedID.value || connections.value[0]?.id || "");
}
function selectConnection(id: string) {
  selectedID.value = id;
  creating.value = false;
  draft.value = { ...(connections.value.find((item) => item.id === id) || blankConnection()) };
  apiKey.value = "";
  addingModel.value = false;
  modelQuery.value = "";
  shownCount.value = 50;
  if (id) void loadModels(id);
  else models.value = [];
}
async function loadModels(connectionID: string) {
  try { models.value = (await api.models(undefined, connectionID)).models; }
  catch (error) { showError(error); }
}
function newConnection() {
  selectedID.value = "";
  models.value = [];
  draft.value = blankConnection();
  apiKey.value = "";
  creating.value = true;
  addingModel.value = false;
}
async function saveConnection() {
  busy.value = true;
  message.value = "";
  messageIsError.value = false;
  try {
    const saved = await api.saveConnection(draft.value, apiKey.value, creating.value);
    await loadConnections(saved.id);
    message.value = "连接已保存";
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function sync() {
  if (!active.value) return;
  busy.value = true;
  message.value = "";
  messageIsError.value = false;
  try {
    const result = await api.syncModels(active.value.id);
    await loadModels(active.value.id);
    message.value = `同步完成，发现 ${result.synced} 个模型`;
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function testConnection() {
  if (!active.value) return;
  busy.value = true;
  message.value = "";
  messageIsError.value = false;
  try {
    const result = await api.testConnection(active.value.id);
    message.value = `连接正常，可查询 ${result.model_count} 个模型`;
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function addModel() {
  if (!active.value || !modelID.value.trim()) return;
  busy.value = true;
  message.value = "";
  messageIsError.value = false;
  try {
    const model = await api.addModel(active.value.id, modelID.value.trim(), modelName.value.trim(), modelCapabilities.value);
    await loadModels(active.value.id);
    modelID.value = "";
    modelName.value = "";
    modelCapabilities.value = [];
    addingModel.value = false;
    message.value = model.supported ? "模型已添加" : "模型已登记，请配置能力后启用";
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function changeModel(model: Model, values: { enabled?: boolean; default?: boolean; capabilities?: Capability[] }) {
  busy.value = true;
  message.value = "";
  messageIsError.value = false;
  try { await api.updateModel(model, values); await loadModels(model.connection_id); }
  catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function removeModel(model: Model) {
  if (!confirm(`删除模型 ${model.name}？`)) return;
  busy.value = true;
  messageIsError.value = false;
  try { await api.deleteModel(model); await loadModels(model.connection_id); }
  catch (error) { showError(error); }
  finally { busy.value = false; }
}
async function removeConnection() {
  if (!active.value || !confirm(`删除连接 ${active.value.name}？请先移除其模型。`)) return;
  busy.value = true;
  messageIsError.value = false;
  try { await api.deleteConnection(active.value.id); selectedID.value = ""; await loadConnections(); }
  catch (error) { showError(error); }
  finally { busy.value = false; }
}
onMounted(() => { void loadConnections().catch(showError); });
</script>

<template>
  <main class="config-page catalog-page">
    <header class="page-intro">
      <div><span class="eyebrow">MODEL MANAGEMENT</span><h1>模型管理</h1></div>
      <button class="primary" @click="newConnection">添加厂商连接</button>
    </header>
    <p v-if="message" :class="['notice', { error: messageIsError }]" :role="messageIsError ? 'alert' : 'status'">{{ message }}</p>
    <div :class="['catalog-layout', { 'catalog-layout-creating': creating }]">
      <aside class="catalog-sidebar">
        <h2>厂商连接</h2>
        <button v-for="connection in connections" :key="connection.id"
          :class="['catalog-connection', { active: selectedID === connection.id }]"
          @click="selectConnection(connection.id)">
          <strong>{{ connection.name }}</strong>
          <small>{{ connection.vendor === 'bailian' ? '百炼' : connection.vendor }} · {{ connection.base_url || '未配置地址' }}</small>
        </button>
        <div v-if="!connections.length" class="empty">暂无厂商连接</div>
      </aside>

      <section class="catalog-main">
        <template v-if="active && !creating">
          <header class="catalog-toolbar">
            <div><h2>{{ active.name }}</h2><small>模型列表</small></div>
            <div class="actions">
              <button :disabled="busy || !active.credential_set" @click="testConnection">测试连接</button>
              <button :disabled="busy || !active.credential_set" @click="sync">同步模型</button>
              <button class="primary" :disabled="busy" @click="addingModel = !addingModel">手动添加</button>
            </div>
          </header>
          <form v-if="addingModel" class="catalog-add" @submit.prevent="addModel">
            <label>模型 ID<input v-model="modelID" required placeholder="例如 qwen-plus" /></label>
            <label>显示名称<input v-model="modelName" placeholder="默认使用模型 ID" /></label>
            <fieldset class="capability-select"><legend>支持的能力</legend>
              <label v-for="cap in (['story', 'image', 'video'] as const)" :key="cap"><input v-model="modelCapabilities" type="checkbox" :value="cap" />{{ labels[cap] }}</label>
            </fieldset>
            <button type="button" @click="addingModel = false">取消</button>
            <button class="primary" :disabled="busy" type="submit">添加</button>
          </form>
          <div class="catalog-filters" aria-label="模型类型">
            <button v-for="item in ([['all', '全部'], ['story', '文本生成'], ['image', '文生图'], ['video', '图生视频']] as const)"
              :key="item[0]" :class="{ active: filter === item[0] }" @click="filter = item[0]">{{ item[1] }}</button>
          </div>
          <label class="catalog-model-search">查找模型<input v-model="modelQuery" type="search" placeholder="名称或模型 ID" @input="shownCount = 50" /></label>
          <div class="catalog-table-wrap">
            <table class="catalog-table">
              <thead><tr><th>模型</th><th>操作</th><th>来源</th><th>状态</th><th>默认</th><th></th></tr></thead>
              <tbody>
                <tr v-for="model in visibleModels" :key="model.id">
                  <td><strong>{{ model.name }}</strong><small>{{ model.remote_id }}</small></td>
                  <td><div class="capability-select" :aria-label="`${model.name} 支持的能力`">
                    <label v-for="cap in (['story', 'image', 'video'] as const)" :key="cap"><input type="checkbox" :checked="model.capabilities?.includes(cap)" :disabled="busy" @change="changeModel(model, { capabilities: model.capabilities?.includes(cap) ? model.capabilities.filter((item) => item !== cap) : [...(model.capabilities || []), cap] })" />{{ labels[cap] }}</label>
                  </div></td>
                  <td>{{ model.source === 'manual' ? '手动添加' : '同步' }}</td>
                  <td><label v-if="model.supported" class="toggle"><input type="checkbox" :checked="model.enabled" :disabled="busy" @change="changeModel(model, { enabled: !model.enabled })" />启用</label><span v-else class="catalog-unsupported">待配置能力</span></td>
                  <td><input v-if="model.supported" type="checkbox" :checked="model.default" :disabled="busy || !model.enabled" aria-label="设为默认模型" @change="changeModel(model, { default: !model.default })" /></td>
                  <td><button class="quiet" :disabled="busy" :aria-label="`删除 ${model.name}`" @click="removeModel(model)">删除</button></td>
                </tr>
              </tbody>
            </table>
            <div v-if="!visibleModels.length" class="empty">暂无模型</div>
          </div>
          <button v-if="shownCount < filteredModels.length" class="quiet" @click="shownCount += 50">显示更多模型 ({{ filteredModels.length - shownCount }})</button>
        </template>
        <div v-else class="empty">选择或添加厂商连接</div>
      </section>

      <aside v-if="active || creating" class="catalog-settings">
        <h2>{{ creating ? '添加连接' : '连接设置' }}</h2>
        <form @submit.prevent="saveConnection">
          <label>名称<input v-model="draft.name" required placeholder="例如 百炼 · 北京" /></label>
          <label>厂商<select v-model="draft.vendor" :disabled="!creating"><option value="bailian">阿里云百炼</option></select></label>
          <label v-if="draft.vendor === 'bailian'">API 完整地址<input v-model.trim="draft.base_url" required type="url" placeholder="https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions" /></label>
          <label>API Key<input v-model="apiKey" type="password" autocomplete="new-password" :placeholder="draft.credential_set ? '已配置，留空表示不修改' : '输入 API Key'" /></label>
          <div class="catalog-settings-actions"><button class="primary" type="submit" :disabled="busy">保存连接</button><button v-if="active" type="button" class="quiet" :disabled="busy" @click="removeConnection">删除连接</button></div>
        </form>
      </aside>
    </div>
  </main>
</template>
