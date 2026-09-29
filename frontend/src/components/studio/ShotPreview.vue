<script setup lang="ts">
defineProps<{url?:string;image?:string;title:string}>();
</script>
<template><section class="shot-preview"><h3>{{title}} · 视频预览</h3><video v-if="url" :src="url" controls preload="metadata" /><img v-else-if="image" :src="image" alt="当前选用首帧" /><p v-else>添加场景与人物，生成并选用首帧后继续制作视频。</p></section></template>
