<script setup lang="ts">
import { defaultEntityName } from "./entityDefaults";
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue';
import { api } from '../../api/client';
import { bindingsApi, type EntityCatalog, type EntityVersion, type ShotEntityBinding, type VoiceProfile } from '../../api/bindings';
import type { Draft, Scene, Material, Model } from '../../api/types';
import { useShotEditor } from '../../composables/useShotEditor';
import EntityReferenceCard from './EntityReferenceCard.vue';
import MentionPromptEditor from './MentionPromptEditor.vue';
import DialogueVoicePanel from './DialogueVoicePanel.vue';
import ReferenceImageList from './ReferenceImageList.vue';
import GenerationToolbar from './GenerationToolbar.vue';
import ShotPreview from './ShotPreview.vue';
import {useTaskStore} from '../../stores/tasks';
import type {components} from '../../api/schema';
const props=defineProps<{projectId:string;draft:Draft;scene:Scene;materials:Material[];models:Model[]}>();
const emit=defineEmits<{saved:[Draft];material:[Material]}>();
const editor=useShotEditor(()=>({project:props.projectId,draft:props.draft,scene:props.scene}),d=>emit('saved',d));
const {state,dirty,saving,error,conflict}=editor;
const catalog=ref<EntityCatalog>({catalog_version:0,entities:[],voice_profiles:[]}),notice=ref(''),busy=ref(false);
const form=ref<EntityVersion>(),formBinding=ref(''),isNew=ref(false),library=ref(false);
const voiceModel=ref(''),voiceRemote=ref(''),voiceLanguage=ref('Chinese'),voiceOptions=ref<{id:string;name:string;languages:string[]}[]>([]);
const uploads=ref<{file:File;status:string;error:string}[]>([]);
const imageModel=ref(''),videoModel=ref(''),aspect=ref('16:9'),resolution=ref('720P'),duration=ref(props.scene.duration_seconds),generationBusy=ref(false);
const capability=ref<components['schemas']['ModelInputCapability']>(),videoCapability=ref<components['schemas']['ModelInputCapability']>();
const takes=ref<components['schemas']['MediaTake'][]>([]),selected=ref<Record<string,string>>({}),preview=ref<components['schemas']['GenerationSnapshot']>();
const taskLabels:Record<string,string>={queued:'排队中',running:'生成中',succeeded:'已完成',failed:'失败',cancelled:'已取消'};
const tasks=useTaskStore();const shotTasks=computed(()=>tasks.items.filter(t=>t.project_id===props.projectId&&t.draft_id===props.draft.id&&t.scene_id===props.scene.id));
const runs=ref<components['schemas']['GenerationRun'][]>((props.draft.generation_runs||[]).filter(r=>r.scene_id===props.scene.id));
const runLabels:Record<string,string>={preparing:'准备首帧与配音',generating:'生成首帧与配音',awaiting_selection:'请选用首帧及每句配音',generating_video:'正在生成视频',awaiting_video_selection:'请选用视频版本',awaiting_composition:'请前往「成片」合成',completed:'成片已完成',cancelled:'已取消',failed:'子任务失败，可按原输入重试'};
async function startRun(){if(generationBusy.value)return;generationBusy.value=true;try{if(!await editor.flush())return;if(dirty.value)return;const r=await bindingsApi.run(props.projectId,props.draft.id,props.scene.id,{...input('reference_image_then_video',imageModel.value),video_model_id:videoModel.value});runs.value.push(r);await refreshResults()}catch(e){notice.value=(e as Error).message}finally{generationBusy.value=false}}
async function refreshRuns(){for(let i=0;i<runs.value.length;i++){const r=runs.value[i]!;if(!['completed','cancelled'].includes(r.state))try{runs.value[i]=await bindingsApi.getRun(props.projectId,props.draft.id,props.scene.id,r.id)}catch(e){notice.value=(e as Error).message}}}
const imageModels=computed(()=>props.models.filter(m=>m.availability?.image?.available === true)),videoModels=computed(()=>props.models.filter(m=>m.availability?.video?.available === true));
const refCount=computed(()=>groups.value.reduce((n,g)=>n+(g.binding.selected_material_ids||g.entity?.references.map(r=>r.material_id)||[]).length,0));
const audioUrls=computed(()=>Object.fromEntries(takes.value.filter(t=>t.kind==='speech'&&props.materials.some(m=>m.id===t.material_id&&m.url)&&selected.value[props.scene.id+':speech:'+t.utterance_id]===t.id).map(t=>[t.utterance_id!,api.mediaUrl(props.materials.find(m=>m.id===t.material_id)?.url||'')])));
const selectedImage=computed(()=>props.materials.find(m=>m.id===takes.value.find(t=>t.id===selected.value[props.scene.id+':image'])?.material_id)?.url);
const selectedVideo=computed(()=>props.draft.video_versions?.find(v=>v.id===props.draft.selected_versions?.[props.scene.id])?.local_media_path);
watch(imageModel,async id=>{capability.value=undefined;if(id)try{capability.value=await bindingsApi.capability(id)}catch(e){notice.value=(e as Error).message}});
watch(videoModel,async id=>{videoCapability.value=undefined;if(id)try{videoCapability.value=await bindingsApi.capability(id)}catch(e){notice.value=(e as Error).message}});
async function refreshResults(){try{const result=await bindingsApi.takes(props.projectId,props.draft.id,props.scene.id);takes.value=result.takes;selected.value=result.selected||{};const ids=new Set(props.materials.map(m=>m.id));for(const t of result.takes)if(!ids.has(t.material_id)){emit('material',await api.material(t.material_id));ids.add(t.material_id)}await tasks.load(props.projectId,props.draft.id)}catch(e){notice.value=(e as Error).message}}
function input(path:'reference_image_then_video'|'first_frame_video'|'speech',model:string){return {expected_version:state.value.expected_version,idempotency_key:crypto.randomUUID(),model_id:model,path,aspect_ratio:aspect.value,resolution:resolution.value,duration_seconds:duration.value}}
async function generate(kind:'preview'|'image'|'video'|'speech',utteranceID=''){
 if(generationBusy.value)return;generationBusy.value=true;notice.value='';try{if(!await editor.flush())return;if(dirty.value){notice.value='请等待当前修改保存后生成。';return}
 const inData=input(kind==='video'?'first_frame_video':kind==='speech'?'speech':'reference_image_then_video',kind==='video'?videoModel.value:imageModel.value);
 if(kind==='speech'){const u=state.value.utterances.find(u=>u.id===utteranceID)!;const c=catalog.value.entities.find(c=>c.id===u.speaker_id&&c.revision===u.speaker_revision);const v=voices.value.find(v=>v.id===(u.voice_profile_id||c?.voice_profile_id)&&v.revision===(u.voice_profile_revision||c?.voice_profile_revision));if(!v)throw Error('请先为此台词选择音色，或设置人物默认音色。');inData.model_id=v.model_id;await bindingsApi.speech(props.projectId,props.draft.id,props.scene.id,utteranceID,inData)}
 else if(kind==='preview'){const result=await bindingsApi.preview(props.projectId,props.draft.id,props.scene.id,inData);preview.value=result.snapshot}
 else if(kind==='image')await bindingsApi.image(props.projectId,props.draft.id,props.scene.id,inData);
 else {if(!selectedImage.value&&!props.draft.bindings?.some(b=>b.scene_id===props.scene.id&&b.usage==='first_frame'))throw Error('请先生成分镜图，并点击「选用此结果」作为视频首帧。');await bindingsApi.video(props.projectId,props.draft.id,props.scene.id,inData);}
 if(kind!=='preview'){notice.value='任务已提交，完成后请试听或预览并选用。';await refreshResults()}
 }catch(e){const f=e as Error&{payload?:{issues?:{message:string}[]}};notice.value=f.payload?.issues?.map(i=>i.message).join('；')||f.message}finally{generationBusy.value=false}}
