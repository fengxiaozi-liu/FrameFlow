<script setup lang="ts">
import {computed,ref} from 'vue';
import type {EntityVersion} from '../../api/bindings';
import type {Material} from '../../api/types';
import {api} from '../../api/client';

const props=defineProps<{entity:EntityVersion;materials:Material[];disabled?:boolean}>();
const selection=ref(props.entity.voice_material_id||'');
const voices=computed(()=>(props.materials||[]).filter(m=>m.kind==='voice'));
const selected=computed(()=>voices.value.find(m=>m.id===selection.value));
function selectedMaterialID():string|undefined{
 if(selection.value&&!selected.value)throw Error('所选音色素材不可用，请重新选择或清除。');
 return selection.value||undefined;
}
defineExpose({selectedMaterialID});
</script>

<template><fieldset class="character-voice-settings" :disabled="disabled"><legend>人物音色</legend>
 <label>从素材库选择<select v-model="selection" aria-label="人物音色素材"><option value="">暂不设置</option><option v-for="voice in voices" :key="voice.id" :value="voice.id">{{voice.name}}</option><option v-if="selection&&!selected" :value="selection" disabled>原音色素材不可用，请重新选择</option></select></label>
 <p v-if="entity.voice_profile_id" class="muted">此人物原有模型音色，请重新选择素材；保存后将替换原音色设定。</p>
 <p v-if="!voices.length" class="muted">素材库中暂无配音素材，请先在素材管理中上传配音音频，再回来选择。</p>
 <div v-if="selected"><p>{{selected.name}}</p><audio :key="selected.id" :src="api.mediaUrl(selected.url)" controls preload="none" aria-label="试听人物音色" style="width:100%" /><button type="button" @click="selection=''">清除音色</button></div>
 <p class="muted">选择素材库中的配音音频，作为人物音色参考。</p>
 <p class="voice-scope-note">当前视频生成暂不使用音色参考音频。</p>
</fieldset></template>
