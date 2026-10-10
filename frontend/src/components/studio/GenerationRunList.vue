<script setup lang="ts">
import type {GenerationRun} from '../../api/bindings';
import {api} from '../../api/client';
import ShotPreview from './ShotPreview.vue';
import GenerationRunStages from './GenerationRunStages.vue';
defineProps<{runs:GenerationRun[];base:string;sceneId:string}>();
const labels:Record<string,string>={preparing:'准备中',generating:'生成中',awaiting_selection:'待选用首帧与配音',generating_video:'视频生成中',awaiting_video_selection:'待选用视频',awaiting_composition:'视频已选用，待成片合成',completed:'成片已完成',failed:'失败',cancelled:'已取消'};
const stepLabels:Record<string,string>={queued:'排队中',running:'生成中',succeeded:'生成完成',failed:'生成失败',cancelled:'已取消'};
</script>
<template>
  <section class="shot-task-structure" aria-label="任务结构">
    <h2>生成记录</h2>
    <p class="muted">提交后在这里查看进度，可继续调整配置并生成新视频。</p>
    <p v-if="!runs.length" class="empty">暂无制作任务，完成配置后点击“生成视频”。</p>
    <article v-for="(r,i) in runs" :key="r.id" class="generation-run-row">
      <div class="generation-run-content">
        <strong>任务 {{String(i+1).padStart(3,'0')}}</strong>
        <GenerationRunStages :run="r" />
        <p role="status">{{labels[r.state]||r.state}}</p>
        <small>{{r.created_at?new Date(r.created_at).toLocaleString():'历史记录'}}</small>
        <p v-if="r.error" role="alert">{{r.error}}</p>
        <div v-for="step in r.steps?.filter(s=>s.kind==='video')" :key="step.id" class="inline-video-result">
          <div v-if="step.status==='queued'||step.status==='running'" class="run-video-progress">
            <strong>{{stepLabels[step.status]}} · {{step.progress||0}}%</strong>
            <progress :value="step.progress||0" max="100" :aria-label="`任务 ${i+1} 视频生成进度`" />
          </div>
          <p v-if="step.error && step.error!==r.error" role="alert">{{step.error}}</p>
          <ShotPreview v-if="step.status==='succeeded' && step.result_url" :title="`任务 ${i+1} 生成视频`" :url="api.mediaUrl(step.result_url)" />
        </div>
      </div>
      <RouterLink :to="`${base}/shots/${encodeURIComponent(sceneId)}/runs/${encodeURIComponent(r.id)}`">进入任务 →</RouterLink>
    </article>
  </section>
</template>

<style scoped>
.generation-run-content { flex: 1; min-width: 0; }
.inline-video-result { max-width: 640px; margin-top: 12px; }
.generation-run-row > a { flex-shrink: 0; }
</style>
