<script setup lang="ts">
defineProps<{name: 'home' | 'studio' | 'images' | 'tasks' | 'settings' | 'exit' | 'person' | 'scene'}>();
const paths = {
  home: 'm3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z',
  studio: 'M4 9h16v11H4ZM4 9V4h16v5M8 4l3 5m3-5 3 5',
  images: 'M4 4h16v16H4Zm0 12 5-5 4 4 3-3 4 4M15 8h.01',
  tasks: 'm3 6 2 2 3-4m3 2h10M3 13l2 2 3-4m3 2h10M3 20l2 2 3-4m3 2h10',
  settings: 'M4 6h16M4 12h16M4 18h16M8 4v4m8 2v4m-6 2v4',
  exit: 'M10 4H4v16h6m-1-8h12m-4-4 4 4-4 4',
  person: 'M8 7a4 4 0 1 0 8 0 4 4 0 1 0-8 0M4 21v-2a8 8 0 0 1 16 0v2',
  scene: 'M4 4h16v16H4Zm0 12 5-5 4 4 3-3 4 4M15 8h.01',
};
</script>
<template><svg class="app-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path :d="paths[name]" /></svg></template>
