<script setup lang="ts">
defineProps<{url?:string;image?:string;title:string}>();
</script>
<template><section class="shot-preview"><h3>{{title}} · {{url?'视频预览':image?'图片预览':'预览'}}</h3><video v-if="url" :src="url" controls preload="metadata" /><img v-else-if="image" :src="image" :alt="title" /><p v-else>添加场景与人物，生成并选用首帧后继续制作视频。</p></section></template>
