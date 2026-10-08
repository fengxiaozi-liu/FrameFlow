<script setup lang="ts">
import {computed,ref,onBeforeUnmount} from 'vue';
import {onBeforeRouteLeave,useRouter,useRoute} from 'vue-router';
import type {GenerationRequest} from '../api/bindings';
import {useShotWorkspace} from '../composables/useShotWorkspace';
import {useGenerationRuns} from '../composables/useGenerationRuns';
import ShotConfigurationPanel from '../components/studio/ShotConfigurationPanel.vue';
import GenerationRunList from '../components/studio/GenerationRunList.vue';
const {projectId,draftId,sceneId,base,draft,scene,models,materials,error,loading}=useShotWorkspace();
const {runs,submitting,error:runError,submit,pendingSubmission}=useGenerationRuns({project:projectId,draft:draftId,scene:sceneId});
const config=ref<InstanceType<typeof ShotConfigurationPanel>>();
const router=useRouter(),route=useRoute();let alive=true;
onBeforeUnmount(()=>{alive=false});
async function generate(input:Omit<GenerationRequest,'idempotency_key'>){const run=await submit(input);if(run&&alive)await router.push(`${base}/shots/${encodeURIComponent(sceneId)}/runs/${encodeURIComponent(run.id)}`)}
const activeRun=computed(()=>!pendingSubmission.value&&runs.value.some(r=>!['completed','cancelled','failed','awaiting_composition'].includes(r.state)));
onBeforeRouteLeave(async()=>!config.value||await config.value.flush());
</script>
<template><main class="shot-workspace"><nav class="shot-breadcrumb"><RouterLink :to="`${base}/shots`">分镜列表</RouterLink><span>› {{draft?.name}} / 分镜 {{scene?.order}}</span><RouterLink class="shot-breadcrumb-end" :to="`${base}?tab=video`">成片</RouterLink></nav><p v-if="loading">加载中…</p><p v-if="error" role="alert">{{error}}</p><template v-if="draft&&scene"><ShotConfigurationPanel ref="config" :focus-name="route.query.created=== '1'" :project-id="projectId" :draft="draft" :scene="scene" :models="models" :materials="materials" :submitting="submitting" :active-run="activeRun" @saved="draft=$event" @material="materials=materials.some(m=>m.id===$event.id)?materials.map(m=>m.id===$event.id?$event:m):[...materials,$event]" @generate="generate" /><p v-if="runError" role="alert">{{runError}}</p><GenerationRunList :runs="runs" :base="base" :scene-id="sceneId" /></template></main></template>
