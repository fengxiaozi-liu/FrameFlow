<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import type { Draft, Project } from "../api/types";
import { projectDisplayName } from "../utils/projectDisplayName";
const router = useRouter();
async function removeProject(project: Project) {
  const name = projectDisplayName(project);
  if (
    deleting.value ||
    !confirm(
      `删除项目“${name}”？项目及全部 ${project.drafts.length} 个草稿、分镜、设定与项目内版本记录将被删除，无法恢复。素材库内容与已完成任务文件会保留。`,
    )
  )
    return;
  deleting.value = project.id;
  deleteError.value = "";
  notice.value = "";
  try {
    await api.deleteProject(project.id, {
      expected_name: project.name,
      expected_catalog_version: project.catalog_version || 0,
      expected_draft_versions: Object.fromEntries(
        project.drafts.map((draft) => [draft.id, draft.version || 0]),
      ),
    });
    projects.value = projects.value.filter((item) => item.id !== project.id);
    notice.value = `项目“${name}”已删除。`;
  } catch (error) {
    deleteError.value = (error as Error).message;
  } finally {
    deleting.value = "";
  }
}
const projects = ref<Project[]>([]),
  projectsLoading = ref(true),
  projectsError = ref(""),
  showAll = ref(false);
const deleting = ref(""),
  deleteError = ref(""),
  notice = ref("");
async function removeDraft(project: Project, draft: Draft) {
  if (
    deleting.value ||
    !confirm(
      `删除草稿“${draft.name}”？其正文、分镜和草稿内版本记录将被删除，无法恢复。其他草稿、项目素材与已完成任务文件会保留。`,
    )
  )
    return;
  deleting.value = `${project.id}/${draft.id}`;
  deleteError.value = "";
  notice.value = "";
  try {
    await api.deleteDraft(project.id, draft.id, draft.version || 0);
    project.drafts = project.drafts.filter((item) => item.id !== draft.id);
    notice.value = `草稿“${draft.name}”已删除。`;
  } catch (error) {
    deleteError.value = (error as Error).message;
  } finally {
    deleting.value = "";
  }
}
function stamp(draft: Draft) {
  return (
    Date.parse(
      draft.updated_at || draft.story.updated_at || draft.created_at || "",
    ) || 0
  );
}
const recent = computed(() =>
  projects.value
    .map((project) => ({
      project,
      draft: [...(project.drafts || [])].sort((a, b) => stamp(b) - stamp(a))[0],
    }))
    .sort(
      (a, b) =>
        (b.draft
          ? stamp(b.draft)
          : Date.parse(b.project.created_at || "") || 0) -
        (a.draft
          ? stamp(a.draft)
          : Date.parse(a.project.created_at || "") || 0),
    ),
);
const displayed = computed(() =>
  showAll.value ? recent.value : recent.value.slice(0, 4),
);
let alive = true;
async function loadProjects() {
  deleteError.value = "";
  projectsLoading.value = true;
  projectsError.value = "";
  try {
    const data = await api.projects();
    if (alive) projects.value = (data.projects || []).map(project => ({ ...project, drafts: project.drafts || [] }));
  } catch {
    if (alive) projectsError.value = "暂时无法加载草稿，请重试。";
  } finally {
    if (alive) projectsLoading.value = false;
  }
}

