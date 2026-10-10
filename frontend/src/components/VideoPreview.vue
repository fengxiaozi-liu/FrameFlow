<script setup lang="ts">
import { ref, watch } from "vue";

const props = defineProps<{ src: string; poster?: string; title: string }>();
const ratio = ref(16 / 9);
watch(
  () => props.src,
  () => {
    ratio.value = 16 / 9;
  },
);

function readDimensions(event: Event) {
  const video = event.currentTarget as HTMLVideoElement;
  // Use the returned video's dimensions, which can differ from the requested ratio.
  if (video.videoWidth > 0 && video.videoHeight > 0) {
    ratio.value = video.videoWidth / video.videoHeight;
  }
}
</script>

<template>
  <div class="video-player" :style="{ '--preview-ratio': ratio }">
    <video
      :key="src"
      :src="src"
      :poster="poster"
      :aria-label="title"
      controls
      playsinline
      preload="metadata"
      @loadedmetadata="readDimensions"
    />
  </div>
</template>

<style scoped>
.video-player {
  --preview-height: min(60vh, 520px);
  width: min(100%, 880px, calc(var(--preview-height) * var(--preview-ratio)));
  aspect-ratio: var(--preview-ratio);
  margin-inline: auto;
  overflow: hidden;
  border-radius: 10px;
  background: #111827;
}
.video-player > video {
  display: block;
  width: 100%;
  height: 100%;
  max-height: none;
  object-fit: contain;
}
</style>