async function selectTake(t:components['schemas']['MediaTake']){generationBusy.value=true;try{if(!await editor.flush())return;const d=await bindingsApi.select(props.projectId,props.draft.id,props.scene.id,{expected_version:state.value.expected_version,take_id:t.id,utterance_id:t.utterance_id}) as Draft;state.value.expected_version=d.version||0;emit('saved',d);await refreshResults()}catch(e){notice.value=(e as Error).message}finally{generationBusy.value=false}}
let refreshTimer:ReturnType<typeof setInterval>|undefined;
onBeforeUnmount(()=>clearInterval(refreshTimer));
const latest=computed(()=>{const map=new Map<string,EntityVersion>();for(const e of catalog.value.entities||[]){if(!map.has(e.id)||map.get(e.id)!.revision<e.revision)map.set(e.id,e)}return [...map.values()].filter(e=>!e.archived)});
const groups=computed(()=>state.value.entity_bindings.map(b=>({binding:b,entity:catalog.value.entities.find(e=>e.id===b.entity_id&&e.revision===b.entity_revision)})));
const sceneGroups=computed(()=>groups.value.filter(g=>g.entity?.kind==='location'));
const characterGroups=computed(()=>groups.value.filter(g=>g.entity?.kind==='character'));
const missingGroups=computed(()=>groups.value.filter(g=>!g.entity));
const mentionOptions=computed(()=>groups.value.filter(g=>g.entity).map(g=>({id:g.binding.id,name:g.entity!.name,kind:g.entity!.kind})));
const voices=computed(()=>catalog.value.voice_profiles||[]);
const mappedMaterials=computed(()=>props.materials.map(m=>({...m,url:api.mediaUrl(m.url)})));
const speechModels=computed(()=>props.models.filter(m=>m.availability?.speech?.available === true));
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
  uploads.value = [];
  voiceModel.value = '';
  voiceRemote.value = '';
}
function edit(e:EntityVersion,id:string){form.value=copy(e);formBinding.value=id;isNew.value=false;uploads.value=[];voiceModel.value='';voiceRemote.value=''}
async function loadVoices(){voiceOptions.value=[];voiceRemote.value='';if(!voiceModel.value)return;try{voiceOptions.value=(await bindingsApi.voices(voiceModel.value)).voices||[]}catch(e){notice.value=(e as Error).message}}
async function saveEntity(){if(busy.value)return;if(!form.value?.name.trim()){notice.value='请填写名称。';return}busy.value=true;notice.value='';try{
 let entity=copy(form.value);
 if(!isNew.value)entity.revision=Math.max(...catalog.value.entities.filter(e=>e.id===entity.id).map(e=>e.revision))+1;
 if(voiceModel.value){const model=speechModels.value.find(m=>m.id===voiceModel.value);if(!model||!voiceRemote.value)throw Error('请选择可用音色');const voice:VoiceProfile={id:crypto.randomUUID(),revision:1,name:entity.name+' · '+voiceRemote.value,connection_id:model.connection_id,model_id:model.id,remote_voice_id:voiceRemote.value,language:voiceLanguage.value};catalog.value=await bindingsApi.saveVoice(props.projectId,{expected_catalog_version:catalog.value.catalog_version,voice_profile:voice},true);entity={...entity,voice_profile_id:voice.id,voice_profile_revision:1}}
 catalog.value=await bindingsApi.saveEntity(props.projectId,{expected_catalog_version:catalog.value.catalog_version,entity},isNew.value);
 const next=catalog.value.entities.filter(e=>e.id===entity.id).sort((a,b)=>b.revision-a.revision)[0]!;
 if(isNew.value)bind(next);else{const b=state.value.entity_bindings.find(b=>b.id===formBinding.value);if(b){changeBinding({...b,entity_revision:next.revision,selected_material_ids:undefined});editor.change({utterances:state.value.utterances.map(u=>u.speaker_id===next.id?{...u,speaker_revision:next.revision}:u)})}}
 form.value=undefined;await editor.flush();
 }catch(e){notice.value=(e as Error).message;const f=e as {status?:number;payload?:{catalog?:EntityCatalog}};if(f.status===409&&f.payload?.catalog){catalog.value=f.payload.catalog;notice.value='档案已被更新，已载入最新版本；你的编辑内容仍在，请比较后再次保存。'}}finally{busy.value=false}}
