<script setup lang="ts">
import { api } from "../api/client";

async function shutdown() {
  if (!window.confirm("确定退出 FrameFlow 吗？")) return;
  await api.shutdown().catch(() => undefined);
}
</script>

<template>
  <header class="app-header">
    <RouterLink class="brand" to="/"
      ><span class="brand-mark">F</span><strong>FrameFlow</strong><span class="brand-tagline">中国 AI 影视创作工作室</span></RouterLink
    >
    <nav>
      <RouterLink to="/studio">创作工作台</RouterLink>
      <RouterLink to="/materials">素材管理</RouterLink>
      <RouterLink to="/tasks">任务中心</RouterLink>
      <RouterLink to="/config">模型管理</RouterLink>
    </nav>
    <button class="header-exit" type="button" @click="shutdown">退出</button>
  </header>
</template>
