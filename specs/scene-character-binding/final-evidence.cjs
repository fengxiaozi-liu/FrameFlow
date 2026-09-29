// Read-only capture of the isolated acceptance run. Contains no credentials.
const fs=require('node:fs'),crypto=require('node:crypto');
const base=process.env.FRAMEFLOW_TEST_URL||'http://127.0.0.1:19090';
async function api(p){const r=await fetch(base+p);if(!r.ok)throw Error(`${r.status} ${p}`);return r.json()}
(async()=>{
 const p='20260928162007.038622200',d=await api(`/api/projects/${p}/drafts/final-review`);
 const run=d.generation_runs[0],composition=d.compositions.at(-1);
 if(!composition)throw Error('No completed composition');
 const tasks=await Promise.all([...run.task_ids,composition.task_id].map(id=>api('/api/tasks/'+id)));
 if(tasks.some(t=>t.status!=='succeeded'))throw Error('Acceptance task incomplete');
 const files=['frontend/src/components/studio/BindingWorkbench.vue','frontend/src/components/studio/DialogueVoicePanel.vue','frontend/src/components/studio/MentionPromptEditor.vue','frontend/src/views/StudioView.vue','frontend/src/styles/studio.css','backend/internal/application/generation_runs.go','backend/internal/application/generation_compiler.go','backend/internal/domain/processor/processor.go'];
 const record={verified_at:new Date().toISOString(),project:p,draft:d.id,run:run.id,state:run.state,composition:composition.id,tasks:tasks.map(t=>({id:t.id,kind:t.kind,status:t.status,result_url:t.result_url,remote_task_id:t.remote_task_id,input_hash:t.input_hash,voices:t.snapshot?.speech?.map(s=>s.voice),reference_count:t.snapshot?.entities?.reduce((n,e)=>n+e.references.length,0)})),source_sha256:Object.fromEntries(files.map(p=>[p,crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex')]))};
 fs.writeFileSync('specs/scene-character-binding/evidence/final-acceptance.json',JSON.stringify(record,null,2));
 const r=await fetch(base+composition.local_media_path);if(!r.ok)throw Error('Download failed');fs.writeFileSync('specs/scene-character-binding/evidence/final-video.mp4',Buffer.from(await r.arrayBuffer()));
 console.log(JSON.stringify({state:run.state,tasks:tasks.map(t=>({kind:t.kind,status:t.status})),video:'evidence/final-video.mp4'}));
})().catch(e=>{console.error(e.message);process.exitCode=1});
