<script setup lang="ts">
import { defaultEntityName } from "./entityDefaults";
import {computed,onMounted,ref} from 'vue';
import {api} from '../../api/client';
import {bindingsApi,normalizeEntityCatalog,type EntityCatalog,type EntityVersion,type ShotEntityBinding,type GenerationRequest,type ShotGenerationSettings as GenerationSettings} from '../../api/bindings';
import type {Draft,Scene,Material,Model} from '../../api/types';
import {useShotEditor} from '../../composables/useShotEditor';
import EntityReferenceCard from './EntityReferenceCard.vue';
import MentionPromptEditor from './MentionPromptEditor.vue';
import DialogueVoicePanel from './DialogueVoicePanel.vue';
import ReferenceImageList from './ReferenceImageList.vue';
import ReferenceImageSource from './ReferenceImageSource.vue';
import ShotGenerationSettings from './ShotGenerationSettings.vue';
import EntityEditorDrawer from './EntityEditorDrawer.vue';
import CharacterVoiceSettings from './CharacterVoiceSettings.vue';
const props=defineProps<{projectId:string;draft:Draft;scene:Scene;materials:Material[];models:Model[];submitting?:boolean;focusName?:boolean}>();
const emit=defineEmits<{saved:[Draft];material:[Material];generate:[Omit<GenerationRequest,'idempotency_key'>]}>();
const editor=useShotEditor(()=>({project:props.projectId,draft:props.draft,scene:props.scene}),d=>emit('saved',d));
const {state,dirty,saving,error,conflict,supplementsMerged}=editor;
const nameInput=ref<HTMLInputElement>();
const voiceEditor=ref<InstanceType<typeof CharacterVoiceSettings>>();
const referenceEditor=ref<InstanceType<typeof ReferenceImageSource>>();
const catalog=ref<EntityCatalog>({catalog_version:0,entities:[],voice_profiles:[]}),notice=ref(''),busy=ref(false);
const form=ref<EntityVersion>(),formBinding=ref(''),isNew=ref(false),library=ref(false);
const bindingPending=ref(false),sourceBusy=ref(false);
const videoReady=ref(false);
const settings=computed<GenerationSettings>(()=>state.value.generation_settings||{image_model_id:'',video_model_id:'',aspect_ratio:'16:9',resolution:'720P',duration_seconds:props.scene.duration_seconds||4});
const latest=computed(()=>{const map=new Map<string,EntityVersion>();for(const e of catalog.value.entities||[]){if(!map.has(e.id)||map.get(e.id)!.revision<e.revision)map.set(e.id,e)}return [...map.values()].filter(e=>!e.archived)});
const groups=computed(()=>state.value.entity_bindings.map(b=>({binding:b,entity:catalog.value.entities.find(e=>e.id===b.entity_id&&e.revision===b.entity_revision)})));
const missingGroups=computed(()=>groups.value.filter(g=>!g.entity));
const mentionOptions=computed(()=>groups.value.filter(g=>g.entity).map(g=>({id:g.binding.id,name:g.entity!.name,kind:g.entity!.kind})));
const voices=computed(()=>catalog.value.voice_profiles||[]);
const mappedMaterials=computed(()=>props.materials.map(m=>({...m,url:api.mediaUrl(m.url)})));
const videoImages = computed(() => {
 const ids = new Set(groups.value.flatMap(({binding, entity}) => binding.reference_material_ids || binding.selected_material_ids || entity?.references.map(ref => ref.material_id) || []));
 return mappedMaterials.value.filter(material => ids.has(material.id));
});
const copy=<T,>(v:T):T=>JSON.parse(JSON.stringify(v));
function changeBinding(b:ShotEntityBinding){editor.change({entity_bindings:state.value.entity_bindings.map(x=>x.id===b.id?b:x)})}
function remove(id:string){editor.change({entity_bindings:state.value.entity_bindings.filter(b=>b.id!==id),prompt_document:{schema_version:1,nodes:state.value.prompt_document.nodes.filter(n=>n.type!=='entity_ref'||n.binding_id!==id)}})}
function bind(e:EntityVersion){if(state.value.entity_bindings.some(b=>b.entity_id===e.id)){notice.value='此档案已在当前镜头中。';return}editor.change({entity_bindings:[...state.value.entity_bindings,{id:crypto.randomUUID(),entity_id:e.id,entity_revision:e.revision,role:e.kind==='character'?'cast':state.value.entity_bindings.some(b=>b.role==='primary')?'supplement':'primary'}]});library.value=false}
function create(kind: 'location' | 'character') {
  form.value = {
    id: crypto.randomUUID(),
    kind,
    name: defaultEntityName(kind, latest.value),
    description: '',
    revision: 1,
    references: [],
  };
  formBinding.value = '';
  isNew.value = true;
  notice.value = '';
  bindingPending.value = false;
}
function edit(e:EntityVersion,id:string){form.value=copy(e);const b=state.value.entity_bindings.find(b=>b.id===id);const ids=b?.reference_material_ids||b?.selected_material_ids;if(ids)form.value.references=ids.map((material_id,order)=>({material_id,order,label:e.references.find(r=>r.material_id===material_id)?.label||props.materials.find(m=>m.id===material_id)?.name||""}));formBinding.value=id;isNew.value=false;notice.value='';bindingPending.value=false}
function closeForm(){if(!busy.value&&!bindingPending.value&&!sourceBusy.value){form.value=undefined;notice.value=''}}
async function saveEntity(){if(busy.value||sourceBusy.value)return;if(referenceEditor.value?.hasPendingUpload()){notice.value='请先使用或取消待确认的图片。';return}if(!form.value?.name.trim()){notice.value='请填写名称。';return}busy.value=true;notice.value='';try{
 if(bindingPending.value){if(await flushAll()){form.value=undefined;bindingPending.value=false}return}
 const entity=copy(form.value);
 if(entity.kind==='character'&&voiceEditor.value){entity.voice_material_id=voiceEditor.value.selectedMaterialID();entity.voice_profile_id=undefined;entity.voice_profile_revision=undefined;form.value.voice_material_id=entity.voice_material_id}
 if(!isNew.value)entity.revision=Math.max(...catalog.value.entities.filter(e=>e.id===entity.id).map(e=>e.revision))+1;
 catalog.value=await bindingsApi.saveEntity(props.projectId,{expected_catalog_version:catalog.value.catalog_version,entity},isNew.value);
 const next=catalog.value.entities.filter(e=>e.id===entity.id).sort((a,b)=>b.revision-a.revision)[0]!;
 if(isNew.value)bind(next);else{const b=state.value.entity_bindings.find(b=>b.id===formBinding.value);if(b){changeBinding({...b,entity_revision:next.revision,selected_material_ids:undefined,reference_material_ids:undefined});editor.change({utterances:state.value.utterances.map(u=>u.speaker_id===next.id?{...u,speaker_revision:next.revision}:u)})}}
 bindingPending.value=true;if(await flushAll()){form.value=undefined;bindingPending.value=false}
 }catch(e){notice.value=(e as Error).message;const f=e as {status?:number;payload?:{catalog?:EntityCatalog}};if(f.status===409&&f.payload?.catalog){catalog.value=normalizeEntityCatalog(f.payload.catalog);notice.value='档案已被更新，已载入最新版本；你的编辑内容仍在，请比较后再次保存。'}}finally{busy.value=false}}
