<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import type { Capability, Provider } from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
const active = ref<Capability>("story"),
  items = ref<Provider[]>([]),
  selectedCode = ref(""),
  message = ref(""),
  saving = ref(false),
  apiKey = ref(""),
  readiness = ref<Record<Capability, boolean>>({
    story: false,
    image: false,
    video: false,
  });
const selected = computed(() =>
  items.value.find((v) => v.code === selectedCode.value),
);
const tabs: { id: Capability; label: string; help: string }[] = [
  { id: "story", label: "故事助手", help: "故事与分镜生成" },
  { id: "image", label: "生图助手", help: "角色与首尾帧" },
  { id: "video", label: "视频助手", help: "异步视频生成" },
];
async function load() {
  items.value = (await api.providers(active.value)).providers;
  readiness.value[active.value] = items.value.some(
    (v) => v.status === "healthy",
  );
  selectedCode.value = items.value[0]?.code || "";
}
async function loadReadiness() {
  for (const tab of tabs)
    readiness.value[tab.id] = (await api.providers(tab.id)).providers.some(
      (v) => v.status === "healthy",
    );
}
async function setTab(id: Capability) {
  active.value = id;
  await load();
}
function add() {
  const code = `${active.value}-${Date.now()}`;
  items.value.unshift({
    code,
    name: "新增接口",
    capability: active.value,
    model: "",
    base_url: "",
    enabled: false,
    status: "disabled",
  });
  selectedCode.value = code;
}
async function save() {
  if (!selected.value) return;
  saving.value = true;
  try {
    await api.saveProvider(selected.value, apiKey.value);
    message.value = "配置已保存。";
    await loadReadiness();
  } catch (e) {
    message.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}
async function test() {
  if (!selected.value) return;
  try {
    Object.assign(selected.value, await api.testProvider(selected.value.code));
    readiness.value[active.value] = true;
    message.value = "连接校验通过。";
  } catch (e) {
    message.value = (e as Error).message;
  }
}
onMounted(async () => {
  await loadReadiness();
  await load();
});
</script>
<template>
  <main class="config-page">
    <div class="page-intro split">
      <div>
        <span class="eyebrow">WORKSPACE SETTINGS</span>
        <h1>配置中心</h1>
        <p>管理故事、生图、视频接口，工作台只读取已启用且校验通过的配置。</p>
      </div>
      <button class="primary" :disabled="!selected || saving" @click="save">
        保存配置
      </button>
    </div>
    <div v-if="message" class="notice">{{ message }}</div>
    <div class="config-grid">
      <aside class="panel config-nav">
        <div class="panel-head"><h2>配置类型</h2></div>
        <div class="panel-body">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            :class="{ active: active === tab.id }"
            @click="setTab(tab.id)"
          >
            <span>{{ tab.label }}</span
            ><small>{{ tab.help }}</small>
          </button>
          <hr />
          <RouterLink to="/studio">返回工作台</RouterLink>
        </div>
      </aside>
      <section class="panel">
        <div class="panel-head split">
          <div>
            <h2>{{ tabs.find((v) => v.id === active)?.label }}</h2>
            <small>{{ tabs.find((v) => v.id === active)?.help }}</small>
          </div>
          <button @click="add">新增接口</button>
        </div>
        <div class="provider-layout">
          <aside class="provider-list">
            <button
              v-for="item in items"
              :key="item.code"
              :class="{ active: selectedCode === item.code }"
              @click="selectedCode = item.code"
            >
              <strong>{{ item.name }}</strong
              ><small>{{ item.model || "待填写模型" }}</small
              ><StatusBadge :status="item.status" />
            </button>
            <div v-if="!items.length" class="empty">暂无配置。</div>
          </aside>
          <form v-if="selected" class="provider-form" @submit.prevent="save">
            <div class="split">
              <div>
                <span class="eyebrow">{{ active.toUpperCase() }} MODEL</span>
                <h2>{{ selected.name }}</h2>
              </div>
              <label class="toggle"
                ><input v-model="selected.enabled" type="checkbox" />启用</label
              >
            </div>
            <div class="form-grid">
              <label>显示名称<input v-model="selected.name" required /></label
              ><label>配置标识<input v-model="selected.code" required /></label
              ><label>模型<input v-model="selected.model" required /></label
              ><label
                >Base URL<input
                  v-model="selected.base_url"
                  type="url"
                  required
                  placeholder="https://api.example.com/v1" /></label
              ><label
                >API Key<input
                  v-model="apiKey"
                  type="password"
                  autocomplete="new-password"
                  placeholder="保存后仅显示已设置状态" /></label
              ><label>超时时间<input value="60 秒" disabled /></label>
            </div>
            <div class="connection-row">
              <StatusBadge :status="selected.status" /><span>{{
                selected.last_error ||
                "保存后测试连接，工作台只显示校验通过的接口。"
              }}</span
              ><button type="button" @click="test">测试连接</button>
            </div>
            <div class="actions end">
              <button type="submit" class="primary">保存配置</button>
            </div>
          </form>
        </div>
      </section>
      <aside class="panel config-check">
        <div class="panel-head"><h2>配置检查</h2></div>
        <div class="panel-body">
          <div v-for="tab in tabs" :key="tab.id" class="check-row">
            <span>{{ readiness[tab.id] ? "✓" : "!" }}</span>
            <div>
              <strong>{{ tab.label }}</strong
              ><small>{{
                readiness[tab.id] ? "可在工作台使用" : "需要配置并校验"
              }}</small>
            </div>
          </div>
          <div class="tip">
            角色和首尾帧共用生图接口。API Key 不会回显到工作台。
          </div>
        </div>
      </aside>
    </div>
  </main>
</template>
