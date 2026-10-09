<script setup lang="ts">
import { computed, watch } from 'vue';
import type { ShotGenerationSettings } from '../../api/bindings';
import type { Model } from '../../api/types';
const props = defineProps<{ modelValue: ShotGenerationSettings; models: Model[]; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [ShotGenerationSettings]; ready: [boolean] }>();
const videos = computed(() => props.models.filter(m => m.capabilities.includes('video')));
const status = computed(() => props.models.find(m => m.id === props.modelValue.video_model_id)?.availability?.video);
const binding = computed(() => status.value?.available ? status.value.binding : undefined);
const durations = computed(() => binding.value?.durations || []);
const resolutions = computed(() => binding.value?.resolutions || []);
const ratios = computed(() => binding.value?.ratios || []);
const ready = computed(() => !!binding.value && durations.value.includes(props.modelValue.duration_seconds) && resolutions.value.includes(props.modelValue.resolution) && ratios.value.includes(props.modelValue.aspect_ratio));
watch(ready, value => emit('ready', value), { immediate: true });
function update<K extends keyof ShotGenerationSettings>(key: K, value: ShotGenerationSettings[K]) {
 emit('update:modelValue', { ...props.modelValue, image_model_id: undefined, [key]: value });
}
function chooseModel(event: Event) {
 const id = (event.target as HTMLSelectElement).value;
 const config = props.models.find(m => m.id === id)?.availability?.video?.binding;
 const old = props.modelValue;
 emit('update:modelValue', {
  ...old, image_model_id: undefined, video_model_id: id,
  aspect_ratio: config?.ratios?.includes(old.aspect_ratio) ? old.aspect_ratio : config?.ratios?.[0] || '',
  duration_seconds: config?.durations?.includes(old.duration_seconds) ? old.duration_seconds : config?.durations?.[0] || 0,
  resolution: config?.resolutions?.includes(old.resolution) ? old.resolution : config?.resolutions?.[0] || '',
 });
}
</script>
<template>
 <section class="shot-config-section" aria-label="视频设置">
  <header class="shot-section-heading"><h3>视频设置</h3>
   <p v-if="binding?.input_mode === 'first_frame'">使用分镜中的一张引用图片作为首帧；请只保留一张参考图片。</p>
   <p v-else-if="binding">直接使用引用素材生成视频，最多 {{ binding.max_references }} 张参考图片。</p>
   <p v-else>选择已配置协议与参数的视频模型。</p>
  </header>
  <div class="shot-settings-grid shot-video-settings">
   <label>视频模型<select aria-label="视频模型" :value="modelValue.video_model_id" :disabled="disabled" @change="chooseModel">
    <option value="">请选择视频模型</option>
    <option v-for="m in videos" :key="m.id" :value="m.id" :disabled="!m.availability?.video?.available">{{ m.name }}{{ m.availability?.video?.available ? '' : ' · ' + (m.availability?.video?.reason || '能力尚未读取') }}</option>
   </select></label>
   <label>画面比例<select aria-label="画面比例" :value="modelValue.aspect_ratio" :disabled="disabled || !binding" @change="update('aspect_ratio', ($event.target as HTMLSelectElement).value)">
    <option v-if="!ratios.includes(modelValue.aspect_ratio)" :value="modelValue.aspect_ratio" disabled>请选择支持的比例</option><option v-for="ratio in ratios" :key="ratio">{{ ratio }}</option>
   </select></label>
   <label>清晰度<select aria-label="清晰度" :value="modelValue.resolution" :disabled="disabled || !binding" @change="update('resolution', ($event.target as HTMLSelectElement).value)">
    <option v-if="!resolutions.includes(modelValue.resolution)" :value="modelValue.resolution" disabled>请选择支持的清晰度</option><option v-for="r in resolutions" :key="r">{{ r }}</option>
   </select></label>
   <label>时长<select aria-label="时长" :value="modelValue.duration_seconds" :disabled="disabled || !binding" @change="update('duration_seconds', Number(($event.target as HTMLSelectElement).value))">
    <option v-if="!durations.includes(modelValue.duration_seconds)" :value="modelValue.duration_seconds" disabled>请选择支持的时长</option><option v-for="d in durations" :key="d" :value="d">{{ d }} 秒</option>
   </select></label>
  </div>
  <p v-if="!binding" class="muted">{{ status?.reason || '暂无已选的可用模型。' }} <RouterLink to="/config">前往模型管理配置协议与参数</RouterLink></p>
  <p v-else-if="!ready" role="alert">当前参数不在模型配置范围内，请重新选择。</p>
 </section>
</template>
