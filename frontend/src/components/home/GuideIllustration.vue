<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { guideImages, guideImageUrl, type GuideImage } from "./guideImages";
const props = defineProps<{ step: number }>();
const emit = defineEmits<{ zoom: [image: GuideImage] }>();
const index = ref(0);
watch(
  () => props.step,
  () => {
    index.value = 0;
  },
);
const slides = computed(() => guideImages[props.step]!);
const current = computed(() => slides.value[index.value]!);
</script>

<template>
  <figure class="guide-illustration">
    <figcaption>
      <strong>看图操作</strong><span>操作示意 · 演示数据</span>
    </figcaption>
    <div class="guide-image-tabs" role="group" aria-label="本步操作图片">
      <button
        v-for="(item, i) in slides"
        :key="item.image"
        type="button"
        :aria-pressed="index === i"
        @click="index = i"
      >
        {{ i + 1 }}. {{ item.title }}
      </button>
    </div>
    <p class="guide-image-location">{{ current.location }}</p>
    <button
      type="button"
      class="guide-image-trigger"
      :aria-label="`放大查看：${current.title}`"
      @click="emit('zoom', current)"
    >
      <img :src="guideImageUrl(current)" :alt="current.alt" />
      <span>放大查看 ↗</span>
    </button>
    <div class="guide-image-caption" aria-live="polite" aria-atomic="true">
      <h4>{{ current.title }}</h4>
      <ol>
        <li v-for="(instruction, n) in current.instructions" :key="instruction">
          <b aria-hidden="true">{{ n + 1 }}</b
          ><span>{{ instruction }}</span>
        </li>
      </ol>
      <p><strong>完成后：</strong>{{ current.done }}</p>
    </div>
    <div class="guide-image-pagination">
      <button type="button" :disabled="index === 0" @click="index--">
        上一张
      </button>
      <span>{{ index + 1 }} / {{ slides.length }}</span>
      <button
        type="button"
        :disabled="index === slides.length - 1"
        @click="index++"
      >
        下一张
      </button>
    </div>
  </figure>
</template>