function start() {
  sessionStorage.setItem("frameflow-idea", "");
  void router.push("/studio/new");
}
onMounted(() => {
  void loadProjects();
});
onBeforeUnmount(() => {
  alive = false;
});
</script>
<template>
  <main class="page project-workbench">
    <header class="page-intro">
      <div>
        <h1>创作工作台</h1>
        <p>管理你的项目，继续创作或开始新的故事。</p>
      </div>
      <button class="primary" @click="start">＋ 新建创作</button>
    </header>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <div v-if="deleteError" class="notice error" role="alert">
      {{ deleteError }}
      <button type="button" @click="loadProjects">刷新项目列表</button>
    </div>
    <section
      v-if="projectsLoading || projectsError || recent.length"
      class="workbench-projects"
      aria-labelledby="workbench-projects-title"
    >
      <div class="workbench-section-heading">
        <div>
          <h2 id="workbench-projects-title">继续创作</h2>
          <p>接着上次的灵感，继续向前。</p>
        </div>
        <button
          v-if="recent.length > 4"
          type="button"
          class="quiet"
          :aria-expanded="showAll"
          @click="showAll = !showAll"
        >
          {{ showAll ? "收起项目" : `全部项目（${recent.length}）` }}
        </button>
      </div>
      <p v-if="projectsLoading" role="status">正在加载项目…</p>
      <div v-else-if="projectsError" class="workbench-load-error">
        <p role="alert">{{ projectsError }}</p>
        <button type="button" @click="loadProjects">重新加载草稿</button>
      </div>
      <div v-else class="workbench-project-grid">
        <article
          v-for="({ project, draft }, index) in displayed"
          :key="project.id"
          class="workbench-project-item"
        >
          <RouterLink
            v-if="draft"
            class="workbench-project-card"
            :to="`/studio/${encodeURIComponent(project.id)}/${encodeURIComponent(draft.id)}`"
          >
            <span class="workbench-project-cover" aria-hidden="true"
              ><span>项目</span
              ><b>{{ String(index + 1).padStart(2, "0") }}</b></span
            >
            <span class="workbench-project-info"
              ><strong>{{ projectDisplayName(project) }}</strong
              ><span v-if="project.drafts.length > 1"
                >当前草稿：{{ draft.name }}</span
              ><small
                >{{ project.drafts.length }} 个草稿 ·
                {{ draft.story.scenes?.length || 0 }} 个分镜</small
              ></span
            >
            <span class="workbench-project-continue"
              >继续编辑 <span aria-hidden="true">→</span></span
            >
          </RouterLink>
          <div v-else class="workbench-project-card">
            <span class="workbench-project-info"
              ><strong>{{ projectDisplayName(project) }}</strong
              ><small>暂无草稿</small></span
            >
          </div>
          <div class="workbench-draft-actions">
            <button
              class="quiet delete-action"
              :disabled="!!deleting"
              :aria-label="`删除项目 ${projectDisplayName(project)}`"
              @click="removeProject(project)"
            >
              {{ deleting === project.id ? "删除中…" : "删除项目" }}
            </button>
          </div>
          <details
            v-if="project.drafts.length > 1"
            class="workbench-other-drafts"
          >
            <summary>其他草稿（{{ project.drafts.length - 1 }}）</summary>
            <div
              v-for="item in project.drafts.filter(
                (item) => item.id !== draft.id,
              )"
              :key="item.id"
              class="workbench-draft-row"
            >
              <RouterLink
                :to="`/studio/${encodeURIComponent(project.id)}/${encodeURIComponent(item.id)}`"
                >{{ item.name }}</RouterLink
              >
              <button
                class="quiet delete-action"
                :disabled="!!deleting"
                :aria-label="`删除草稿 ${item.name}`"
                @click="removeDraft(project, item)"
              >
                删除
              </button>
            </div>
          </details>
        </article>
      </div>
    </section>

    <section
      v-if="!projectsLoading && !projectsError && !recent.length"
      class="workbench-empty"
    >
      <h2>开始你的第一个故事</h2>
      <p>还没有创作草稿，写下想法或导入文本后即可继续制作分镜。</p>
      <button class="primary" @click="start">开始创作</button>
    </section>
  </main>
</template>
<style scoped>
.workbench-project-item {
  min-width: 0;
  border: 1px solid var(--line);
  border-radius: 9px;
  background: white;
  overflow: hidden;
}
.workbench-project-item .workbench-project-card {
  border: 0;
  border-radius: 0;
}
.workbench-draft-actions {
  display: flex;
  justify-content: flex-end;
  padding: 0 12px 8px;
}
.workbench-project-item .delete-action {
  color: var(--danger);
  font-size: 12px;
  padding: 4px 8px;
}
.workbench-other-drafts {
  margin: 0 12px 12px;
  border-top: 1px solid var(--line);
  padding-top: 8px;
}
.workbench-other-drafts summary {
  cursor: pointer;
  font-size: 12px;
  color: var(--muted);
}
.workbench-draft-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 8px;
  font-size: 12px;
}
.workbench-draft-row a {
  min-width: 0;
  overflow-wrap: anywhere;
}
.workbench-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.workbench-section-heading h2 {
  margin: 0;
}
.workbench-section-heading p {
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 13px;
}
.workbench-project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 420px), 1fr));
  gap: 14px;
}
.workbench-project-card {
  display: flex;
  gap: 14px;
  align-items: center;
  text-decoration: none;
  padding: 12px;
  background: white;
  border: 1px solid var(--line);
  border-radius: 9px;
  min-width: 0;
}
.workbench-project-card:hover {
  border-color: var(--accent);
}
.workbench-project-cover {
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 6px;
  color: var(--accent);
  background: var(--tint);
}
.workbench-project-cover > span {
  font-size: 10px;
}
.workbench-project-cover b {
  font-size: 20px;
  font-weight: 400;
}
.workbench-project-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 4px;
  flex: 1;
}
.workbench-project-info strong,
.workbench-project-info > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.workbench-project-info strong {
  font-size: 14px;
}
.workbench-project-info > span,
.workbench-project-info small {
  font-size: 12px;
  color: var(--muted);
}
.workbench-project-continue {
  color: var(--accent);
  font-size: 12px;
  white-space: nowrap;
}
.workbench-empty,
.workbench-load-error {
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: 9px;
  background: white;
}
.workbench-empty h2 {
  margin: 0 0 8px;
}
.workbench-empty p {
  color: var(--muted);
  margin: 0 0 18px;
}
@media (max-width: 600px) {
  .workbench-project-card {
    gap: 10px;
    flex-wrap: wrap;
  }
  .workbench-project-continue {
    margin-left: auto;
  }
}
</style>
