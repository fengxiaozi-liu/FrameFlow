<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";
import { steps, type HomeExample } from "./content";
import GuideIllustration from "./GuideIllustration.vue";
import { guideImageUrl, type GuideImage } from "./guideImages";
const props = defineProps<{ initialStep: number; example: HomeExample }>();
const emit = defineEmits<{ close: []; start: [useExample: boolean] }>();
const router = useRouter();
const step = ref(Math.max(0, Math.min(props.initialStep, steps.length - 1)));
const current = computed(() => steps[step.value]);
const dialog = ref<HTMLDialogElement>();
const body = ref<HTMLElement>();
const preview = ref<GuideImage>();
const previewBack = ref<HTMLButtonElement>();
let imageOpener: HTMLElement | null = null;
async function openImage(image: GuideImage) {
  imageOpener = document.activeElement as HTMLElement;
  preview.value = image;
  await nextTick();
  previewBack.value?.focus();
}
async function closeImage() {
  preview.value = undefined;
  await nextTick();
  imageOpener?.focus();
}
function cancelGuide() {
  if (preview.value) void closeImage();
  else emit("close");
}
let previous: HTMLElement | null = null,
  overflow = "";
watch(
  step,
  () => {
    if (body.value) body.value.scrollTop = 0;
  },
  { flush: "post" },
);
function trapFocus(event: KeyboardEvent) {
  if (event.key === "Escape" && preview.value) {
    event.preventDefault();
    event.stopPropagation();
    void closeImage();
    return;
  }
  if (event.key !== "Tab") return;
  const controls = [
    ...(dialog.value?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), a[href], summary, [tabindex="0"]',
    ) || []),
  ].filter((el) => el.getClientRects().length);
  const first = controls[0],
    last = controls.at(-1);
  if (!first || !last) return;
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}
function proceed() {
  if (step.value === 1) {
    emit("start", true);
    return;
  }
  emit("close");
  void router.push(current.value.destination);
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
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="home-guide home-guide-complete"
      aria-labelledby="home-guide-title"
      @cancel.self.prevent="cancelGuide"
      @keydown="trapFocus"
      @click="$event.target === dialog && emit('close')"
    >
      <header>
        <div>
          <span class="home-eyebrow">创作入门 · 完整操作指南</span>
          <h2 id="home-guide-title">从模型配置，到完整成片</h2>
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
      <nav v-show="!preview" aria-label="指南步骤">
        <button
          v-for="(item, index) in steps"
          :key="item.title"
          type="button"
          :aria-current="step === index ? 'step' : undefined"
          @click="step = index"
        >
          0{{ index + 1 }} {{ item.title
          }}<small v-if="item.optional">可跳过联想</small>
        </button>
      </nav>
      <div v-show="!preview" ref="body" class="home-guide-body">
        <article :key="step" class="guide-instructions">
          <span class="home-eyebrow">STEP 0{{ step + 1 }} / 04</span>
          <h3>
            {{ current.title }}
            <span v-if="current.optional" class="guide-optional"
              >可跳过联想</span
            >
          </h3>
          <p>{{ current.body }}</p>
          <section class="guide-prepare">
            <h4>开始前准备</h4>
            <p>{{ current.prepare }}</p>
          </section>
          <h4>按步骤操作</h4>
          <ol class="guide-action-list">
            <li
              v-for="([title, description], index) in current.actions"
              :key="title"
            >
              <span class="guide-action-number" aria-hidden="true">{{
                index + 1
              }}</span>
              <div>
                <strong>{{ title }}</strong>
                <p>{{ description }}</p>
              </div>
            </li>
          </ol>
          <section v-if="step === 1" class="guide-story-paths">
            <h4>已有故事？直接开始写作</h4>
            <p>跳过 AI 联想，在正文页输入、粘贴或导入故事，再继续生成分镜。</p>
            <button type="button" @click="emit('start', false)">
              已有正文，跳过联想 →
            </button>
            <details>
              <summary>查看示例想法 · {{ example.title }}</summary>
              <blockquote>{{ example.idea }}</blockquote>
            </details>
          </section>
          <section class="guide-checks">
            <h4>完成检查</h4>
            <ul>
              <li v-for="check in current.checks" :key="check">{{ check }}</li>
            </ul>
          </section>
          <p class="home-guide-tip">{{ current.tip }}</p>
          <section class="guide-faq">
            <h4>遇到问题</h4>
            <details v-for="[question, answer] in current.faq" :key="question">
              <summary>{{ question }}</summary>
              <p>{{ answer }}</p>
            </details>
          </section>
        </article>
        <aside class="guide-side">
          <GuideIllustration :step="step" @zoom="openImage" />
          <p class="guide-route-note">
            {{
              step < 2
                ? "可以随时打开对应页面开始操作，再回到首页继续阅读。"
                : "先在工作台选择已有项目，再按本步指引继续制作。"
            }}
          </p>
        </aside>
      </div>
      <section
        v-if="preview"
        class="guide-image-preview"
        aria-label="操作图片放大预览"
      >
        <div class="guide-preview-heading">
          <div>
            <strong>{{ preview.title }}</strong>
            <p>{{ preview.location }} · 演示数据</p>
          </div>
          <button ref="previewBack" type="button" @click="closeImage">
            返回图文指南
          </button>
        </div>
        <p class="guide-preview-hint">
          可滚动查看完整图片。按 Esc 返回图文指南。
        </p>
        <div
          class="guide-preview-scroll"
          tabindex="0"
          role="region"
          aria-label="可滚动的操作图片"
        >
          <img :src="guideImageUrl(preview)" :alt="preview.alt" />
        </div>
        <ol class="guide-preview-notes">
          <li v-for="instruction in preview.instructions" :key="instruction">
            {{ instruction }}
          </li>
        </ol>
      </section>
      <footer v-show="!preview">
        <p>
          {{
            step === 1
              ? "使用示例只填入想法；由你选择模型并点击生成。"
              : "阅读与切换步骤不会创建项目或提交生成任务。"
          }}
        </p>
        <div>
          <button type="button" :disabled="step === 0" @click="step--">
            上一步</button
          ><button type="button" class="primary" @click="proceed">
            {{ current.action }}</button
          ><button v-if="step < steps.length - 1" type="button" @click="step++">
            下一步</button
          ><button v-else type="button" @click="emit('close')">完成阅读</button>
        </div>
      </footer>
    </dialog>
  </Teleport>
</template>
