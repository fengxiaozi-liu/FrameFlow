import {test,expect,type APIRequestContext,type Page} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import AxeBuilder from '@axe-core/playwright';

test('cancelling file selection keeps the entity drawer and unsaved input',async({page,request})=>{
 const s=await seed(request);await page.goto(`${s.base}/shots/shot`);
 for(const kind of ['场景','人物']){
  await page.getByRole('button',{name:`＋ 添加${kind}`,exact:true}).click();
  const drawer=page.getByRole('dialog');
  await drawer.getByLabel('名称',{exact:true}).fill(`待保存${kind}`);
  await drawer.getByLabel('参考描述',{exact:true}).fill('取消选图后仍需保留的描述');
  const upload=drawer.getByLabel('上传参考图片',{exact:true});
  // The native picker dispatches a bubbling cancel event on its file input.
  await upload.dispatchEvent('cancel',{bubbles:true,cancelable:false});
  await expect(drawer).toBeVisible();
  await expect(drawer.getByLabel('名称',{exact:true})).toHaveValue(`待保存${kind}`);
  await expect(drawer.getByLabel('参考描述',{exact:true})).toHaveValue('取消选图后仍需保留的描述');
  await upload.setInputFiles('public/home/cafe.png');
  await upload.dispatchEvent('cancel',{bubbles:true,cancelable:false});
  await expect(drawer.locator('.reference-upload-preview')).toContainText('cafe.png');
  await expect(drawer.locator('.reference-pending-upload').getByRole('checkbox')).toHaveCount(0);
  await expect(drawer.getByRole('button',{name:'上传到素材库',exact:true})).toBeEnabled();
  await drawer.getByRole('button',{name:'取消上传',exact:true}).click();
  await expect(drawer).toBeVisible();
  if(kind==='场景'){
   await drawer.getByLabel('名称',{exact:true}).focus();await page.keyboard.press('Escape');
  }else{
   await drawer.getByRole('button',{name:'取消',exact:true}).click();
  }
  await expect(drawer).toHaveCount(0);
  await expect(page.getByRole('button',{name:`＋ 添加${kind}`,exact:true})).toBeFocused();
 }
});

