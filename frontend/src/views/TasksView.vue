<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../api/client";
import { useTaskStore } from "../stores/tasks";
import type { Task, TaskStatus } from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
import ProgressBar from "../components/ProgressBar.vue";
const store = useTaskStore(),
  filter = ref<"all" | TaskStatus>("all"),
  message = ref("");
const visible = computed(() =>
  filter.value === "all"
    ? store.items
    : store.items.filter((v) => v.status === filter.value),
);
onMounted(async () => {
  await store.load();
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
        <span class="eyebrow">后台任务</span>
        <h1>任务中心</h1>
        <p>所有生成操作异步执行，可离开页面，完成后再查看结果。</p>
      </div>
      <button @click="store.load">刷新状态</button>
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
      <article v-for="task in visible" :key="task.id" class="card task-detail">
        <div class="split">
          <div>
            <h2>{{ task.kind }} · {{ task.id.slice(-9) }}</h2>
            <small
              >{{ task.id }} ·
              {{ new Date(task.created_at).toLocaleString() }}</small
            >
          </div>
          <StatusBadge :status="task.status" />
        </div>
        <div class="task-meta">
          <span>阶段：{{ task.stage }}</span
          ><span>接口：{{ task.provider_code || "未指定" }}</span
          ><strong>{{ task.progress }}%</strong>
        </div>
        <ProgressBar :value="task.progress" />
        <p v-if="task.error" class="error-text">{{ task.error }}</p>
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
          ><a
            v-if="task.status === 'succeeded'"
            class="button primary"
            :href="api.resultUrl(task.id)"
            >下载结果</a
          >
        </div>
      </article>
      <div v-if="!visible.length" class="empty">当前筛选下没有任务。</div>
    </div>
  </main>
</template>