async function upload(files:File[]){uploads.value.push(...files.map(file=>({file,status:'等待上传',error:''})));await retryUploads()}
async function retryUploads(){if(!form.value)return;busy.value=true;for(const item of uploads.value){if(item.status==='已上传')continue;try{item.error='';item.status='上传中';const m=await api.uploadMaterial(item.file,form.value.kind==='character'?'character':'scene',n=>item.status=`上传中 ${n}%`);emit('material',m);form.value.references.push({material_id:m.id,order:form.value.references.length,label:item.file.name.replace(/\.[^.]+$/,'')});item.status='已上传'}catch(e){item.status='失败';item.error=(e as Error).message}}busy.value=false}
function refs(ids:string[]){if(form.value)form.value.references=ids.map((id,i)=>({material_id:id,order:i,label:form.value!.references.find(r=>r.material_id===id)?.label||''}))}
function cardUpload(e:EntityVersion,b:string,files:File[]){edit(e,b);void upload(files)}
onMounted(async()=>{try{catalog.value=await bindingsApi.catalog(props.projectId);imageModel.value=imageModels.value.find(m=>(m.availability?.image?.binding.max_references || 0)>0)?.id||'';videoModel.value=videoModels.value.find(m=>m.availability?.video?.binding.input_mode==='first_frame')?.id||'';await refreshResults();refreshTimer=setInterval(()=>{void refreshResults();void refreshRuns()},4000)}catch(e){notice.value=(e as Error).message}});
async function flushAll(){for(let i=0;i<5;i++){if(!await editor.flush())return false;if(!dirty.value)return true}return false}
defineExpose({flush:flushAll});
</script>
<template><section class="binding-workbench" aria-label="场景人物工作台">
 <header class="binding-heading"><div><h3>场景与人物</h3><p>文字设定与参考图一起参与生成</p></div><button @click="create('location')">＋ 添加场景</button><button @click="create('character')">＋ 添加人物</button><button @click="library=!library">复用项目档案</button></header>
 <p role="status">{{saving?'保存中…':dirty?'未保存':'已保存'}} <button v-if="dirty&&!saving" @click="editor.flush()">保存 / 重试</button></p>
 <p v-if="notice||error" role="alert" class="notice error">{{notice||error}}</p>
 <div v-if="conflict" class="candidate-card"><p>服务器版本 {{conflict.version}}；以下是服务器镜头内容：</p><pre>{{conflict.story.scenes.find(s=>s.id===scene.id)}}</pre><button @click="editor.rebase()">保留本地镜头内容并保存到最新版本</button><button @click="editor.useServer()">使用服务器内容</button></div>
 <div v-if="library" class="candidate-card"><p v-if="!latest.length">暂无档案，请先添加场景或人物。</p><button v-for="e in latest" :key="e.id" @click="bind(e)">{{e.kind==='location'?'场景':'人物'}} · {{e.name}} · {{e.id.slice(-6)}} · v{{e.revision}}</button></div>
 <div class="studio-preview-grid"> <ShotPreview :title="scene.title" :url="selectedVideo?api.mediaUrl(selectedVideo):undefined" :image="selectedImage?api.mediaUrl(selectedImage):undefined" /> <DialogueVoicePanel :lines="state.utterances" :characters="catalog.entities.filter(e=>e.kind==='character')" :voices="voices" :audio-urls="audioUrls" :busy="generationBusy" @change="editor.change({utterances:$event})" @generate="generate('speech',$event)" /></div>
 <div class="binding-groups" aria-label="当前镜头绑定">
  <section class="binding-group binding-group-scene" aria-label="场景绑定"><h3>场景绑定 <small>{{sceneGroups.length}} 个</small></h3><p v-if="!sceneGroups.length" class="empty">尚未绑定场景，点击上方“添加场景”或“复用项目档案”。</p><EntityReferenceCard v-for="g in sceneGroups" :key="g.binding.id" :entity="g.entity!" :binding="g.binding" :materials="mappedMaterials" :voices="voices" :disabled="busy" @change="changeBinding" @edit="edit(g.entity!,g.binding.id)" @remove="remove(g.binding.id)" @upload="cardUpload(g.entity!,g.binding.id,$event)" /></section>
  <section class="binding-group binding-group-character" aria-label="人物绑定"><h3>人物绑定 <small>{{characterGroups.length}} 个</small></h3><p v-if="!characterGroups.length" class="empty">尚未绑定人物，点击上方“添加人物”或“复用项目档案”。</p><EntityReferenceCard v-for="g in characterGroups" :key="g.binding.id" :entity="g.entity!" :binding="g.binding" :materials="mappedMaterials" :voices="voices" :disabled="busy" @change="changeBinding" @edit="edit(g.entity!,g.binding.id)" @remove="remove(g.binding.id)" @upload="cardUpload(g.entity!,g.binding.id,$event)" /></section>
  <p v-for="g in missingGroups" :key="g.binding.id" role="alert">档案引用失效 <button @click="remove(g.binding.id)">移除失效分组</button></p>
 </div>
 <form v-if="form" class="entity-form" @submit.prevent="saveEntity"><h3>{{isNew?'添加':'编辑项目'}}{{form.kind==='location'?'场景':'人物'}}</h3><p>保存为新的项目档案版本，仅当前镜头采用新版本；其他镜头保留原设定。</p><label>名称<input v-model="form.name" required maxlength="100" /></label><label>基础描述<textarea v-model="form.description" rows="3" maxlength="2000" /></label><ReferenceImageList :ids="form.references.map(r=>r.material_id)" :materials="mappedMaterials" :disabled="busy" @update:ids="refs" @upload="upload" /><label v-for="r in form.references" :key="r.material_id">视角标签<input v-model="r.label" placeholder="正面 / 侧面 / 室内" /></label>
 <p v-for="(u,i) in uploads" :key="i">{{u.file.name}} · {{u.status}} {{u.error}}</p><button v-if="uploads.some(u=>u.status==='失败')" type="button" :disabled="busy" @click="retryUploads">重试失败图片</button>
 <fieldset v-if="form.kind==='character'"><legend>默认音色</legend><p v-if="form.voice_profile_id">当前：{{voices.find(v=>v.id===form!.voice_profile_id&&v.revision===form!.voice_profile_revision)?.name}}</p><label>语音模型<select v-model="voiceModel" @change="loadVoices"><option value="">保留当前音色</option><option v-for="m in speechModels" :key="m.id" :value="m.id">{{m.name}}</option></select></label><label v-if="voiceModel">音色<select v-model="voiceRemote"><option value="">请选择</option><option v-for="v in voiceOptions" :key="v.id" :value="v.id">{{v.name}}</option></select></label><label v-if="voiceModel">语言<select v-model="voiceLanguage"><option v-for="l in voiceOptions.find(v=>v.id===voiceRemote)?.languages||['Chinese','English']" :key="l">{{l}}</option></select></label><p v-if="!speechModels.length">请先在模型配置中添加语音合成模型。</p></fieldset>
 <div class="actions"><button type="button" :disabled="busy" @click="form=undefined">取消</button><button class="primary" :disabled="busy">保存设定并用于此镜头</button></div></form>
 <h3>当前镜头描述</h3><MentionPromptEditor :model-value="state.prompt_document" :options="mentionOptions" @update:model-value="editor.change({prompt_document:$event})" />


 <div class="generation-settings"><label>分镜图模型<select v-model="imageModel"><option value="">请选择多图模型</option><option v-for="m in imageModels" :key="m.id" :value="m.id">{{m.name}}</option></select></label><label>视频模型<select v-model="videoModel"><option value="">请选择视频模型</option><option v-for="m in videoModels" :key="m.id" :value="m.id">{{m.name}}</option></select></label><label>画幅<select v-model="aspect"><option>16:9</option><option>9:16</option><option>1:1</option></select></label><label>清晰度<select v-model="resolution"><option v-for="r in videoCapability?.resolutions||[]" :key="r">{{r}}</option></select></label><label>时长<select v-model.number="duration"><option v-for="d in videoCapability?.durations||[]" :key="d" :value="d">{{d}} 秒</option></select></label></div>
 <p>单张参考图不超过 10 MB；当前生图模型最多使用 {{capability?.reference_images ?? '待查询'}} 张，可在分组中移除本镜头不使用的参考图。参考图用于生成分镜图；选用的分镜图作为视频首帧。逐句配音在成片阶段合入，并静音原片人声。</p>
 <section class="shot-task-section" aria-label="当前镜头制作任务"><h3>制作任务 <small>当前镜头：{{scene.title}}</small></h3><p>这里显示生成进度和结果；上方的场景与人物是镜头绑定。</p><button class="primary" :disabled="generationBusy||runs.some(r=>!['completed','cancelled','failed'].includes(r.state))" @click="startRun">开始完整制作流程</button>
 <div v-for="r in runs" :key="r.id" class="candidate-card"><strong>{{runLabels[r.state]||r.state}}</strong><p>{{r.error}}</p><p>子任务：{{r.task_ids.length}} 个</p><button v-if="!['completed','cancelled'].includes(r.state)" @click="bindingsApi.cancelRun(projectId,draft.id,scene.id,r.id).then(refreshRuns).catch(e=>notice=e.message)">取消制作流程</button></div>
 <GenerationToolbar :busy="generationBusy||saving" :status="shotTasks.some(t=>['queued','running'].includes(t.status))?'任务处理中':'可预览生成输入'" :reference-count="refCount" :reference-limit="capability?.reference_images" @preview="generate('preview')" @image="generate('image')" @video="generate('video')" />
 <details v-if="preview" open><summary>实际生成输入</summary><p>{{preview.prompt}}</p><p v-for="e in preview.entities" :key="e.binding_id">{{e.entity_id}} · {{e.references.length}} 张参考图 · {{e.description}}</p></details>
 <div v-for="t in takes" :key="t.id" class="media-take"><img v-if="t.kind==='image'" :src="api.mediaUrl(materials.find(m=>m.id===t.material_id)?.url||'')" alt="生成分镜候选" /><audio v-else :src="api.mediaUrl(materials.find(m=>m.id===t.material_id)?.url||'')" controls preload="none" /><span v-if="t.stale">设定已改变，请重新生成</span><button :disabled="generationBusy||t.stale||Object.values(selected).includes(t.id)" @click="selectTake(t)">{{Object.values(selected).includes(t.id)?'已选用':'选用此结果'}}</button></div>
 <div v-for="t in shotTasks" :key="t.id" class="task-row"><span>任务 · {{t.kind==='speech'?'配音':t.kind==='image'?'分镜图':'视频'}} · {{taskLabels[t.status]||t.status}} · {{t.progress}}%</span><p v-if="t.error">{{t.error}}</p><button v-if="t.status==='failed'||t.status==='cancelled'" @click="api.retryTask(t.id).then(refreshResults).catch(e=>notice=e.message)">按原输入重试</button><button v-if="t.status==='running'||t.status==='queued'" @click="api.cancelTask(t.id).then(refreshResults).catch(e=>notice=e.message)">取消</button></div></section>
</section></template>