test('new shot opens detail, keeps a single name field and recovers creation after lost response',async({page,request},testInfo)=>{
 const s=await seed(request);await page.goto(`${s.base}/shots`);
 await expect(page.getByLabel('新分镜名称')).toHaveCount(0);
 const ids:string[]=[];
 await page.route('**/scenes',async route=>{
  if(route.request().method()!=='POST'){await route.continue();return}
  ids.push(route.request().postDataJSON().creation_id);
  if(ids.length===1){await route.fetch();await route.abort('failed')}else await route.continue();
 });
 await page.getByRole('button',{name:'＋ 新增分镜',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();
 await page.reload();await expect(page.locator('.shot-list-row')).toHaveCount(2);
 await page.getByRole('button',{name:'＋ 新增分镜',exact:true}).click();await expect(page).toHaveURL(/\/shots\/scene-created-.+\?created=1$/);
 const name=page.getByLabel('分镜名称',{exact:true});await expect(name).toBeFocused();await expect(name).toHaveCount(1);await expect(page.getByLabel('分镜标题',{exact:true})).toHaveCount(0);
 await name.fill('晨光中的咖啡馆');await expect(page.getByRole('status').first()).toHaveText('已保存');
 await page.screenshot({path:testInfo.outputPath('creation-detail-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.screenshot({path:testInfo.outputPath('creation-detail-mobile.png'),fullPage:true});
 await page.getByRole('link',{name:'分镜列表',exact:true}).click();await expect(page.locator('.shot-list-row')).toHaveCount(2);await expect(page.locator('.shot-list-row').last()).toContainText('晨光中的咖啡馆');
 expect(ids).toHaveLength(2);expect(ids[0]).toBe(ids[1]);
});

test('legacy supplements move into editable description once and survive reload',async({page,request})=>{
 const s=await seed(request);
 const entity={id:'legacy-person',kind:'character',name:'林夏',description:'蓝外套',revision:1,references:[]};
 expect((await request.post(`/api/projects/${s.p}/entities`,{data:{expected_catalog_version:0,entity}})).ok()).toBeTruthy();
 const draft=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();
 s.d+='-legacy';s.base="/studio/"+s.p+"/"+s.d;draft.id=s.d;draft.version=0;
 draft.story.scenes[0].entity_bindings=[{id:'old-binding',entity_id:entity.id,entity_revision:1,role:'cast',description_supplement:'雨中湿发'}];
 expect((await request.post(`/api/projects/${s.p}/drafts`,{data:draft})).ok()).toBeTruthy();
 await page.goto(`${s.base}/shots/shot`);await expect(page.locator('.migration-note')).toContainText('已合并到场景描述');await expect(page.getByLabel('本镜头补充')).toHaveCount(0);
 await expect(page.getByLabel('镜头描述文本')).toContainText('：雨中湿发');
 const prompt=page.getByLabel('镜头描述文本');await prompt.press('Control+End');for(let i=0;i<4;i++)await prompt.press('Backspace');await prompt.pressSequentially('已擦干头发');await expect(page.getByRole('status').first()).toHaveText('已保存');
 await page.reload();await expect(page.getByLabel('镜头描述文本')).toContainText('：已擦干头发');await expect(page.getByLabel('镜头描述文本')).toHaveCount(1);
 const saved=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(saved.story.scenes[0].entity_bindings[0].description_supplement).toBeUndefined();expect(saved.story.scenes[0].visual_prompt).toContain('林夏：已擦干头发');expect(saved.story.scenes[0].visual_prompt).not.toContain('雨中湿发');
});

test('character selects existing voice materials, retries save, edits and clears without generation',async({page,request},testInfo)=>{
 const s=await seed(request);
 const voiceIDs=[randomUUID(),randomUUID()];
 for(const [i,id] of voiceIDs.entries())expect((await request.post('/api/materials',{data:{id,kind:'voice',name:i?'沉稳男声':'温柔女声',url:'/media/test-voice.wav',media:{format:'audio/wav'}}})).ok()).toBeTruthy();
 await page.goto(`${s.base}/shots/shot`);await page.getByRole('button',{name:'＋ 添加人物',exact:true}).click();
 const drawer=page.getByRole('dialog');await drawer.getByLabel('名称',{exact:true}).fill('林夏');await drawer.getByLabel('参考描述',{exact:true}).fill('短发，浅色风衣');
 let voiceRequests=0,entityWrites=0;
 await page.route('**/voices',async route=>{voiceRequests++;await route.continue()});
 await page.route('**/voice-profiles',async route=>{voiceRequests++;await route.continue()});
 await page.route('**/entities',async route=>{if(route.request().method()!=='POST'){await route.continue();return}entityWrites++;if(entityWrites===1)await route.fulfill({status:503,json:{message:'人物保存暂时失败'}});else await route.continue()});
 await expect(drawer.getByLabel('语音模型',{exact:true})).toHaveCount(0);
 await drawer.getByLabel('人物音色素材',{exact:true}).selectOption(voiceIDs[0]!);await expect(drawer.getByLabel('试听人物音色')).toHaveAttribute('src',/test-voice.wav/);
 await drawer.locator('.character-voice-settings').scrollIntoViewIfNeeded();await page.screenshot({path:testInfo.outputPath('character-voice-library-desktop.png')});
 const axe=await new AxeBuilder({page}).include('.entity-editor-drawer').withTags(['wcag2a','wcag2aa']).analyze();expect(axe.violations).toEqual([]);
 await page.setViewportSize({width:390,height:844});await drawer.getByLabel('人物音色素材',{exact:true}).scrollIntoViewIfNeeded();await page.screenshot({path:testInfo.outputPath('character-voice-library-mobile.png')});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer.getByRole('alert').first()).toContainText('人物保存暂时失败');await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);
 await page.reload();await expect(page.locator('.entity-card')).toContainText('温柔女声');await page.getByRole('button',{name:'编辑项目设定',exact:true}).click();await expect(drawer.getByLabel('人物音色素材',{exact:true})).toHaveValue(voiceIDs[0]!);
 await drawer.getByLabel('人物音色素材',{exact:true}).selectOption(voiceIDs[1]!);await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await expect(page.locator('.entity-card')).toContainText('沉稳男声');
 await page.getByRole('button',{name:'编辑项目设定',exact:true}).click();await drawer.getByRole('button',{name:'清除音色'}).click();await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await page.reload();await expect(page.locator('.entity-card')).toContainText('未设置');
 const catalog=await(await request.get(`/api/projects/${s.p}/entities`)).json();expect(catalog.voice_profiles||[]).toHaveLength(0);expect(catalog.entities).toHaveLength(3);expect(catalog.entities[0].voice_material_id).toBe(voiceIDs[0]);expect(catalog.entities[2].voice_material_id).toBeUndefined();expect(voiceRequests).toBe(0);
 expect((await request.delete(`/api/materials/${voiceIDs[0]}`)).status()).toBe(409);
 await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();await expect(drawer.getByLabel('人物音色素材',{exact:true})).toHaveCount(0);
});

test('drawer retains failed edits, retries binding save and restores focus on cancel',async({page,request},testInfo)=>{
 const s=await seed(request);await page.goto(`${s.base}/shots/shot`);
 const add=page.getByRole('button',{name:'＋ 添加人物',exact:true});await add.click();
 const drawer=page.getByRole('dialog',{name:'添加人物'});
 await expect(drawer.getByLabel('名称',{exact:true})).toBeFocused();
 await drawer.getByLabel('名称',{exact:true}).fill('周晴');await drawer.getByLabel('参考描述',{exact:true}).fill('黑色长发，米色风衣。');
 await page.screenshot({path:testInfo.outputPath('reference-drawer-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:testInfo.outputPath('reference-drawer-mobile.png'),fullPage:true});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await drawer.getByRole('button',{name:'取消',exact:true}).click();await expect(drawer).toHaveCount(0);await expect(add).toBeFocused();await expect(page.locator('.entity-card')).toHaveCount(0);
 await add.click();await page.keyboard.press('Escape');await expect(drawer).toHaveCount(0);
 await add.click();await drawer.getByLabel('名称',{exact:true}).fill('保留输入');await drawer.getByLabel('参考描述',{exact:true}).fill('失败后仍保留描述');
 let entityWrites=0,contextWrites=0;
 await page.route('**/entities',async route=>{if(route.request().method()!=='POST'){await route.continue();return}entityWrites++;if(entityWrites===1)await route.fulfill({status:503,json:{message:'保存暂时失败'}});else await route.continue()});
 await page.route('**/context',async route=>{contextWrites++;if(contextWrites===1)await route.fulfill({status:503,json:{message:'引用暂时失败'}});else await route.continue()});
 await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer.getByRole('alert')).toBeVisible();await expect(drawer.getByLabel('名称',{exact:true})).toHaveValue('保留输入');await expect(drawer.getByLabel('参考描述',{exact:true})).toHaveValue('失败后仍保留描述');
 await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer.getByRole('button',{name:'重试引用保存'})).toBeVisible();await drawer.getByRole('button',{name:'重试引用保存'}).click();await expect(drawer).toHaveCount(0);await expect(page.locator('.entity-card')).toHaveCount(1);expect(entityWrites).toBe(2);
 await page.reload();await expect(page.locator('.entity-card')).toContainText('保留输入');
});
async function seed(request:APIRequestContext){
 const id='e2e-'+randomUUID();
 const c=await request.post('/api/connections',{data:{id,name:'本地验收连接',vendor:'bailian',region:'cn',base_url:'https://example.invalid'}});expect(c.ok()).toBeTruthy();
 for(const [remote,name,cap] of [['qwen-image-2.0-pro','验收分镜图','image'],['wan2.7-r2v','验收视频','video'],['qwen3-tts-flash','验收配音','speech']]){const r=await request.post(`/api/connections/${id}/models`,{data:{model_id:remote,name,capabilities:[cap]}});expect(r.ok()).toBeTruthy()}
 const p=await(await request.post('/api/projects',{data:{name:'分镜工作区验收'}})).json();const d='draft-'+randomUUID();
 const r=await request.post(`/api/projects/${p.id}/drafts`,{data:{id:d,name:'雨夜故事',story:{body:'雨夜咖啡馆',summary:'',scenes:[{id:'shot',order:1,title:'雨夜 · 走进咖啡馆',visual_prompt:'中景，镜头缓慢推进，林夏推开玻璃门。',narration:'',duration_seconds:4}]}}});expect(r.ok()).toBeTruthy();
 return {p:p.id as string,d,base:`/studio/${p.id}/${d}`,image:`${id}:qwen-image-2.0-pro`,video:`${id}:wan2.7-r2v`};
}
async function configure(page:Page,s:Awaited<ReturnType<typeof seed>>){
 await page.getByRole('combobox',{name:'视频模型',exact:true}).selectOption(s.video);await expect(page.getByRole('status').first()).toHaveText('已保存');
}

test('failed run deletion retains errors, removes children, persists after reload and works on mobile',async({page,request},testInfo)=>{
 test.setTimeout(60000);
 const s=await seed(request);
 await page.goto(`${s.base}/shots/shot`);await configure(page,s);
 await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();
 const drawer=page.getByRole('dialog');await drawer.getByLabel('名称',{exact:true}).fill('失败记录删除验收');
 await drawer.locator('input[type=file]').setInputFiles('public/home/cafe.png');
 await drawer.getByRole('button',{name:'使用这张图片',exact:true}).click();
 await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);
 for(let i=1;i<=2;i++){
  await page.getByRole('button',{name:'生成视频',exact:true}).click();
  await expect(page.locator('.generation-run-row')).toHaveCount(i);
  await expect(page.locator('.generation-run-row').nth(i-1).getByRole('button',{name:`删除任务 ${String(i).padStart(3,'0')}`,exact:true})).toBeVisible({timeout:15000});
 }
 const endpoint=`/api/projects/${s.p}/drafts/${s.d}/scenes/shot/generation-runs`;
 const before=(await(await request.get(endpoint)).json()).runs;
 const first=before[0].id as string,second=before[1].id as string;
 page.once('dialog',dialog=>dialog.dismiss());await page.getByRole('button',{name:'删除任务 001',exact:true}).click();await expect(page.locator('.generation-run-row')).toHaveCount(2);
 let reject=true;
 await page.route(`**/generation-runs/${first}`,async route=>{
  if(route.request().method()==='DELETE'&&reject){reject=false;await route.fulfill({status:503,json:{error:{message:'删除暂不可用，请重试'}}})}else await route.continue();
 });
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'删除任务 001',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:'删除暂不可用'})).toBeVisible();await expect(page.locator('.generation-run-row')).toHaveCount(2);
 await page.screenshot({path:testInfo.outputPath('failed-run-delete-desktop.png'),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:testInfo.outputPath('failed-run-delete-mobile.png'),fullPage:true});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'删除任务 001',exact:true}).click();
 await expect(page.locator('.generation-run-row')).toHaveCount(1);await expect(page.getByRole('heading',{name:'生成记录',exact:true})).toBeFocused();
 expect((await(await request.get(endpoint)).json()).runs.map((r:{id:string})=>r.id)).toEqual([second]);
 expect((await(await request.get(`/api/tasks?project_id=${s.p}&draft_id=${s.d}`)).json()).tasks.some((t:{run_id?:string})=>t.run_id===first)).toBe(false);
 await page.reload();await expect(page.locator('.generation-run-row')).toHaveCount(1);
 await page.getByRole('link',{name:'进入任务 →'}).click();await expect(page.getByRole('button',{name:'删除任务',exact:true})).toBeVisible();
 page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'删除任务',exact:true}).focus();await page.keyboard.press('Enter');
 await expect(page).toHaveURL(`${s.base}/shots/shot?deleted=1`);await expect(page.locator('.generation-run-row')).toHaveCount(0);await expect(page.getByRole('heading',{name:'生成记录',exact:true})).toBeFocused();
 await page.reload();await expect(page.getByText('暂无制作任务', {exact:false})).toBeVisible();
 // The task center uses the same run deletion semantics.
 await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page.getByRole('button',{name:'删除任务 001',exact:true})).toBeVisible({timeout:15000});
 const latest=(await(await request.get(endpoint)).json()).runs[0];
 await page.goto('/tasks');const card=page.locator('.task-detail').filter({hasText:latest.task_ids[0]});await expect(card).toHaveCount(1);
 page.once('dialog',dialog=>dialog.accept());await card.getByRole('button',{name:'删除任务',exact:true}).click();await expect(card).toHaveCount(0);
 expect((await(await request.get(endpoint)).json()).runs).toHaveLength(0);
});
test('list, configuration order, frozen task details, failure retry and another explicit generation',async({page,request},testInfo)=>{
 const s=await seed(request);await page.goto(`${s.base}/shots`);await expect(page.locator('.shot-list-row')).toHaveCount(1);await expect(page.locator('.binding-workbench')).toHaveCount(0);await page.getByRole('link',{name:'进入详情 →'}).click();
 await expect(page.getByRole('region',{name:'任务结构'})).toContainText('暂无制作任务');await expect(page.getByRole('button',{name:'生成视频',exact:true})).toHaveCount(1);
 const order=await page.locator('.shot-configuration').evaluate(el=>Array.from(el.querySelectorAll('h3')).map(n=>n.textContent));expect(order.indexOf('视频设置')).toBeGreaterThan(order.indexOf('场景描述'));
 await configure(page,s);
 for(const [kind,name,desc] of [['场景','雨夜咖啡馆','玻璃门、暖色灯光与窗外雨幕'],['人物','林夏','短发，浅色外套'],['人物','陈默','深色衬衫，靠窗坐着']]){
  await page.getByRole('button',{name:`＋ 添加${kind}`,exact:true}).click();const form=page.locator('.entity-form');
  await form.getByLabel('名称',{exact:true}).fill(name);await form.getByLabel('参考描述',{exact:true}).fill(desc);
  const accessibility=await new AxeBuilder({page}).include('.entity-form').withTags(['wcag2a','wcag2aa']).analyze();expect(accessibility.violations).toEqual([]);
  await form.screenshot({path:testInfo.outputPath(`entity-form-${name}-desktop.png`)});
  await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await form.screenshot({path:testInfo.outputPath(`entity-form-${name}-mobile.png`)});await page.setViewportSize({width:1440,height:1000});
  await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);await expect(page.getByRole('status').first()).toHaveText('已保存');
 }
 const card=page.locator('.entity-card').nth(1);await card.locator('summary').click();const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');await card.locator('input[type=file]').setInputFiles({name:'林夏.png',mimeType:'image/png',buffer:png});await expect(card.locator('.reference-tile')).toHaveCount(1);await expect(page.getByRole('status').first()).toHaveText('已保存');await card.locator('input[type=file]').first().setInputFiles({name:'林夏替换.png',mimeType:'image/png',buffer:png});await expect(card.locator('.reference-tile')).toHaveCount(1);await expect(page.getByRole('status').first()).toHaveText('已保存');
 await card.getByRole('button',{name:'编辑项目设定'}).click();await expect(page.getByRole('dialog').locator('.reference-tile')).toHaveCount(1);await page.getByRole('dialog').getByRole('button',{name:'保存并引用'}).click();await expect(page.getByRole('dialog')).toHaveCount(0);await expect(card.locator('.reference-tile')).toHaveCount(1);await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:testInfo.outputPath('shot-detail-desktop.png'),fullPage:true});await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:testInfo.outputPath('shot-detail-mobile.png'),fullPage:true});await page.setViewportSize({width:1440,height:1000});await page.reload();await expect(page.getByRole('combobox',{name:'视频模型',exact:true})).toHaveValue(s.video);
 await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(`${s.base}/shots/shot`);await expect(page.locator('.generation-run-row')).toHaveCount(1);await page.getByRole('link',{name:'进入任务 →'}).first().click();await expect(page.getByRole('heading',{name:'分镜图',exact:true})).toHaveCount(0);await expect(page.locator('.run-step')).toHaveCount(1);
 await expect(page.getByRole('heading',{name:'制作任务详情'})).toBeVisible();await expect(page.getByRole('region',{name:'本次生成设定'})).toContainText('4 秒');await expect(page.getByRole('button',{name:'按原输入重试'})).toBeVisible({timeout:15000});
 await page.screenshot({path:testInfo.outputPath('video-task-detail-desktop.png'),fullPage:true});await page.getByRole('button',{name:'按原输入重试'}).click();await expect(page.getByRole('button',{name:'按原输入重试'})).toBeVisible({timeout:15000});
 await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row')).toHaveCount(1);await page.getByLabel('镜头描述文本').fill('第二次生成的新镜头描述');await expect(page.getByRole('status').first()).toHaveText('已保存');
 await expect(page.getByRole('button',{name:'生成视频',exact:true})).toBeEnabled({timeout:15000});await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(`${s.base}/shots/shot`);await expect(page.locator('.generation-run-row')).toHaveCount(2);
 await page.getByRole('link',{name:'进入任务 →'}).first().click();await page.getByText('提交时的场景描述与引用素材').click();await expect(page.locator('.snapshot-prompt')).toContainText('中景，镜头缓慢推进');await expect(page.locator('.snapshot-prompt')).not.toContainText('第二次');
 await page.getByRole('link',{name:'分镜列表',exact:true}).click();await expect(page.locator('.shot-list-row')).toContainText('2 个制作任务');await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();await page.getByRole('link',{name:'进入详情 →'}).click();await page.getByRole('link',{name:'进入任务 →'}).nth(1).click();await page.getByRole('button',{name:'取消制作任务'}).click();await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row').nth(1)).toContainText('已取消');
});
test('lost submit response recovers same run without duplication',async({page,request})=>{
 const s=await seed(request);await page.goto(`${s.base}/shots/shot`);await configure(page,s);await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();const form=page.getByRole('dialog');await form.getByLabel('名称',{exact:true}).fill('参考场景');await form.locator('input[type=file]').setInputFiles({name:'场景.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64')});await form.getByRole('button',{name:'使用这张图片',exact:true}).click();await expect(form.locator('.reference-tile')).toHaveCount(1);await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);const keys:string[]=[];
 await page.route('**/generation-runs',async route=>{if(route.request().method()!=='POST'){await route.continue();return}keys.push(route.request().postDataJSON().idempotency_key);if(keys.length===1){await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(`${s.base}/shots/shot`);await expect(page.locator('.generation-run-row')).toHaveCount(1);expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1]);await page.reload();await expect(page.locator('.generation-run-row')).toHaveCount(1);
});


