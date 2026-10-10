<script setup lang="ts">
import VideoPreview from "../VideoPreview.vue";
defineProps<{
  url?: string;
  image?: string;
  title: string;
  hideTitle?: boolean;
}>();
</script>
<template>
  <section class="shot-preview" :class="{ 'video-result': !!url }">
    <h3 v-if="!hideTitle">
      {{ title }} · {{ url ? "视频预览" : image ? "图片预览" : "预览" }}
    </h3>
    <VideoPreview v-if="url" :src="url" :title="title" />
    <img v-else-if="image" :src="image" :alt="title" />
    <p v-else>添加场景与人物，生成并选用首帧后继续制作视频。</p>
  </section>
</template>

<style scoped>
.shot-preview.video-result {
  background: #f4f6f8;
  border: 0;
  padding: 16px;
}
</style>
