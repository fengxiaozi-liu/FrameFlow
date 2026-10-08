<script setup lang="ts">
import { computed, ref } from 'vue';
import type { EntityVersion, ShotEntityBinding, VoiceProfile } from '../../api/bindings';
import type { Material } from '../../api/types';
import ReferenceImageList from './ReferenceImageList.vue';
import AppIcon from '../AppIcon.vue';
const props=defineProps<{entity:EntityVersion;binding:ShotEntityBinding;materials:Material[];voices:VoiceProfile[];disabled?:boolean;inline?:boolean}>();
const emit=defineEmits<{change:[ShotEntityBinding];edit:[];remove:[];upload:[File[],boolean];replace:[string,File,boolean];voice:[string]}>();
const collapsed=ref(false),saveToLibrary=ref(false);
const selectedIDs=computed(()=>props.binding.reference_material_ids||props.binding.selected_material_ids||props.entity.references.map(r=>r.material_id));
const cover=computed(()=>props.materials.find(m=>m.id===selectedIDs.value[0]));
const voiceName=computed(()=>props.entity.voice_material_id?(props.materials.find(m=>m.id===props.entity.voice_material_id)?.name||'素材不可用，请重新选择'):(props.voices.find(v=>v.id===props.entity.voice_profile_id&&v.revision===props.entity.voice_profile_revision)?.name||'未设置（编辑项目设定可选择）'));
function ids(values:string[]){emit('change',props.inline?{...props.binding,selected_material_ids:undefined,reference_material_ids:values}:{...props.binding,selected_material_ids:values})}
</script>
<template>
 <section class="entity-card" :class="{'entity-card-compact':inline}">
  <template v-if="inline">
   <header class="entity-summary"><img v-if="cover" class="entity-cover" :src="cover.url" :alt="entity.name+'参考图'" /><span v-else class="entity-cover entity-placeholder"><AppIcon :name="entity.kind==='character'?'person':'scene'" /></span><div class="entity-identity"><strong :title="entity.name">{{entity.name}}</strong><span class="entity-meta">{{entity.kind==='location'?'场景':'人物'}} · {{selectedIDs.length}} 张参考图 · v{{entity.revision}}</span></div></header>
   <p class="entity-description" :title="entity.description">{{entity.description||'尚未填写参考描述'}}</p>
   <p v-if="entity.kind==='character'" class="entity-voice" :title="voiceName">默认音色：{{voiceName}}</p>
   <div class="entity-card-actions"><button type="button" :disabled="disabled" @click="emit('edit')">编辑项目设定</button><button class="entity-remove" type="button" :disabled="disabled" @click="emit('remove')">移除分组</button></div>
   <details class="entity-reference-details"><summary>管理参考图 <span>{{selectedIDs.length}} 张</span></summary><div class="entity-reference-controls"><button v-if="binding.selected_material_ids||binding.reference_material_ids" type="button" :disabled="disabled" @click="emit('change',{...binding,selected_material_ids:undefined,reference_material_ids:undefined})">恢复全部参考图</button><label class="reference-library-option"><input v-model="saveToLibrary" type="checkbox" :disabled="disabled" />新上传图片同时保存到素材库</label><ReferenceImageList :ids="selectedIDs" :materials="materials" :disabled="disabled" replaceable @replace="(id,file)=>emit('replace',id,file,saveToLibrary)" @update:ids="ids" @upload="emit('upload',$event,saveToLibrary)" /></div></details>
  </template>
  <template v-else><header><span class="entity-kind">{{entity.kind==='location'?'场景':'人物'}}</span><strong>{{entity.name}}</strong><span class="muted">v{{entity.revision}} · {{selectedIDs.length}} 张选用图</span><button type="button" :disabled="disabled" @click="emit('edit')">编辑项目设定</button><button type="button" :aria-expanded="!collapsed" @click="collapsed=!collapsed">{{collapsed?'展开':'折叠'}}</button><button type="button" :disabled="disabled" @click="emit('remove')">移除分组</button></header><div v-show="!collapsed" class="entity-body"><div><label>项目基础描述<textarea :value="entity.description" readonly aria-label="项目基础描述" /></label><p v-if="entity.kind==='character'" class="muted">默认音色：{{voiceName}}</p></div><div><span>参考图</span><button v-if="binding.selected_material_ids||binding.reference_material_ids" type="button" @click="emit('change',{...binding,selected_material_ids:undefined,reference_material_ids:undefined})">恢复全部参考图</button><ReferenceImageList :ids="selectedIDs" :materials="materials" :disabled="disabled" @update:ids="ids" @upload="emit('upload',$event,false)" /></div></div></template>
 </section>
</template>