test('drawer image generation recovers submission, retries failure and adds classified references explicitly',async({page,request},testInfo)=>{
 const s=await seed(request);const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');
 await page.route('**/media/reference-demo.png',r=>r.fulfill({contentType:'image/png',body:png}));
 const generatedId='drawer-'+randomUUID();let body:any,status='failed',submits=0,retries=0,cancels=0;
 await page.route('**/reference-image-tasks',async route=>{submits++;const input=route.request().postDataJSON();if(submits===1){body=input;await route.abort('failed');return}expect(input).toEqual(body);await route.fulfill({status:202,json:{id:generatedId,status,kind:'image',progress:40}})});
 await page.route(`**/api/tasks/${generatedId}`,route=>route.fulfill({json:{id:generatedId,status,kind:'image',progress:status==='succeeded'?100:40,error:status==='failed'?'模型暂时不可用':''}}));
 await page.route(`**/api/tasks/${generatedId}/retry`,route=>{retries++;status='running';return route.fulfill({json:{id:generatedId,status,kind:'image',progress:40}})});
 await page.route(`**/api/tasks/${generatedId}/cancel`,route=>{cancels++;status='cancelled';return route.fulfill({json:{id:generatedId,status,kind:'image',progress:40}})});
 const material={id:`result-${generatedId}`,name:'雨夜咖啡馆 · 生成参考图',kind:'scene',url:'/media/reference-demo.png',reference_only:true,project_id:s.p};
 expect((await request.post('/api/materials',{data:material})).ok()).toBeTruthy();
 await page.goto(`${s.base}/shots/shot`);await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();const drawer=page.getByRole('dialog');await drawer.getByLabel('名称',{exact:true}).fill('雨夜咖啡馆');await drawer.getByLabel('参考描述',{exact:true}).fill('雨夜，玻璃窗，室内暖色灯光');await drawer.getByRole('button',{name:'图片生成',exact:true}).click();
 const generator=drawer.getByRole('region',{name:'图片生成设置'});await generator.getByLabel('生图模型').selectOption(s.image);await generator.getByRole('button',{name:'使用参考描述'}).click();await expect(generator.getByLabel('生图提示词')).toHaveValue('雨夜，玻璃窗，室内暖色灯光');
 await generator.getByRole('button',{name:'生成图片',exact:true}).click();await generator.getByRole('button',{name:'恢复提交',exact:true}).click();await expect(generator).toContainText('模型暂时不可用');expect(body.kind).toBe('scene');expect(body.aspect_ratio).toBe('16:9');
 await generator.getByRole('button',{name:'按原设置重试'}).click();await generator.getByRole('button',{name:'取消生成'}).click();await expect(generator.getByRole('status')).toContainText('已取消');await generator.getByRole('button',{name:'按原设置重试'}).click();status='succeeded';
 await expect(generator.getByRole('button',{name:'使用这张图片',exact:true})).toBeVisible();await expect(drawer.locator('.reference-tile')).toHaveCount(0);
 await generator.scrollIntoViewIfNeeded();await page.screenshot({path:testInfo.outputPath('reference-image-generator-desktop.png')});
 const axe=await new AxeBuilder({page}).include('.entity-editor-drawer').withTags(['wcag2a','wcag2aa']).analyze();expect(axe.violations).toEqual([]);
 await page.setViewportSize({width:390,height:844});await generator.getByRole('button',{name:'使用这张图片',exact:true}).scrollIntoViewIfNeeded();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.screenshot({path:testInfo.outputPath('reference-image-generator-mobile.png')});
 await generator.getByRole('button',{name:'使用这张图片',exact:true}).click();await expect(drawer.locator('.reference-tile')).toHaveCount(1);await expect(generator.getByRole('button',{name:'已加入参考图',exact:true})).toBeDisabled();await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await page.reload();await expect(page.locator('.entity-card .reference-tile')).toHaveCount(1);
 expect((await(await request.get('/api/materials')).json()).materials.some((m:any)=>m.id===material.id)).toBe(false);expect(submits).toBe(2);expect(retries).toBe(2);expect(cancels).toBe(1);const catalog=await(await request.get(`/api/projects/${s.p}/entities`)).json();expect(catalog.entities[0].references[0].material_id).toBe(material.id);const draft=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(draft.generation_runs||[]).toHaveLength(0);
 await page.getByRole('button',{name:'＋ 添加人物',exact:true}).click();await drawer.getByRole('button',{name:'从素材库选择',exact:true}).first().click();await expect(drawer.getByRole('button',{name:`选择参考图 ${material.name}`,exact:true})).toHaveCount(0);await drawer.getByRole('button',{name:'图片生成',exact:true}).click();await expect(drawer.getByLabel('图片画幅')).toHaveValue('1:1');await drawer.getByRole('button',{name:'取消',exact:true}).click();
});


