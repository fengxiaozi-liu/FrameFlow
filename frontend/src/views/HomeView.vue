<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import HomeGuide from "../components/home/HomeGuide.vue";
import { examples, steps } from "../components/home/content";
import "../styles/home.css";

const router = useRouter();
const slide = ref(0),
  paused = ref(false),
  hovered = ref(false),
  focused = ref(false);
const guideStep = ref<number | null>(null);
const current = computed(() => examples[slide.value]);
const modelsLoading = ref(true),
  modelsError = ref(false);
const available = ref<string[]>([]);
const capabilities = ["story", "image", "video"] as const;
const capabilityNames = {
  story: "故事辅助",
  image: "参考图生成",
  video: "视频生成",
};
const missing = computed(() =>
  capabilities
    .filter((k) => !available.value.includes(k))
    .map((k) => capabilityNames[k]),
);
const modelHint = computed(() => {
  if (modelsLoading.value) return "正在检查创作准备情况…";
  if (modelsError.value)
    return "暂时无法读取模型配置，你仍可以先写故事、整理分镜。";
  if (!available.value.length)
    return "首次使用？先写下故事，需要 AI 生成时再连接对应模型。";
  if (missing.value.length)
    return `已准备好 ${available.value.map((k) => capabilityNames[k as (typeof capabilities)[number]]).join("、")}；${missing.value.join("、")}可按需配置。`;
  return "故事、参考图和视频模型已配置，可以开始创作了。";
});
let alive = true;
async function loadModels() {
  modelsLoading.value = true;
  modelsError.value = false;
  try {
    const data = await api.models();
    if (alive)
      available.value = capabilities.filter((k) =>
        data.models.some(
          (m) => m.enabled && m.supported && m.capabilities.includes(k),
        ),
      );
  } catch {
    if (alive) modelsError.value = true;
  } finally {
    if (alive) modelsLoading.value = false;
  }
}
function start(idea = "") {
  sessionStorage.setItem("frameflow-idea", idea);
  void router.push("/studio/new");
}
function select(index: number) {
  slide.value = (index + examples.length) % examples.length;
  paused.value = true;
}
const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
function syncMotion() {
  paused.value = motion.matches;
}
let timer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  void loadModels();
  syncMotion();
  motion.addEventListener("change", syncMotion);
  timer = setInterval(() => {
    if (
      !paused.value &&
      !hovered.value &&
      !focused.value &&
      guideStep.value === null &&
      !document.hidden
    )
      slide.value = (slide.value + 1) % examples.length;
  }, 7000);
});
onBeforeUnmount(() => {
  alive = false;
  clearInterval(timer);
  motion.removeEventListener("change", syncMotion);
});
</script>

