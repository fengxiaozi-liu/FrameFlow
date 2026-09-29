import { ref, watch, onBeforeUnmount } from 'vue';
import { bindingsApi, type ShotContext } from '../api/bindings';
import type { Draft, Scene } from '../api/types';

const clone=<T>(v:T):T=>JSON.parse(JSON.stringify(v));
export function useShotEditor(source:()=>{project:string;draft:Draft;scene:Scene}, saved:(d:Draft)=>void){
 const initial=source();
 const key=`frameflow-shot:${initial.project}:${initial.draft.id}:${initial.scene.id}`;
 const state=ref<ShotContext>({expected_version:initial.draft.version||0,entity_bindings:clone(initial.scene.entity_bindings||[]),prompt_document:clone(initial.scene.prompt_document||{schema_version:1,nodes:[{type:'text',text:initial.scene.visual_prompt||''}]}),utterances:clone(initial.scene.utterances||[])});
 const dirty=ref(false),saving=ref(false),error=ref(''),conflict=ref<Draft>();let revision=0,timer:ReturnType<typeof setTimeout>|undefined,pending:Promise<boolean>|undefined;
 try{const local=localStorage.getItem(key);if(local){state.value=JSON.parse(local);dirty.value=true;error.value='已恢复未保存内容，请检查后保存。'}}catch{/* Storage may be unavailable. */}
 function remember(){try{localStorage.setItem(key,JSON.stringify(state.value))}catch{error.value='浏览器无法暂存，请保持页面打开并保存。'}}
 function change(patch:Partial<ShotContext>){Object.assign(state.value,clone(patch));dirty.value=true;revision++;error.value='';remember();clearTimeout(timer);timer=setTimeout(()=>{void flush()},650)}
 async function flush():Promise<boolean>{
  clearTimeout(timer);if(pending)return pending;if(!dirty.value)return true;if(conflict.value)return false;
  if(state.value.utterances.some(u=>!u.text.trim())){error.value='请填写台词或删除空台词后保存。';return false}
  saving.value=true;const at=revision;const body=clone(state.value);
  pending=(async()=>{try{
   const result=await bindingsApi.saveContext(initial.project,initial.draft.id,initial.scene.id,body) as Draft;
   state.value.expected_version=result.version||0;saved(result);
   if(at===revision){dirty.value=false;localStorage.removeItem(key)}else{remember()}
   error.value='';return true;
  }catch(e){const failure=e as Error & {status?:number;payload?:{draft?:Draft}};error.value=failure.message;if(failure.status===409){conflict.value=failure.payload?.draft;error.value='此草稿已在其他位置更新。本地输入已保留，请比较后选择。'}return false}
  finally{saving.value=false;pending=undefined;if(dirty.value&&!error.value&&!conflict.value)timer=setTimeout(()=>{void flush()},0)}})();return pending;
 }
 function rebase(){if(!conflict.value)return;state.value.expected_version=conflict.value.version||0;conflict.value=undefined;error.value='';remember();void flush()}
 function useServer(){const d=conflict.value;const s=d?.story.scenes.find(s=>s.id===initial.scene.id);if(!d||!s)return;state.value={expected_version:d.version||0,entity_bindings:clone(s.entity_bindings||[]),prompt_document:clone(s.prompt_document||{schema_version:1,nodes:[{type:'text',text:s.visual_prompt}]}),utterances:clone(s.utterances||[])};dirty.value=false;conflict.value=undefined;error.value='';localStorage.removeItem(key);saved(d)}
 watch(()=>source().draft.version,v=>{if(!dirty.value&&!saving.value)state.value.expected_version=v||0});
 const before=(e:BeforeUnloadEvent)=>{if(dirty.value){remember();e.preventDefault()}};window.addEventListener('beforeunload',before);
 onBeforeUnmount(()=>{clearTimeout(timer);window.removeEventListener('beforeunload',before);if(dirty.value){remember();void flush()}});
 return {state,dirty,saving,error,conflict,change,flush,rebase,useServer};
}
