<script setup lang="ts">
defineProps<{ status: string }>();
const labels: Record<string, string> = {
  queued: "排队中",
  running: "执行中",
  succeeded: "已完成",
  failed: "失败",
  cancelled: "已取消",
  healthy: "已验证",
  enabled: "已启用",
  disabled: "未启用",
};
</script>
<template>
  <span class="badge" :class="`badge-${status}`">{{
    labels[status] || status
  }}</span>
</template>
