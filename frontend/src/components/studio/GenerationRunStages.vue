<script setup lang="ts">
import {computed} from 'vue';
import type {GenerationRun} from '../../api/bindings';
const props=defineProps<{run:GenerationRun}>();
const stages=computed(()=>{
 const r=props.run;const video=['generating_video','awaiting_video_selection','awaiting_composition','completed'].includes(r.state);
 const speech=r.plan?.filter(p=>p.path==='speech')||[];
 function status(kind:string,fallback:string){const items=r.steps?.filter(t=>t.kind===kind)||[];return items.some(t=>t.status==='failed')?'失败':r.state==='cancelled'?'已取消':items.length&&items.every(t=>t.status==='succeeded')?'待选用':r.state==='failed'?'制作失败':fallback}
 if(r.plan?.[0]?.path==='reference_video')return [{name:'视频',status:['awaiting_composition','completed'].includes(r.state)?'已选用':r.state==='awaiting_video_selection'?'待选用':status('video',r.state==='preparing'?'准备中':'生成中')}];
 return [{name:'分镜图',status:r.selection?.image_take_id||video?'已选用':status('image',r.state==='awaiting_selection'?'待选用':r.state==='preparing'?'准备中':'生成中')},
 {name:'视频',status:['awaiting_composition','completed'].includes(r.state)?'已选用':r.state==='awaiting_video_selection'?'待选用':video?status('video','生成中'):'待开始'},
 {name:'配音',status:!speech.length?'不适用':video?'已选用':status('speech',r.state==='awaiting_selection'?'待选用':'生成中')}];
});
</script>
<template><div class="run-stages"><span v-for="s in stages" :key="s.name">{{s.name}} · {{s.status}}</span></div></template>
