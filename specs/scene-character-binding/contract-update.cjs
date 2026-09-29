// Phase 1 reproducible OpenAPI contract update; no runtime credentials.
const fs = require('node:fs');
const path = require('node:path');
const yaml = require('../../frontend/node_modules/js-yaml');
const file = path.resolve(__dirname, '../../docs/api/openapi.yaml');
const original = fs.readFileSync(file, 'utf8');
const api = yaml.load(original);
const s = {type:'string'}, integer = {type:'integer',minimum:0};
const ref = name => ({$ref:'#/components/schemas/'+name});
const arr = value => ({type:'array',items:value});
const obj = (properties,required=[]) => ({type:'object',additionalProperties:false,properties,required});
const rev = {...integer,minimum:1};
const schema = {
 ReferenceImage:obj({material_id:s,content_hash:s,label:s,order:integer},['material_id','order']),
 EntityVersion:obj({id:s,kind:{type:'string',enum:['location','character']},name:{...s,minLength:1},description:s,revision:rev,references:arr(ref('ReferenceImage')),voice_profile_id:s,voice_profile_revision:rev,archived:{type:'boolean'}},['id','kind','name','description','revision','references']),
 VoiceProfile:obj({id:s,revision:rev,name:s,connection_id:s,model_id:s,remote_voice_id:s,language:s,instructions:s,archived:{type:'boolean'}},['id','revision','name','connection_id','model_id','remote_voice_id','language']),
 EntityCatalog:obj({catalog_version:integer,entities:arr(ref('EntityVersion')),voice_profiles:arr(ref('VoiceProfile'))},['catalog_version','entities','voice_profiles']),
 EntityWrite:obj({expected_catalog_version:integer,entity:ref('EntityVersion')},['expected_catalog_version','entity']),
 VoiceProfileWrite:obj({expected_catalog_version:integer,voice_profile:ref('VoiceProfile')},['expected_catalog_version','voice_profile']),
 ShotEntityBinding:obj({id:s,entity_id:s,entity_revision:rev,description_override:{...s,description:'Present replaces base description; absent inherits.'},description_supplement:s,selected_material_ids:arr(s),role:{type:'string',enum:['primary','supplement','cast']}},['id','entity_id','entity_revision','role']),
 PromptNode:{oneOf:[obj({type:{const:'text'},text:s},['type','text']),obj({type:{const:'entity_ref'},binding_id:s},['type','binding_id'])]},
 PromptDocument:obj({schema_version:{type:'integer',const:1},nodes:arr(ref('PromptNode'))},['schema_version','nodes']),
 Utterance:obj({id:s,speaker_type:{type:'string',enum:['character','narrator']},speaker_id:s,speaker_revision:rev,text:s,order:integer,emotion:s,voice_profile_id:s,voice_profile_revision:rev,pause_after_ms:integer},['id','speaker_type','text','order']),
 ShotContextWrite:obj({expected_version:integer,entity_bindings:arr(ref('ShotEntityBinding')),prompt_document:ref('PromptDocument'),utterances:arr(ref('Utterance'))},['expected_version','entity_bindings','prompt_document','utterances']),
 GenerationRequest:obj({expected_version:integer,idempotency_key:{...s,minLength:1},model_id:s,path:{type:'string',enum:['reference_video','reference_image_then_video','first_frame_video','speech']},aspect_ratio:s,resolution:s,duration_seconds:{type:'integer',minimum:1},utterance_id:s},['expected_version','idempotency_key','model_id','path']),
 ResolvedEntity:obj({binding_id:s,entity:ref('EntityVersion'),description:s,references:arr(ref('ReferenceImage'))},['binding_id','entity','description','references']),
 GenerationSnapshot:obj({id:s,schema_version:{type:'integer',const:1},project_id:s,draft_id:s,scene_id:s,draft_version:integer,entities:arr(ref('ResolvedEntity')),utterances:arr(ref('Utterance')),voice_profiles:arr(ref('VoiceProfile')),prompt:s,model_id:s,capability_version:s,path:s,aspect_ratio:s,resolution:s,duration_seconds:integer,input_hash:s,created_at:{type:'string',format:'date-time'}},['id','schema_version','project_id','draft_id','scene_id','draft_version','entities','prompt','model_id','capability_version','path','input_hash']),
 BindingIssue:obj({code:s,field:s,message:s,binding_id:s,utterance_id:s,material_id:s},['code','field','message']),
 GenerationPreview:obj({valid:{type:'boolean'},issues:arr(ref('BindingIssue')),snapshot:ref('GenerationSnapshot')},['valid','issues']),
 MediaTake:obj({id:s,kind:{type:'string',enum:['image','speech']},scene_id:s,utterance_id:s,task_id:s,material_id:s,snapshot_id:s,input_hash:s,duration_seconds:{type:'number',minimum:0},stale:{type:'boolean'}},['id','kind','scene_id','task_id','material_id','snapshot_id','input_hash']),
 SelectMediaTake:obj({expected_version:integer,take_id:s,utterance_id:s},['expected_version','take_id']),
 GenerationRun:obj({id:s,scene_id:s,state:{type:'string',enum:['preparing','generating','awaiting_selection','completed','failed','cancelled']},task_ids:arr(s),snapshot_id:s,error:s},['id','scene_id','state','task_ids','snapshot_id']),
 ModelInputCapability:obj({version:s,model_id:s,reference_images:integer,speech:{type:'boolean'},reference_video:{type:'boolean'},first_frame:{type:'boolean'},durations:arr(integer),resolutions:arr(s),voices:arr(obj({id:s,name:s,languages:arr(s)},['id','name','languages']))},['version','model_id','reference_images','speech','reference_video','first_frame']),
 BindingError:obj({code:{type:'string',enum:['entity_revision_conflict','draft_version_conflict','reference_missing','entity_in_use','reference_limit_exceeded','capability_unsupported','speaker_unresolved','voice_unavailable','audio_exceeds_duration','idempotency_key_reused']},message:s,issues:arr(ref('BindingIssue')),catalog:ref('EntityCatalog'),draft:ref('Draft')},['code','message'])
};
const paths = {};
const P='/api/projects/{id}', S=P+'/drafts/{draftId}/scenes/{sceneId}';
function route(url,method,input,output,status='200') {
 const parameters=[...url.matchAll(/\{([^}]+)\}/g)].map(m=>({name:m[1],in:'path',required:true,schema:s}));
 const operation={parameters,'x-implementation-status':'planned',responses:{[status]:{description:'Success',content:{'application/json':{schema:ref(output)}}},'409':{description:'Version conflict or resource in use',content:{'application/json':{schema:ref('BindingError')}}},'422':{description:'Invalid binding or unsupported capability',content:{'application/json':{schema:ref('BindingError')}}}}};
 if(input) operation.requestBody={required:true,content:{'application/json':{schema:ref(input)}}};
 (paths[url]??={})[method]=operation;
}
route(P+'/entities','get',null,'EntityCatalog');route(P+'/entities','post','EntityWrite','EntityCatalog');
route(P+'/entities/{entityId}','patch','EntityWrite','EntityCatalog');
route(P+'/entities/{entityId}/archive','post','EntityWrite','EntityCatalog');
route(P+'/voice-profiles','get',null,'EntityCatalog');route(P+'/voice-profiles','post','VoiceProfileWrite','EntityCatalog');
route(P+'/voice-profiles/{voiceId}','patch','VoiceProfileWrite','EntityCatalog');
route(S+'/context','put','ShotContextWrite','Draft');route(S+'/generation-preview','post','GenerationRequest','GenerationPreview');
route(S+'/image-tasks','post','GenerationRequest','Task','202');
route(S+'/utterances/{utteranceId}/speech-tasks','post','GenerationRequest','Task','202');
route(S+'/media-takes','get',null,'MediaTakeList');
schema.MediaTakeList=obj({takes:arr(ref('MediaTake'))},['takes']);
route(S+'/selected-media','put','SelectMediaTake','Draft');
route(S+'/generation-runs','post','GenerationRequest','GenerationRun','202');
route(S+'/generation-runs/{runId}','get',null,'GenerationRun');
route(S+'/generation-runs/{runId}/cancel','post',null,'GenerationRun');
route('/api/models/{model}/input-capability','get',null,'ModelInputCapability');
route('/api/models/{model}/voices','get',null,'ModelInputCapability');
for(const key of Object.keys(schema)) if(api.components.schemas[key]) throw Error('Already exists: '+key);
for(const key of Object.keys(paths)) if(api.paths[key]) throw Error('Already exists: '+key);
// Preserve existing formatting and avoid rewriting unrelated API definitions.
let next=original.replace(/^components:/m,Object.entries(paths).map(([k,v])=>'  '+JSON.stringify(k)+': '+JSON.stringify(v)+'\n').join('')+'components:');
next+='\n'+Object.entries(schema).map(([k,v])=>'    '+k+': '+JSON.stringify(v)).join('\n')+'\n';
const parsed=yaml.load(next); // Reject duplicate keys / invalid YAML before writing.
for(const [key,value] of Object.entries(schema)) if(!parsed.components.schemas[key]) throw Error('Missing schema '+key);
fs.writeFileSync(file,next);
console.log(JSON.stringify({schemasAdded:Object.keys(schema).length,pathsAdded:Object.keys(paths).length}));