test('character drawer can select library images and explains unavailable generation models',async({page,request})=>{
 const s=await seed(request);const id='character-'+randomUUID();const image={id,kind:'character',name:'林夏已有参考',url:'/media/existing-person.png'};
 expect((await request.post('/api/materials',{data:image})).ok()).toBeTruthy();
 await page.route('**/api/models?*',r=>r.fulfill({json:{models:[]}}));
 await page.goto(`${s.base}/shots/shot`);await page.getByRole('button',{name:'＋ 添加人物',exact:true}).click();const drawer=page.getByRole('dialog');await drawer.getByLabel('名称',{exact:true}).fill('林夏');await drawer.getByRole('button',{name:'图片生成',exact:true}).click();await expect(drawer).toContainText('暂无配置就绪的生图模型');await expect(drawer.getByRole('button',{name:'生成图片',exact:true})).toBeDisabled();
 await drawer.getByRole('button',{name:'从素材库选择',exact:true}).click();const select=drawer.getByRole('button',{name:`选择参考图 ${image.name}`,exact:true});await select.click();await expect(select).toBeDisabled();await expect(drawer.locator('.reference-tile')).toHaveCount(1);await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await page.reload();await expect(page.locator('.entity-card .reference-tile')).toHaveCount(1);const catalog=await(await request.get(`/api/projects/${s.p}/entities`)).json();expect(catalog.entities[0].kind).toBe('character');expect(catalog.entities[0].references[0].material_id).toBe(id);
});

