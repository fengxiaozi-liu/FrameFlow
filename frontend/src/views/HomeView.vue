<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { useTaskStore } from "../stores/tasks";
import type { Overview, Project, Model } from "../api/types";
import StatusBadge from "../components/StatusBadge.vue";
import ProgressBar from "../components/ProgressBar.vue";
const router = useRouter(),
  tasks = useTaskStore(),
  idea = ref(""),
  overview = ref<Overview>({
    projects: 0,
    tasks: 0,
    running_tasks: 0,
    providers: 0,
    materials: 0,
  }),
  projects = ref<Project[]>([]),
  providers = ref<Record<string, Model[]>>({
    story: [],
    image: [],
    video: [],
  });
onMounted(async () => {
  await Promise.all([
    tasks.load(),
    api.overview().then((v) => (overview.value = v)),
    api.projects().then((v) => (projects.value = v.projects)),
    ...(["story", "image", "video"] as const).map((k) =>
      api
        .models(k)
        .then(
          (v) =>
            (providers.value[k] = v.models.filter(
              (m) => m.enabled && m.supported,
            )),
        ),
    ),
  ]);
});
function start() {
  sessionStorage.setItem("frameflow-idea", idea.value);
  router.push("/studio");
}
</script>
<template>
  <main class="page">
    <div class="page-intro">
      <div>
        <span class="eyebrow">创作概览</span>
        <h1>开始下一段视频创作</h1>
        <p>从一个想法开始，或接着完成已有草稿。</p>
      </div>
    </div>
    <div class="home-grid">
      <section class="card start-card">
        <span class="eyebrow">新建创作</span>
        <h2>你想讲一个怎样的故事？</h2>
        <p>描述主题、受众或画面，进入工作台完善脚本与分镜。</p>
        <label
          >视频想法<textarea
            v-model="idea"
            placeholder="例如：用 30 秒介绍城市的一天，从清晨街道到夜晚灯火。"
          />
        </label>
        <div class="actions">
          <small>可以先写脚本，缺少生图接口不会阻塞编辑。</small
          ><button class="primary" @click="start">开始创作</button>
        </div>
      </section>
      <aside class="card">
        <h2>AI 接口准备情况</h2>
        <p>各步骤独立选择，按需完成配置。</p>
        <div
          v-for="(label, key) in {
            story: '故事与分镜',
            image: '角色与首尾帧',
            video: '视频生成',
          }"
          :key="key"
          class="status-row"
        >
          <div>
            <strong>{{ label }}</strong
            ><small>{{
              providers[key]?.[0]?.name || "尚未配置可用接口"
            }}</small>
          </div>
          <StatusBadge
            :status="
              providers[key]?.some((v) => v.enabled && v.supported)
                ? 'healthy'
                : 'disabled'
            "
          />
        </div>
        <RouterLink class="text-link" to="/config">管理接口配置</RouterLink>
      </aside>
    </div>
    <section>
      <div class="section-head">
        <h2>继续草稿</h2>
        <span>{{ overview.projects }} 个项目</span>
      </div>
      <div class="cards">
        <article
          v-for="project in projects"
          :key="project.id"
          class="card draft-card"
        >
          <div>
            <h3>{{ project.name }}</h3>
            <p>{{ project.drafts?.length || 0 }} 个草稿</p>
          </div>
          <RouterLink
            v-if="project.drafts?.[0]"
            class="text-link"
            :to="'/studio/' + project.id + '/' + project.drafts[0].id"
            >继续编辑</RouterLink
          >
          <span v-else class="muted">暂无可编辑草稿</span>
        </article>
        <div v-if="!projects.length" class="empty">
          暂无草稿，从上方输入想法开始创作。
        </div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>后台任务</h2>
        <RouterLink class="text-link" to="/tasks">进入任务中心</RouterLink>
      </div>
      <div class="card">
        <article
          v-for="task in tasks.items.slice(0, 3)"
          :key="task.id"
          class="task-row"
        >
          <div>
            <strong>{{ task.kind }} · {{ task.id.slice(-8) }}</strong
            ><small
              >{{ task.stage }} ·
              {{ task.provider_code || "未指定接口" }}</small
            >
          </div>
          <div class="task-progress">
            <div>
              <StatusBadge :status="task.status" /><b>{{ task.progress }}%</b>
            </div>
            <ProgressBar :value="task.progress" />
          </div>
        </article>
        <div v-if="!tasks.items.length" class="empty">暂无后台任务。</div>
      </div>
    </section>
  </main>
</template>
