<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount,computed} from 'vue';
import {useRouter} from 'vue-router';
import {bindingsApi,type EntityCatalog} from '../api/bindings';
import type {Scene} from '../api/types';
import {api} from '../api/client';
import {useShotWorkspace} from '../composables/useShotWorkspace';
const {projectId,draftId,base,draft,error,loading}=useShotWorkspace();const busy=ref(false);
const router=useRouter();let alive=true;
onBeforeUnmount(()=>{alive=false});
const createKey=`frameflow-create-scene:${projectId}:${draftId}`;
let pendingCreate:{creation_id:string;expected_version:number}|undefined;
try{const stored=sessionStorage.getItem(createKey);if(stored)pendingCreate=JSON.parse(stored)}catch{/* retain in memory */}
async function createScene(){
 if(!draft.value||loading.value||busy.value)return;
 busy.value=true;error.value='';
 if(!pendingCreate)pendingCreate={creation_id:`scene-created-${crypto.randomUUID()}`,expected_version:draft.value.version||0};
 try{sessionStorage.setItem(createKey,JSON.stringify(pendingCreate))}catch{/* retain in memory */}
 try{
  const id=pendingCreate.creation_id;
  const next=await api.writeScene(projectId,draftId,null,{...pendingCreate,action:'create',title:'未命名分镜',duration_seconds:4});
  if(!alive)return;
  draft.value=next;pendingCreate=undefined;try{sessionStorage.removeItem(createKey)}catch{/* storage unavailable */}
  await router.push(`${base}/shots/${encodeURIComponent(id)}?created=1`);
 }catch(e){
  const failure=e as Error&{status?:number;payload?:{draft?:typeof draft.value}};
  if(failure.status&&failure.status<500){pendingCreate=undefined;try{sessionStorage.removeItem(createKey)}catch{/* storage unavailable */}if(failure.payload?.draft)draft.value=failure.payload.draft}
  if(alive)error.value=failure.message||'创建未完成，请重试。';
 }finally{busy.value=false}
}
const scenes=computed(()=>draft.value?.story.scenes||[]);
const catalog=ref<EntityCatalog>();onMounted(async()=>{try{catalog.value=await bindingsApi.catalog(projectId)}catch{catalog.value=undefined}});
function references(scene:Scene){return (scene.entity_bindings||[]).map(b=>catalog.value?.entities.find(e=>e.id===b.entity_id&&e.revision===b.entity_revision)).filter(Boolean).map(e=>`${e!.kind==='character'?'人物':'场景'}：${e!.name}`).join(' · ')}
function progress(scene:Scene){const run=draft.value?.generation_runs?.filter(r=>r.scene_id===scene.id).at(-1);return !run?'尚未创建任务':run.state==='completed'?'成片已完成':run.state==='awaiting_composition'?'视频已选用，待合成':run.state==='failed'?'失败':run.state==='cancelled'?'已取消':'制作中 / 待选用'}
async function action(id:string,kind:'copy'|'move',order?:number){if(!draft.value||busy.value)return;busy.value=true;try{draft.value=await api.writeScene(projectId,draftId,id,{expected_version:draft.value.version||0,action:kind,order})}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function remove(id:string){if(!draft.value||busy.value)return;const shot=scenes.value.find(s=>s.id===id);if(!window.confirm(`删除分镜「${shot?.title||id}」？`))return;busy.value=true;try{draft.value=await api.deleteScene(projectId,draftId,id,draft.value.version||0)}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
</script>
<template><main class="shot-workspace"><nav class="shot-breadcrumb"><RouterLink :to="base">返回正文</RouterLink><RouterLink :to="`${base}?tab=video`">成片</RouterLink></nav><header class="shot-page-heading"><h1>分镜列表</h1><span>{{scenes.length}} 个分镜</span></header><p v-if="error" role="alert">{{error}}</p><p v-if="loading">加载中…</p><div class="shot-list-create"><button class="primary" :disabled="busy || loading || !draft" @click="createScene">{{busy?'处理中…':'＋ 新增分镜'}}</button></div><section v-if="draft" class="shot-list" aria-label="分镜列表"><p v-if="!scenes.length" class="empty">暂无分镜，可新增或返回正文生成。</p><article v-for="s in scenes" :key="s.id" class="shot-list-row"><span class="shot-number">{{String(s.order).padStart(2,'0')}}</span><div><RouterLink :to="`${base}/shots/${encodeURIComponent(s.id)}`">{{s.title}}</RouterLink><p>{{s.duration_seconds}} 秒 · {{s.entity_bindings?.length||0}} 个引用素材 · {{draft.generation_runs?.filter(r=>r.scene_id===s.id).length||0}} 个制作任务</p><p>{{references(s)||'尚未绑定人物与场景'}}</p><span>{{progress(s)}}</span></div><RouterLink :to="`${base}/shots/${encodeURIComponent(s.id)}`">进入详情 →</RouterLink><div class="shot-list-actions"><button :disabled="busy" @click="action(s.id,'copy')">复制</button><button :disabled="busy||s.order===1" @click="action(s.id,'move',s.order-1)">上移</button><button :disabled="busy||s.order===scenes.length" @click="action(s.id,'move',s.order+1)">下移</button><button :disabled="busy" @click="remove(s.id)">删除</button></div></article></section></main></template>
