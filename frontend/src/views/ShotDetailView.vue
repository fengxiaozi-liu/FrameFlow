<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { onBeforeRouteLeave, useRoute } from "vue-router";
import type { GenerationRequest } from "../api/bindings";
import { useShotWorkspace } from "../composables/useShotWorkspace";
import { useGenerationRuns } from "../composables/useGenerationRuns";
import ShotConfigurationPanel from "../components/studio/ShotConfigurationPanel.vue";
import GenerationRunList from "../components/studio/GenerationRunList.vue";
const {
  projectId,
  draftId,
  sceneId,
  base,
  draft,
  scene,
  models,
  materials,
  error,
  loading,
  reload,
} = useShotWorkspace();
const {
  runs,
  submitting,
  error: runError,
  submit,
  remove,
  deleting,
} = useGenerationRuns({ project: projectId, draft: draftId, scene: sceneId });
const config = ref<InstanceType<typeof ShotConfigurationPanel>>();
const route = useRoute();
const records = ref<InstanceType<typeof GenerationRunList>>();
watch(loading, async (value) => {
  if (!value && route.query.deleted === "1") {
    await nextTick();
    await records.value?.focusHeading();
  }
});
async function removeRun(run: Parameters<typeof remove>[0]) {
  if (config.value && !(await config.value.flush())) return;
  if (await remove(run)) {
    await reload();
    await records.value?.focusHeading();
  }
}
async function generate(input: Omit<GenerationRequest, "idempotency_key">) {
  await submit(input);
}
onBeforeRouteLeave(async () => !config.value || (await config.value.flush()));
</script>
<template>
  <main class="shot-workspace">
    <nav class="shot-breadcrumb">
      <RouterLink :to="`${base}/shots`">分镜列表</RouterLink
      ><span>› {{ draft?.name }} / 分镜 {{ scene?.order }}</span
      ><RouterLink class="shot-breadcrumb-end" :to="`${base}?tab=video`"
        >成片</RouterLink
      >
    </nav>
    <p v-if="loading">加载中…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="draft && scene"
      ><ShotConfigurationPanel
        ref="config"
        :focus-name="route.query.created === '1'"
        :project-id="projectId"
        :draft="draft"
        :scene="scene"
        :models="models"
        :materials="materials"
        :submitting="submitting"
        @saved="draft = $event"
        @material="
          materials = materials.some((m) => m.id === $event.id)
            ? materials.map((m) => (m.id === $event.id ? $event : m))
            : [...materials, $event]
        "
        @generate="generate" />
      <p v-if="runError" role="alert">{{ runError }}</p>
      <GenerationRunList
        ref="records"
        :runs="runs"
        :base="base"
        :scene-id="sceneId"
        :deleting="deleting"
        @remove="removeRun"
    /></template>
  </main>
</template>