<template>
  <main class="home-page">
    <section class="home-hero" aria-labelledby="home-title">
      <div class="home-hero-copy">
        <span class="home-eyebrow">AI 视频创作 · 从灵感到成片</span>
        <h1 id="home-title">把你的故事，<br />变成画面</h1>
        <p>
          从一个想法出发，<br class="home-copy-break" />完成你的第一支视频。
        </p>
        <div class="home-hero-actions">
          <button type="button" class="primary" @click="start()">
            开始创作 <span aria-hidden="true">↗</span>
          </button>
          <button type="button" @click="guideStep = 0">
            查看入门指南 <span aria-hidden="true">→</span>
          </button>
        </div>
        <span class="home-hero-note">写故事 · 做分镜 · 生成视频</span>
      </div>
      <section
        class="home-showcase"
        aria-label="创作灵感轮播"
        aria-roledescription="轮播"
        @mouseenter="hovered = true"
        @mouseleave="hovered = false"
        @focusin="focused = true"
        @focusout="
          focused = !!(
            $event.relatedTarget &&
            ($event.currentTarget as HTMLElement).contains(
              $event.relatedTarget as Node,
            )
          )
        "
      >
        <img
          v-for="(example, index) in examples"
          v-show="index === slide"
          :key="example.title"
          :src="example.image"
          :alt="example.alt"
          :fetchpriority="index === 0 ? 'high' : 'auto'"
        />
        <span class="home-example-label">创意示例 · AI 参考图</span>
        <div class="home-slide-arrows">
          <button
            type="button"
            aria-label="上一个示例"
            @click="select(slide - 1)"
          >
            ←
          </button>
          <button
            type="button"
            aria-label="下一个示例"
            @click="select(slide + 1)"
          >
            →
          </button>
        </div>
        <div
          class="home-slide-caption"
          :aria-live="paused ? 'polite' : 'off'"
          aria-atomic="true"
        >
          <span>{{ current.category }}</span>
          <h2>{{ current.title }}</h2>
          <p>{{ current.subtitle }}</p>
          <button type="button" @click="guideStep = 0">
            看看如何创作 <span aria-hidden="true">↗</span>
          </button>
        </div>
        <div class="home-carousel-controls">
          <span>{{ String(slide + 1).padStart(2, "0") }} / 03</span>
          <div class="home-carousel-dots" aria-label="选择创意示例">
            <button
              v-for="(example, index) in examples"
              :key="example.title"
              type="button"
              :aria-label="`查看示例：${example.title}`"
              :aria-pressed="slide === index"
              @click="select(index)"
            >
              <span />
            </button>
          </div>
          <button
            type="button"
            :aria-label="paused ? '播放轮播' : '暂停轮播'"
            @click="paused = !paused"
          >
            {{ paused ? "播放" : "暂停" }}
          </button>
        </div>
      </section>
    </section>

    <section class="home-onboarding" aria-labelledby="home-steps-title">
      <div class="home-section-heading">
        <div>
          <h2 id="home-steps-title">三步，完成第一支视频</h2>
          <p>不必一次准备好所有内容，跟着步骤慢慢完善。</p>
        </div>
        <span class="home-small-note">从这里开始 ↓</span>
      </div>
      <div class="home-steps">
        <button
          v-for="(step, index) in steps"
          :key="step.title"
          type="button"
          class="home-step"
          :aria-label="`查看第${index + 1}步：${step.title}`"
          @click="guideStep = index"
        >
          <span class="home-step-text"
            ><span class="home-step-number">0{{ index + 1 }}</span
            ><strong>{{ step.title }}</strong
            ><span>{{ step.summary }}</span
            ><span class="home-step-link">查看指南 →</span></span
          >
          <span
            :class="['home-step-art', `home-step-art-${index}`]"
            aria-hidden="true"
          >
            <template v-if="index === 0"
              ><span class="home-paper"
                ><b>故事的开始</b><i /><i /><i /><em
                  >一个雨夜，一次相遇。</em
                ></span
              ></template
            >
            <template v-else-if="index === 1"
              ><img src="/home/cafe.png" alt="" loading="lazy" /><img
                src="/home/city.png"
                alt=""
                loading="lazy"
              /><span>01</span><span>02</span></template
            >
            <template v-else
              ><img src="/home/forest.png" alt="" loading="lazy" /><span
                class="home-film-track"
                ><i /><i /><i /></span
            ></template>
          </span>
        </button>
      </div>
    </section>

    <aside class="home-model-hint" aria-label="创作准备提示">
      <span class="home-hint-icon" aria-hidden="true">✧</span>
      <p role="status">{{ modelHint }}</p>
      <button
        v-if="modelsError"
        type="button"
        :disabled="modelsLoading"
        @click="loadModels"
      >
        重新检查
      </button>
      <RouterLink to="/config"
        >{{
          !modelsLoading && !modelsError && !missing.length
            ? "管理模型"
            : "去设置"
        }}
        <span aria-hidden="true">→</span></RouterLink
      >
    </aside>

    <p class="home-footer-note">让灵感有画面，让故事有声音。</p>
    <HomeGuide
      v-if="guideStep !== null"
      :initial-step="guideStep"
      :example="current"
      @close="guideStep = null"
      @start="start(current.idea)"
    />
  </main>
</template>