function addReference(material:Material){if(form.value&&!form.value.references.some(r=>r.material_id===material.id))form.value.references.push({material_id:material.id,order:form.value.references.length,label:material.name})}
function refs(ids:string[]){if(form.value)form.value.references=ids.map((id,i)=>({material_id:id,order:i,label:form.value!.references.find(r=>r.material_id===id)?.label||''}))}
const cardUploads=ref<{entity:EntityVersion;binding:string;file:File;error:string;done:boolean;saveToLibrary:boolean;replaceID?:string}[]>([]);
async function retryCardUploads(){busy.value=true;try{for(const u of cardUploads.value){if(u.done)continue;try{u.error='';const m=await api.uploadMaterial(u.file,u.entity.kind==='character'?'character':'scene',undefined,{projectId:props.projectId,referenceOnly:!u.saveToLibrary});emit('material',m);const binding=state.value.entity_bindings.find(b=>b.id===u.binding);if(binding){const ids=binding.reference_material_ids||binding.selected_material_ids||u.entity.references.map(r=>r.material_id);changeBinding({...binding,selected_material_ids:undefined,reference_material_ids:u.replaceID?ids.map(id=>id===u.replaceID?m.id:id):[...ids,m.id]})}u.done=true}catch(e){u.error=(e as Error).message}}}finally{busy.value=false}}
function cardUpload(entity:EntityVersion,binding:string,files:File[],replaceID?:string,saveToLibrary=false){cardUploads.value.push(...files.map(file=>({entity,binding,file,replaceID,saveToLibrary,error:'',done:false})));void retryCardUploads()}
onMounted(async()=>{if(props.focusName){nameInput.value?.focus();nameInput.value?.select()}try{catalog.value=await bindingsApi.catalog(props.projectId)}catch(e){notice.value=(e as Error).message}});
async function flushAll(){for(let i=0;i<5;i++){if(!await editor.flush())return false;if(!dirty.value)return true}return false}
async function generate() {
  if (props.submitting || busy.value || form.value || !videoReady.value) return;
  if (!await flushAll()) return;
  const s = settings.value;
  emit('generate', {
    expected_version: state.value.expected_version,
    model_id: s.video_model_id,
    video_model_id: s.video_model_id,
    path: 'reference_video',
    video_input_mode: s.video_input_mode,
    first_frame_material_id: s.video_input_mode === 'first_frame' ? s.first_frame_material_id : undefined,
    last_frame_material_id: s.video_input_mode === 'first_frame' ? s.last_frame_material_id : undefined,
    aspect_ratio: s.aspect_ratio,
    resolution: s.resolution,
    duration_seconds: s.duration_seconds,
  });
}

