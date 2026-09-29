<script setup lang="ts">
import {ref,watch} from 'vue';
import type {Candidate} from '../../api/types';
const props=defineProps<{candidate:Candidate}>();const emit=defineEmits<{change:[NonNullable<Candidate['entities']>]}>();
const entities=ref<NonNullable<Candidate['entities']>>([]);watch(()=>props.candidate.id,()=>{entities.value=JSON.parse(JSON.stringify(props.candidate.entities||[]));emit('change',entities.value)},{immediate:true});
</script>
<template><section aria-label="AI 场景人物候选"><p>应用前核对场景与人物。相同名字以编号区分；可修改描述消除歧义，应用后再添加参考图及音色。</p><div v-for="e in entities" :key="e.id" class="candidate-entity"><strong>{{e.kind==='location'?'场景':'人物'}} · {{e.key}} · {{e.id.slice(-6)}}</strong><label>候选名称<input v-model="e.name" @input="emit('change',entities)" /></label><label>候选描述<textarea v-model="e.description" @input="emit('change',entities)" /></label><p v-if="!e.description.trim()||e.description.includes('待确认')" class="notice">此实体还需要确认，请补充具体描述。</p></div><details><summary>镜头关联与台词</summary><div v-for="s in candidate.scenes" :key="s.order"><strong>{{s.order}}. {{s.title}}</strong><p>出现：{{s.entity_bindings?.map(b=>entities.find(e=>e.id===b.entity_id)?.name||'未归组').join('、')||'未归组，可在应用后添加档案'}}</p><p v-for="u in s.utterances" :key="u.id">{{u.speaker_type==='narrator'?'旁白':entities.find(e=>e.id===u.speaker_id)?.name}}：{{u.text}}</p></div></details></section></template>
