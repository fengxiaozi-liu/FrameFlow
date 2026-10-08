<script setup lang="ts">
import type {GenerationRun} from '../../api/bindings';
import GenerationRunStages from './GenerationRunStages.vue';
defineProps<{runs:GenerationRun[];base:string;sceneId:string}>();
const labels:Record<string,string>={preparing:'准备中',generating:'生成中',awaiting_selection:'待选用首帧与配音',generating_video:'视频生成中',awaiting_video_selection:'待选用视频',awaiting_composition:'视频已选用，待成片合成',completed:'成片已完成',failed:'失败',cancelled:'已取消'};
</script>
<template><section class="shot-task-structure" aria-label="任务结构"><h2>任务结构</h2><p v-if="!runs.length" class="empty">暂无制作任务，完成配置后点击“生成视频”。</p><article v-for="(r,i) in runs" :key="r.id" class="generation-run-row"><div><strong>任务 {{String(i+1).padStart(3,'0')}}</strong><GenerationRunStages :run="r" /><p>{{labels[r.state]||r.state}}</p><small>{{r.created_at?new Date(r.created_at).toLocaleString():'历史记录'}} · {{r.id}}</small></div><RouterLink :to="`${base}/shots/${encodeURIComponent(sceneId)}/runs/${encodeURIComponent(r.id)}`">进入任务 →</RouterLink></article></section></template>
