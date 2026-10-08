import {ref,onMounted,onBeforeUnmount,computed} from 'vue';
import {useRoute} from 'vue-router';
import {api} from '../api/client';
import type {Draft,Material,Model} from '../api/types';

export function useShotWorkspace(){
 const route=useRoute();const projectId=String(route.params.projectId),draftId=String(route.params.draftId),sceneId=String(route.params.sceneId||'');
 const base=`/studio/${encodeURIComponent(projectId)}/${encodeURIComponent(draftId)}`;
 const draft=ref<Draft>(),models=ref<Model[]>([]),materials=ref<Material[]>([]),error=ref(''),loading=ref(true);let alive=true;
 const scene=computed(()=>draft.value?.story.scenes.find(s=>s.id===sceneId));
 async function reload(){try{const d=await api.draft(projectId,draftId);if(alive)draft.value=d}catch(e){if(alive)error.value=(e as Error).message}}
 onMounted(async()=>{try{const [d,m,a]=await Promise.all([api.draft(projectId,draftId),api.models(),api.materials("", "", "", projectId)]);if(alive){draft.value=d;models.value=m.models;materials.value=a.materials;if(sceneId&&!d.story.scenes.some(s=>s.id===sceneId))error.value='分镜不存在或已删除'}}catch(e){if(alive)error.value=(e as Error).message}finally{if(alive)loading.value=false}});
 onBeforeUnmount(()=>alive=false);
 return {projectId,draftId,sceneId,base,draft,scene,models,materials,error,loading,reload};
}