defineExpose({flush:async()=>!form.value&&!busy.value&&await flushAll()});
</script>
<template><header class="shot-name-section" aria-label="分镜基本信息"><div><span class="shot-title-eyebrow">分镜 {{String(scene.order).padStart(2,'0')}}</span><h1 class="sr-only">{{state.title || '未命名分镜'}}</h1><label><span class="sr-only">分镜名称</span><input ref="nameInput" :value="state.title" maxlength="100" placeholder="为这个分镜命名" @input="editor.change({title:($event.target as HTMLInputElement).value})" /></label><p>点击名称编辑 · 自动保存</p></div></header><section class="binding-workbench shot-configuration" aria-label="场景人物工作台"><header class="shot-config-heading"><h2 class="sr-only">分镜配置</h2>
<p role="status">{{saving?'保存中…':dirty?'未保存':'已保存'}} <button v-if="dirty&&!saving" @click="editor.flush()">保存 / 重试</button></p></header>
<p v-if="notice||error" role="alert" class="notice error">{{notice||error}}</p>
<p v-if="supplementsMerged" class="migration-note">原“本镜头补充”已合并到场景描述，可在下方继续修改。</p><div v-if="conflict" class="candidate-card"><p>草稿版本冲突，本地输入已保留</p><button @click="editor.rebase()">保留本地镜头内容并保存到最新版本</button><button @click="editor.useServer()">使用服务器内容</button></div>
<section aria-label="分镜引用素材"><header class="binding-heading"><h3>分镜引用素材 <small>{{groups.length}} 项</small></h3><button @click="create('location')">＋ 添加场景</button><button @click="create('character')">＋ 添加人物</button><button @click="library=!library">复用项目档案</button></header>
<div v-if="library" class="candidate-card"><p v-if="!latest.length">暂无档案，请先添加场景或人物。</p><button v-for="e in latest" :key="e.id" @click="bind(e)">{{e.kind==='location'?'场景':'人物'}} · {{e.name}} · v{{e.revision}}</button></div>
<div class="shot-entity-grid"><EntityReferenceCard v-for="g in groups.filter(g=>g.entity)" :key="g.binding.id" :entity="g.entity!" :binding="g.binding" :materials="mappedMaterials" :voices="voices" :disabled="busy||submitting" inline @change="changeBinding" @edit="edit(g.entity!,g.binding.id)" @remove="remove(g.binding.id)" @upload="(files,save)=>cardUpload(g.entity!,g.binding.id,files,undefined,save)" @replace="(id,file,save)=>cardUpload(g.entity!,g.binding.id,[file],id,save)" /></div>
<p v-if="!groups.length" class="empty">添加人物或场景，并上传参考图、填写描述。</p>
<p v-for="g in missingGroups" :key="g.binding.id" role="alert">档案引用失效 <button @click="remove(g.binding.id)">移除失效分组</button></p>
<p v-for="u in cardUploads.filter(u=>u.error)" :key="u.file.name" role="alert">{{u.file.name}}：{{u.error}}</p><button v-if="cardUploads.some(u=>u.error)" :disabled="busy" @click="retryCardUploads">重试失败图片</button>
<EntityEditorDrawer v-if="form" :title="`${isNew?'添加':'编辑'}${form.kind==='location'?'场景':'人物'}`" :busy="busy||bindingPending||sourceBusy" @close="closeForm"><form class="entity-form" @submit.prevent="saveEntity"><p>{{isNew?'保存人物或场景设定，并引用到当前分镜。':'保存后仅当前分镜采用新版本。'}}</p><p v-if="notice||error" role="alert" class="error">{{notice||error}}</p><p v-if="supplementsMerged" class="migration-note">原“本镜头补充”已合并到场景描述，可在下方继续修改。</p><div v-if="conflict" class="candidate-card"><p>草稿版本冲突，本地输入已保留</p><button type="button" @click="editor.rebase()">保留本地内容并重试</button></div><fieldset class="entity-drawer-fields" :disabled="busy||bindingPending"><label>名称<input v-model="form.name" required maxlength="100" /></label><div><h3>参考图片</h3><ReferenceImageSource ref="referenceEditor" :key="form.id" :project-id="projectId" :draft-id="draft.id" :entity="form" :materials="materials" :models="models" :disabled="busy||bindingPending" @add="addReference" @material="emit('material',$event)" @busy="sourceBusy=$event" /><div class="reference-selected-summary"><h4>已选参考图 · {{form.references.length}} 张</h4><ReferenceImageList :ids="form.references.map(r=>r.material_id)" :materials="mappedMaterials" :disabled="busy||bindingPending||sourceBusy" hide-upload @update:ids="refs" /><p v-if="!form.references.length" class="muted">从上方选择来源，确认使用后显示在这里。</p></div></div><label>参考描述<textarea v-model="form.description" rows="5" maxlength="2000" placeholder="填写场景环境、人物外观与服装等特征" /></label><CharacterVoiceSettings v-if="form.kind==='character'" ref="voiceEditor" :entity="form" :materials="materials" :disabled="busy||bindingPending" /></fieldset>

 <div class="actions"><button type="button" :disabled="busy||bindingPending||sourceBusy" @click="closeForm">取消</button><button class="primary" :disabled="busy||sourceBusy">{{busy?'保存中…':bindingPending?'重试引用保存':'保存并引用'}}</button></div></form></EntityEditorDrawer>