test('reference upload uses explicit library button, survives reload and retries without duplicate uploads',async({page,request},testInfo)=>{
 const s=await seed(request);await page.goto(`${s.base}/shots/shot`);
 const ids:string[]=[];
 for(const publish of [false,true]){
  await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();const drawer=page.getByRole('dialog');
  await drawer.getByLabel('名称',{exact:true}).fill(publish?'共享咖啡馆':'仅作参考的咖啡馆');
  await expect(drawer.locator('.upload-tile')).toHaveCount(0);
  await drawer.getByLabel('上传参考图片').setInputFiles('public/home/cafe.png');
  const upload=drawer.locator('.reference-pending-upload');await expect(upload.getByRole('checkbox')).toHaveCount(0);
  await expect(drawer.locator('.reference-tile')).toHaveCount(0);await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer.getByRole('alert')).toContainText('请先使用或取消');
  let uploadRequests=0;const countUpload=(r:any)=>{if(r.url().endsWith('/api/materials/upload')&&r.method()==='POST')uploadRequests++};page.on('request',countUpload);
  const uploaded=page.waitForResponse(r=>r.url().endsWith('/api/materials/upload')&&r.request().method()==='POST');
  if(publish){
   let promotions=0;await page.route('**/api/materials/*/library',async r=>{promotions++;if(promotions===1)await r.fulfill({status:503,json:{error:{message:'入库暂时失败，请重试'}}});else await r.continue()});
   await upload.getByRole('button',{name:'上传到素材库',exact:true}).click();await expect(drawer.getByRole('alert').last()).toContainText('入库暂时失败');
   await expect(drawer.locator('.reference-tile')).toHaveCount(0);
   await upload.getByRole('button',{name:'上传到素材库',exact:true}).click();await expect(upload.getByRole('button',{name:'已上传到素材库',exact:true})).toBeDisabled();
   await expect(drawer.locator('.reference-tile')).toHaveCount(0);expect(promotions).toBe(2);
   const m=await(await uploaded).json();expect((await(await request.get('/api/materials')).json()).materials.filter((v:any)=>v.id===m.id)).toHaveLength(1);
  }
  await drawer.getByRole('button',{name:'图片生成',exact:true}).click();await drawer.getByRole('button',{name:'从素材库选择',exact:true}).click();
  await expect(upload).toBeVisible();if(publish)await expect(upload.getByRole('button',{name:'已上传到素材库',exact:true})).toBeDisabled();
  if(!publish){await upload.scrollIntoViewIfNeeded();await page.screenshot({path:testInfo.outputPath('optional-library-desktop.png')});expect((await new AxeBuilder({page}).include('.entity-editor-drawer').withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([])}
  await upload.getByRole('button',{name:'使用这张图片',exact:true}).click();const m=await(await uploaded).json();ids.push(m.id);
  await expect(drawer.locator('.reference-tile')).toHaveCount(1);await expect(upload).toHaveCount(0);
  expect(uploadRequests).toBe(1);page.off('request',countUpload);
  const library=await(await request.get('/api/materials')).json();expect(library.materials.some((v:any)=>v.id===m.id)).toBe(publish);
  await drawer.getByRole('button',{name:'图片生成',exact:true}).click();await expect(drawer.locator('.reference-tile')).toHaveCount(1);
  await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await page.reload();
  await expect(page.locator('.entity-card .reference-tile')).toHaveCount(ids.length);await expect(page.locator('.entity-card .entity-cover').first()).toBeVisible();
 }
 await page.locator('.entity-card').first().getByRole('button',{name:'编辑项目设定'}).click();const drawer=page.getByRole('dialog');await expect(drawer.locator('.reference-tile')).toHaveCount(1);
 await page.setViewportSize({width:390,height:844});await drawer.locator('.reference-selected-summary').scrollIntoViewIfNeeded();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.screenshot({path:testInfo.outputPath('optional-library-mobile.png')});
 await drawer.getByRole('button',{name:'移除参考图绑定'}).click();await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);
 expect((await request.get(`/api/materials/${ids[0]}`)).ok()).toBeTruthy();expect((await(await request.get('/api/materials')).json()).materials.some((m:any)=>m.id===ids[1])).toBeTruthy();
});

