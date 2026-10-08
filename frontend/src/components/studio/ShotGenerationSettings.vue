<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {bindingsApi,type ShotGenerationSettings} from '../../api/bindings';
import type {Model} from '../../api/types';
const props=defineProps<{modelValue:ShotGenerationSettings;models:Model[];disabled?:boolean}>();
const emit=defineEmits<{'update:modelValue':[ShotGenerationSettings];ready:[boolean]}>();
type Capability=Awaited<ReturnType<typeof bindingsApi.capability>>;
const capabilities=ref<Record<string,Capability>>({}),error=ref(''),loading=ref(false);let requestID=0;
const candidates=computed(()=>props.models.filter(m=>m.enabled&&m.supported&&m.capabilities.includes('video')));
const videos=computed(()=>candidates.value.filter(m=>capabilities.value[m.id]?.reference_video));
const selected=computed(()=>capabilities.value[props.modelValue.video_model_id]);
const durations=computed(()=>selected.value?.durations||[]),resolutions=computed(()=>selected.value?.resolutions||[]);
watch(()=>candidates.value.map(m=>m.id).join(','),async()=>{const at=++requestID;loading.value=true;error.value='';const result=await Promise.allSettled(candidates.value.map(async m=>({id:m.id,cap:await bindingsApi.capability(m.id)})));if(at!==requestID)return;const next:Record<string,Capability>={};for(const entry of result){if(entry.status==='fulfilled')next[entry.value.id]=entry.value.cap;else error.value='部分模型能力读取失败，请刷新后重试。'}capabilities.value=next;loading.value=false},{immediate:true});
watch(()=>!!selected.value?.reference_video&&durations.value.includes(props.modelValue.duration_seconds)&&resolutions.value.includes(props.modelValue.resolution),value=>emit('ready',value),{immediate:true});
function update<K extends keyof ShotGenerationSettings>(key:K,value:ShotGenerationSettings[K]){emit('update:modelValue',{...props.modelValue,image_model_id:undefined,[key]:value})}
</script>
<template><section class="shot-config-section" aria-label="视频设置"><header class="shot-section-heading"><h3>视频设置</h3><p>直接使用引用素材生成视频，最多支持 {{selected?.reference_images||5}} 张参考图片。</p></header><div class="shot-settings-grid shot-video-settings">
<label>视频模型<select :value="modelValue.video_model_id" :disabled="disabled||loading" @change="update('video_model_id',($event.target as HTMLSelectElement).value)"><option value="">请选择参考生视频模型</option><option v-if="modelValue.video_model_id&&!videos.some(m=>m.id===modelValue.video_model_id)" :value="modelValue.video_model_id" disabled>原模型不支持直接引用，请重新选择</option><option v-for="m in videos" :key="m.id" :value="m.id">{{m.name}}</option></select></label>
<label>画面比例<select :value="modelValue.aspect_ratio" :disabled="disabled" @change="update('aspect_ratio',($event.target as HTMLSelectElement).value)"><option>16:9</option><option>9:16</option><option>1:1</option></select></label>
<label>清晰度<select :value="modelValue.resolution" :disabled="disabled" @change="update('resolution',($event.target as HTMLSelectElement).value)"><option v-if="!resolutions.includes(modelValue.resolution)" :value="modelValue.resolution">{{modelValue.resolution}}（请核对模型支持）</option><option v-for="r in resolutions" :key="r">{{r}}</option></select></label>
<label>时长<select :value="modelValue.duration_seconds" :disabled="disabled" @change="update('duration_seconds',Number(($event.target as HTMLSelectElement).value))"><option v-if="!durations.includes(modelValue.duration_seconds)" :value="modelValue.duration_seconds">{{modelValue.duration_seconds}} 秒（请核对模型支持）</option><option v-for="d in durations" :key="d" :value="d">{{d}} 秒</option></select></label>
</div><p v-if="!loading&&!videos.length" class="muted">请在<RouterLink to="/config">模型配置</RouterLink>中添加万相参考生视频模型（wan2.7-r2v）。</p><p v-if="selected?.reference_video&&(!durations.includes(modelValue.duration_seconds)||!resolutions.includes(modelValue.resolution))" role="alert">请选择此模型支持的时长与清晰度。</p><p v-if="error" role="alert">{{error}}</p></section></template>