</section>
<section class="shot-config-section shot-description" aria-label="场景描述">
 <header class="shot-section-heading"><h3>场景描述</h3><p>人物动作、景别和镜头运动统一在这里描述。</p></header>
 <div class="shot-prompt-field"><MentionPromptEditor :show-insert-button="false" :model-value="state.prompt_document" :options="mentionOptions" @update:model-value="editor.change({prompt_document:$event})" /></div>
 <details class="shot-audio-options" :open="!!state.narration||state.utterances.length>0"><summary>旁白与台词 <span>选填 · 作为视频描述提交</span></summary><div class="shot-audio-content"><label>旁白文本<textarea :value="state.narration" placeholder="填写这段分镜的旁白文字" @input="editor.change({narration:($event.target as HTMLTextAreaElement).value})" /></label><DialogueVoicePanel :lines="state.utterances" :characters="catalog.entities.filter(e=>e.kind==='character')" :voices="voices" :busy="busy||submitting" edit-only text-only @change="editor.change({utterances:$event})" /></div></details>
</section>
<ShotGenerationSettings :model-value="settings" :models="models" :images="videoImages" :disabled="submitting" @ready="videoReady=$event" @update:model-value="editor.change({generation_settings:$event})" />
<p v-if="groups.some(g=>g.entity?.voice_profile_id)" class="voice-scope-note">当前参考生视频不会使用人物设定的配音音色，视频中的声音由视频模型生成。</p><footer class="shot-generate-footer"><span>使用引用素材与场景描述生成视频</span><button class="primary" :disabled="submitting||busy||saving||!videoReady" @click="generate">{{submitting?'提交中…':'生成视频'}}</button></footer>
</section></template>
