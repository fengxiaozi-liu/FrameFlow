<script setup lang="ts">
import { ref } from 'vue';
import type { EntityVersion, ShotEntityBinding, VoiceProfile } from '../../api/bindings';
import type { Material } from '../../api/types';
import ReferenceImageList from './ReferenceImageList.vue';
const props=defineProps<{entity:EntityVersion;binding:ShotEntityBinding;materials:Material[];voices:VoiceProfile[];disabled?:boolean}>();
const emit=defineEmits<{change:[ShotEntityBinding];edit:[];remove:[];upload:[File[]];voice:[string]}>();
const collapsed=ref(false);
function ids(values:string[]){emit('change',{...props.binding,selected_material_ids:values})}
</script>
<template><section class="entity-card"><header><span class="entity-kind">{{entity.kind==='location'?'场景':'人物'}}</span><strong>{{entity.name}}</strong><span class="muted">v{{entity.revision}} · {{(binding.selected_material_ids||entity.references.map(r=>r.material_id)).length}} 张选用图</span><button type="button" :disabled="disabled" @click="emit('edit')">编辑项目设定</button><button type="button" :aria-expanded="!collapsed" @click="collapsed=!collapsed">{{collapsed?'展开':'折叠'}}</button><button type="button" :disabled="disabled" @click="emit('remove')">移除分组</button></header><div v-show="!collapsed" class="entity-body"><div><label>项目基础描述<textarea :value="entity.description" readonly aria-label="项目基础描述" /></label><label>本镜头补充<textarea :value="binding.description_supplement||''" :disabled="disabled" placeholder="仅影响当前镜头，例如夜晚、湿发或手持道具" @input="emit('change',{...binding,description_supplement:($event.target as HTMLTextAreaElement).value})" /></label><p v-if="entity.kind==='character'" class="muted">默认音色：{{voices.find(v=>v.id===entity.voice_profile_id&&v.revision===entity.voice_profile_revision)?.name||'未设置（编辑项目设定可配置）'}}</p></div><div><span>参考图</span><button v-if="binding.selected_material_ids" type="button" @click="emit('change',{...binding,selected_material_ids:undefined})">恢复全部参考图</button><ReferenceImageList :ids="binding.selected_material_ids||entity.references.map(r=>r.material_id)" :materials="materials" :disabled="disabled" @update:ids="ids" @upload="emit('upload',$event)" /></div></div></section></template>
