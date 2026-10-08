import type { components } from './schema';
import { request } from './client';
type S = components['schemas'];
export type EntityVersion=S['EntityVersion'];
export type EntityCatalog=S['EntityCatalog'];
export type VoiceProfile=S['VoiceProfile'];
export type ShotEntityBinding=S['ShotEntityBinding'];
export type PromptDocument=S['PromptDocument'];
export type Utterance=S['Utterance'];
export type ShotContext=S['ShotContextWrite'];
export type GenerationRequest=S['GenerationRequest'];
export type GenerationRun=S['GenerationRun'];
export type ShotGenerationSettings=S['ShotGenerationSettings'];
const project=(id:string)=>`/api/projects/${encodeURIComponent(id)}`;
const shot=(p:string,d:string,s:string)=>`${project(p)}/drafts/${encodeURIComponent(d)}/scenes/${encodeURIComponent(s)}`;
const write=<T>(path:string,method:string,body:unknown)=>request<T>(path,{method,body:JSON.stringify(body)});
// Older catalog writes serialize an empty Go slice as null. Keep one stable
// shape for all readers, including the intermediate voice-before-entity save.
export const normalizeEntityCatalog=(catalog:EntityCatalog):EntityCatalog=>({...catalog,entities:catalog.entities||[],voice_profiles:catalog.voice_profiles||[]});
export const bindingsApi={
 catalog:(p:string)=>request<EntityCatalog>(`${project(p)}/entities`).then(normalizeEntityCatalog),
 saveEntity:(p:string,input:S['EntityWrite'],create=false)=>write<EntityCatalog>(`${project(p)}/entities${create?'':`/${encodeURIComponent(input.entity.id)}`}`,create?'POST':'PATCH',input).then(normalizeEntityCatalog),
 archiveEntity:(p:string,input:S['EntityWrite'])=>write<EntityCatalog>(`${project(p)}/entities/${encodeURIComponent(input.entity.id)}/archive`,'POST',input).then(normalizeEntityCatalog),
 saveVoice:(p:string,input:S['VoiceProfileWrite'],create=false)=>write<EntityCatalog>(`${project(p)}/voice-profiles${create?'':`/${encodeURIComponent(input.voice_profile.id)}`}`,create?'POST':'PATCH',input).then(normalizeEntityCatalog),
 saveContext:(p:string,d:string,s:string,input:ShotContext)=>write<S['Draft']>(`${shot(p,d,s)}/context`,'PUT',input),
 capability:(model:string)=>request<S['ModelInputCapability']>(`/api/models/${encodeURIComponent(model)}/input-capability`),
 voices:(model:string)=>request<S['ModelInputCapability']>(`/api/models/${encodeURIComponent(model)}/voices`),
 preview:(p:string,d:string,s:string,input:GenerationRequest)=>write<S['GenerationPreview']>(`${shot(p,d,s)}/generation-preview`,'POST',input),
 image:(p:string,d:string,s:string,input:GenerationRequest)=>write<S['Task']>(`${shot(p,d,s)}/image-tasks`,'POST',input),
 video:(p:string,d:string,s:string,input:GenerationRequest)=>write<S['Task']>(`${shot(p,d,s)}/bound-video-tasks`,'POST',input),
 speech:(p:string,d:string,s:string,u:string,input:GenerationRequest)=>write<S['Task']>(`${shot(p,d,s)}/utterances/${encodeURIComponent(u)}/speech-tasks`,'POST',input),
 takes:(p:string,d:string,s:string)=>request<S['MediaTakeList']>(`${shot(p,d,s)}/media-takes`),
 select:(p:string,d:string,s:string,input:S['SelectMediaTake'])=>write<S['Draft']>(`${shot(p,d,s)}/selected-media`,'PUT',input),
 run:(p:string,d:string,s:string,input:GenerationRequest)=>write<S['GenerationRun']>(`${shot(p,d,s)}/generation-runs`,'POST',input),
 runs:(p:string,d:string,s:string)=>request<S['GenerationRunList']>(`${shot(p,d,s)}/generation-runs`),
 selectRun:(p:string,d:string,s:string,r:string,input:S['RunSelectionWrite'])=>write<S['GenerationRun']>(`${shot(p,d,s)}/generation-runs/${encodeURIComponent(r)}/selected-media`,'PUT',input),
 getRun:(p:string,d:string,s:string,r:string)=>request<S['GenerationRun']>(`${shot(p,d,s)}/generation-runs/${encodeURIComponent(r)}`),
 cancelRun:(p:string,d:string,s:string,r:string)=>write<S['GenerationRun']>(`${shot(p,d,s)}/generation-runs/${encodeURIComponent(r)}/cancel`,'POST',{}),
};