test('generated reference uploads independently from adoption and recovers publication failure',async({page,request},testInfo)=>{
 const s=await seed(request);const id='optional-'+randomUUID(),materialID='result-'+id;
 const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');
 await page.route('**/media/optional.png',r=>r.fulfill({contentType:'image/png',body:png}));
 expect((await request.post('/api/materials',{data:{id:materialID,name:'生成参考',kind:'character',url:'/media/optional.png',reference_only:true,project_id:s.p}})).ok()).toBeTruthy();
 const task={id,status:'succeeded',kind:'image',progress:100};
 await page.route('**/reference-image-tasks',r=>r.fulfill({status:202,json:task}));await page.route(`**/api/tasks/${id}`,r=>r.fulfill({json:task}));
 let promotions=0;await page.route(`**/api/materials/${materialID}/library`,async r=>{promotions++;if(promotions===1)await r.fulfill({status:503,json:{error:{message:'入库暂时失败，请重试'}}});else await r.continue()});
 await page.goto(`${s.base}/shots/shot`);await page.getByRole('button',{name:'＋ 添加人物',exact:true}).click();const drawer=page.getByRole('dialog');await drawer.getByLabel('名称',{exact:true}).fill('林夏');await drawer.getByLabel('参考描述',{exact:true}).fill('米色风衣');await drawer.getByRole('button',{name:'图片生成',exact:true}).click();const generator=drawer.getByRole('region',{name:'图片生成设置'});await generator.getByLabel('生图模型').selectOption(s.image);await generator.getByRole('button',{name:'生成图片',exact:true}).click();
 await expect(generator.getByRole('checkbox')).toHaveCount(0);await expect(generator.getByRole('button',{name:'上传到素材库',exact:true})).toBeEnabled();await expect(drawer.locator('.reference-tile')).toHaveCount(0);
 expect((await(await request.get('/api/materials')).json()).materials.some((m:any)=>m.id===materialID)).toBeFalsy();
 await generator.getByRole('button',{name:'上传到素材库',exact:true}).click();await expect(generator.getByRole('alert')).toContainText('入库暂时失败');await expect(drawer.locator('.reference-tile')).toHaveCount(0);
 await generator.getByRole('button',{name:'上传到素材库',exact:true}).click();
 await expect(generator.getByRole('button',{name:'已上传到素材库',exact:true})).toBeDisabled();
 await expect(drawer.locator('.reference-tile')).toHaveCount(0);
 await page.screenshot({path:testInfo.outputPath('generated-library-button.png')});
 await page.setViewportSize({width:390,height:844});
 await generator.getByRole('button',{name:'使用这张图片'}).scrollIntoViewIfNeeded();
 await page.screenshot({path:testInfo.outputPath('generated-library-button-mobile.png')});
 await generator.getByRole('button',{name:'使用这张图片'}).click();await expect(drawer.locator('.reference-tile')).toHaveCount(1);await expect(generator.getByRole('button',{name:'已加入参考图'})).toBeDisabled();expect(promotions).toBe(2);
 expect((await(await request.get('/api/materials')).json()).materials.filter((m:any)=>m.id===materialID)).toHaveLength(1);
 await drawer.getByRole('button',{name:'保存并引用'}).click();await expect(drawer).toHaveCount(0);await page.reload();await expect(page.locator('.entity-card .reference-tile')).toHaveCount(1);
});


