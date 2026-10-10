<script setup lang="ts">
import { requestAddress } from "./connectionAddress";
defineProps<{
  baseUrl: string;
  path?: string;
  pollingPath?: string;
  method?: string;
}>();
</script>
<template>
  <div class="request-address" aria-label="最终请求地址">
    <span
      >{{ pollingPath ? "提交任务" : "请求地址" }} ·
      {{ method || "POST" }}</span
    >
    <code>{{ requestAddress(baseUrl, path) }}</code>
    <template v-if="pollingPath">
      <span>查询任务 · GET</span>
      <code>{{ requestAddress(baseUrl, pollingPath) }}</code>
    </template>
  </div>
</template>
<style scoped>
.request-address {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 12px;
  border-radius: 6px;
  background: #f4f7fa;
}
span {
  color: #53657b;
  font-size: 12px;
}
code {
  color: #20334b;
  white-space: normal;
  overflow-wrap: anywhere;
  font-size: 12px;
}
code + span {
  margin-top: 8px;
}
</style>
