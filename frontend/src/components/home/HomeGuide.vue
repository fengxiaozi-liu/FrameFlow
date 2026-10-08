<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { steps, type HomeExample } from "./content";
const props = defineProps<{ initialStep: number; example: HomeExample }>();
const emit = defineEmits<{ close: []; start: [] }>();
const step = ref(props.initialStep),
  dialog = ref<HTMLDialogElement>();
let previous: HTMLElement | null = null,
  overflow = "";
function trapFocus(event: KeyboardEvent) {
  if (event.key !== "Tab") return;
  const buttons = dialog.value?.querySelectorAll<HTMLButtonElement>(
    "button:not(:disabled)",
  );
  if (!buttons?.length) return;
  const first = buttons[0],
    last = buttons[buttons.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}
onMounted(() => {
  previous = document.activeElement as HTMLElement;
  overflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  dialog.value?.showModal();
});
onBeforeUnmount(() => {
  dialog.value?.close();
  document.body.style.overflow = overflow;
  if (previous?.isConnected) previous.focus();
});
</script>
<template>
  <Teleport to="body"
    ><dialog
      ref="dialog"
      class="home-guide"
      aria-labelledby="home-guide-title"
      @cancel.prevent="emit('close')"
      @keydown="trapFocus"
      @click="$event.target === dialog && emit('close')"
    >
      <header>
        <div>
          <span class="home-eyebrow">创作入门 · 图文指南</span>
          <h2 id="home-guide-title">从一个想法，到一支视频</h2>
        </div>
        <button
          type="button"
          aria-label="关闭入门指南"
          autofocus
          @click="emit('close')"
        >
          ×
        </button>
      </header>
      <nav aria-label="指南步骤">
        <button
          v-for="(item, index) in steps"
          :key="item.title"
          type="button"
          :aria-current="step === index ? 'step' : undefined"
          @click="step = index"
        >
          0{{ index + 1 }} {{ item.title }}
        </button>
      </nav>
      <div class="home-guide-body">
        <figure>
          <img :src="example.image" :alt="example.alt" />
          <figcaption>创意示例 · {{ example.title }}</figcaption>
        </figure>
        <section aria-live="polite" aria-atomic="true">
          <span class="home-eyebrow">STEP 0{{ step + 1 }}</span>
          <h3>{{ steps[step].title }}</h3>
          <p>{{ steps[step].body }}</p>
          <p class="home-guide-tip">{{ steps[step].tip }}</p>
          <blockquote v-if="step === 0">{{ example.idea }}</blockquote>
        </section>
      </div>
      <footer>
        <p>使用示例会把想法带入工作台的“故事联想”，由你决定何时生成。</p>
        <div>
          <button type="button" :disabled="step === 0" @click="step--">
            上一步</button
          ><button
            v-if="step < 2"
            type="button"
            class="primary"
            @click="step++"
          >
            下一步</button
          ><button v-else type="button" class="primary" @click="emit('start')">
            使用示例想法
          </button>
        </div>
      </footer>
    </dialog></Teleport
  >
</template>