test('new entities have editable defaults that persist and avoid existing names', async ({page, request}, testInfo) => {
 const s = await seed(request);
 await page.goto(`${s.base}/shots/shot`);
 const drawer = page.getByRole('dialog');
 for (const [kind, expected, renamed] of [
   ['场景', '场景 1', ''],
   ['场景', '场景 2', '雨夜咖啡馆'],
   ['人物', '人物 1', ''],
   ['人物', '人物 2', '林夏'],
 ]) {
   await page.getByRole('button', {name: `＋ 添加${kind}`, exact: true}).click();
   await expect(drawer.getByLabel('名称', {exact: true})).toHaveValue(expected!);
   if (renamed) await drawer.getByLabel('名称', {exact: true}).fill(renamed);
   if (expected === '人物 1') await page.screenshot({path: testInfo.outputPath('default-character-name.png')});
   await drawer.getByRole('button', {name: '保存并引用', exact: true}).click();
   await expect(drawer).toHaveCount(0);
   await expect(page.getByRole('status').first()).toHaveText('已保存');
 }
 await page.reload();
 const cards = page.locator('.entity-card');
 await expect(cards).toHaveCount(4);
 for (const name of ['场景 1', '雨夜咖啡馆', '人物 1', '林夏']) await expect(cards.filter({hasText: name})).toHaveCount(1);
 await cards.filter({hasText: '林夏'}).getByRole('button', {name: '编辑项目设定'}).click();
 await expect(drawer.getByLabel('名称', {exact: true})).toHaveValue('林夏');
});


test('video submissions stay inline and allow another run while the first is running', async ({page,request}, testInfo) => {
 const s = await seed(request);
 await page.goto(`${s.base}/shots/shot`);
 await configure(page,s);
 await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();
 const drawer = page.getByRole('dialog');
 await drawer.locator('input[type=file]').setInputFiles({name:'参考.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64')});
 await drawer.getByRole('button',{name:'使用这张图片',exact:true}).click();
 await drawer.getByRole('button',{name:'保存并引用'}).click();
 await expect(drawer).toHaveCount(0);
 const endpoint = `/api/projects/${s.p}/drafts/${s.d}/scenes/shot/generation-runs`;
 const keys: string[] = [];
 const ids: string[] = [];
 let complete = false;
 let releaseSubmit!: () => void;
 const submissionGate = new Promise<void>(resolve => {releaseSubmit = resolve});
 // Persist real runs, but simulate slow provider progress without paid calls.
 await page.route(`**${endpoint}/run-*`,async route => {
  const response = await route.fetch();
  const run = await response.json();
  const succeeded = complete && run.id === ids[0];
  await route.fulfill({json:{...run,state:succeeded?'awaiting_video_selection':'generating_video',error:'',steps:[{id:run.task_ids[0],kind:'video',status:succeeded?'succeeded':'running',progress:succeeded?100:35,result_url:succeeded?'/media/inline-preview.mp4':'',error:''}]}});
 });
 await page.route(`**${endpoint}`,async route => {
  if(route.request().method()!=='POST'){await route.continue();return}
  keys.push(route.request().postDataJSON().idempotency_key);
  const response = await route.fetch();
  expect(response.status()).toBe(202);
  const run = await response.json();
  ids.push(run.id);
  if(keys.length===1) await submissionGate;
  await route.fulfill({response});
 });
 const generate = page.getByRole('button',{name:'生成视频',exact:true});
 await generate.click();
 await expect(page.getByRole('button',{name:'提交中…',exact:true})).toBeDisabled();
 releaseSubmit();
 await expect(page).toHaveURL(`${s.base}/shots/shot`);
 await expect(page.getByLabel('任务 1 视频生成进度')).toHaveAttribute('value','35');
 await expect(generate).toBeEnabled();
 await page.getByLabel('镜头描述文本').fill('第二次生成，镜头拉远');
 await expect(page.getByRole('status').first()).toHaveText('已保存');
 await generate.click();
 await expect(page.locator('.generation-run-row')).toHaveCount(2);
 await expect(page.getByLabel('任务 2 视频生成进度')).toHaveAttribute('value','35');
 await expect(page.getByLabel('任务 1 视频生成进度')).toBeVisible();
 await expect(generate).toBeEnabled();
 expect(keys).toHaveLength(2);
 expect(new Set(keys).size).toBe(2);
 expect(new Set(ids).size).toBe(2);
 const persisted = (await (await request.get(endpoint)).json()).runs;
 expect(persisted).toHaveLength(2);
 expect(persisted.find((r:any)=>r.id===ids[0]).plan[0].prompt).not.toContain('第二次生成');
 expect(persisted.find((r:any)=>r.id===ids[1]).plan[0].prompt).toContain('第二次生成');
 await page.getByRole('region',{name:'任务结构'}).screenshot({path:testInfo.outputPath('inline-runs-desktop.png')});
 await page.setViewportSize({width:390,height:844});
 await page.getByRole('region',{name:'任务结构'}).screenshot({path:testInfo.outputPath('inline-runs-mobile.png')});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.route('**/media/inline-preview.mp4',r=>r.fulfill({status:200,contentType:'video/mp4',body:''}));
 complete = true;
 await expect(page.locator('.generation-run-row').first().locator('video')).toHaveAttribute('src',/\/media\/inline-preview\.mp4$/);
 await expect(page.getByLabel('任务 2 视频生成进度')).toBeVisible();
 await expect(page).toHaveURL(`${s.base}/shots/shot`);
 await page.reload();
 await expect(page.locator('.generation-run-row')).toHaveCount(2);
 await expect(generate).toBeEnabled();
});


