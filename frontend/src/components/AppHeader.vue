<script setup lang="ts">
import { api } from "../api/client";
import { useRoute } from "vue-router";
import AppIcon from './AppIcon.vue';
const route = useRoute();
const links = [
  { to: '/', label: '创作首页', icon: 'home' },
  { to: '/studio', label: '创作工作台', icon: 'studio' },
  { to: '/materials', label: '素材管理', icon: 'images' },
  { to: '/tasks', label: '任务中心', icon: 'tasks' },
  { to: '/config', label: '模型管理', icon: 'settings' },
] as const;
function active(to: string) { return to === '/' ? route.path === '/' : route.path.startsWith(to); }

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
    <nav aria-label="全局导航">
      <RouterLink v-for="link in links" :key="link.to" :to="link.to" :class="{ 'is-active': active(link.to) }" :aria-current="active(link.to) ? 'page' : undefined"><AppIcon :name="link.icon" /><span>{{link.label}}</span></RouterLink>
    </nav>
    <div class="sidebar-footer"><span>AI 影视创作工作室</span><button class="header-exit" type="button" @click="shutdown"><AppIcon name="exit" />退出</button></div>
  </header>
</template>