test('image editing is configured first and keeps ordered project-only references', async ({page, request}, testInfo) => {
 const s = await seed(request);
 const catalog = await (await request.get('/api/models')).json();
 const model = catalog.models.find((m: any) => m.id === s.image);
 expect(model).toBeTruthy();
 model.operations.image.image_modes = ['text_to_image'];
 expect((await request.put(`/api/models/${encodeURIComponent(s.image)}/operations`, {data:{expected_revision:model.revision,capabilities:['image'],operations:model.operations}})).ok()).toBeTruthy();
 await page.route(/\/api\/models(?:\?.*)?$/, async route => {
  const response = await route.fetch();const data = await response.json();
  await route.fulfill({response,json:{...data,models:data.models.filter((m:any)=>m.id===s.image||m.id===s.video)}});
 });
 await page.goto(`${s.base}/shots/shot`);
 await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();
 let drawer = page.getByRole('dialog');
 await drawer.getByRole('button',{name:'图片生成',exact:true}).click();
 await expect(drawer.getByRole('button',{name:'图生图',exact:true})).toBeDisabled();
 await expect(drawer).toContainText('图生图尚未配置');
 await drawer.getByRole('button',{name:'取消',exact:true}).click();
 // Enable the capability through the real model configuration UI.
 await page.route('**/api/connections', async route => {
  if (route.request().method() !== 'GET') { await route.continue(); return; }
  const response=await route.fetch();const data=await response.json();
  await route.fulfill({response,json:{...data,connections:data.connections.filter((c:any)=>c.id===model.connection_id)}});
 });
 await page.goto('/config');
 const row = page.getByRole('row').filter({hasText:'验收分镜图'});
 await row.getByRole('button',{name:'协议与参数',exact:true}).click();
 drawer = page.getByRole('dialog');
 await drawer.getByLabel('图生图',{exact:true}).check();
 await drawer.getByLabel('最少参考图片数',{exact:true}).fill('1');
 await drawer.getByLabel('最多参考图片数',{exact:true}).fill('3');
 await drawer.getByLabel('单张图片上限（MB）',{exact:true}).fill('10');
 await drawer.getByRole('group',{name:'图片生成能力（可多选）'}).scrollIntoViewIfNeeded();
 await page.screenshot({path:testInfo.outputPath('image-mode-configuration-desktop.png')});
 await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:testInfo.outputPath('image-mode-configuration-mobile.png')});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
 await drawer.getByRole('button',{name:'保存配置',exact:true}).click();await expect(drawer).toHaveCount(0);
 await page.setViewportSize({width:1440,height:1000});
 await page.goto(`${s.base}/shots/shot`);
 await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();drawer=page.getByRole('dialog');
 await drawer.getByLabel('名称',{exact:true}).fill('暖色咖啡馆');
 await drawer.getByRole('button',{name:'图片生成',exact:true}).click();
 await drawer.getByRole('button',{name:'图生图',exact:true}).click();
 await drawer.getByRole('combobox',{name:'生图模型',exact:true}).selectOption(s.image);
 await drawer.getByLabel('生图提示词',{exact:true}).fill('保留图 1 的空间，采用图 2 的暖色光线。');
 await drawer.getByLabel('上传图生图参考图片',{exact:true}).setInputFiles(['public/home/cafe.png','public/home/forest.png']);
 await expect(drawer.locator('.image-input-list li')).toHaveCount(2);
 await drawer.getByRole('button',{name:'前移图 2',exact:true}).click();
 const uploaded = (await (await request.get(`/api/materials?project_id=${s.p}`)).json()).materials;
 const inputs = uploaded.filter((m:any)=>m.reference_only && m.project_id===s.p);
 expect(inputs).toHaveLength(2);
 let submitted:any;
 const taskId = `edit-demo-${randomUUID()}`;
 await page.route('**/reference-image-tasks',route=>{
  submitted=route.request().postDataJSON();return route.fulfill({status:202,json:{id:taskId,status:'running',kind:'image',progress:10}});
 });
 await page.route(`**/api/tasks/${taskId}`,route=>route.fulfill({json:{id:taskId,status:'running',kind:'image',progress:10}}));
 await drawer.locator('.image-edit-references').scrollIntoViewIfNeeded();
 await page.screenshot({path:testInfo.outputPath('image-editing-desktop.png')});
 const axe = await new AxeBuilder({page}).include('.entity-editor-drawer').withTags(['wcag2a','wcag2aa']).analyze();expect(axe.violations).toEqual([]);
 await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:testInfo.outputPath('image-editing-mobile.png')});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
 await drawer.getByRole('button',{name:'生成图片',exact:true}).click();
 await expect(drawer.getByRole('status').last()).toContainText('生成中');
 expect(submitted.mode).toBe('image_to_image');
 expect(submitted.reference_material_ids).toEqual([inputs.find((m:any)=>m.name.includes('forest')).id,inputs.find((m:any)=>m.name.includes('cafe')).id]);
 expect((await (await request.get('/api/materials')).json()).materials.some((m:any)=>inputs.some((i:any)=>i.id===m.id))).toBeFalsy();
 await expect(drawer.locator('.reference-tile')).toHaveCount(0);
});
