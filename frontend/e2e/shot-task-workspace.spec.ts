import {test,expect,type APIRequestContext,type Page} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import AxeBuilder from '@axe-core/playwright';

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
 const c=await request.post('/api/connections',{data:{id,name:'本地验收连接',vendor:'bailian',region:'cn',base_url:'https://example.invalid/api/v1'}});expect(c.ok()).toBeTruthy();
 for(const [remote,name,cap] of [['qwen-image-2.0-pro','验收分镜图','image'],['wan2.7-r2v','验收视频','video'],['qwen3-tts-flash','验收配音','speech']]){const r=await request.post(`/api/connections/${id}/models`,{data:{model_id:remote,name,capabilities:[cap]}});expect(r.ok()).toBeTruthy()}
 const p=await(await request.post('/api/projects',{data:{name:'分镜工作区验收'}})).json();const d='draft-'+randomUUID();
 const r=await request.post(`/api/projects/${p.id}/drafts`,{data:{id:d,name:'雨夜故事',story:{body:'雨夜咖啡馆',summary:'',scenes:[{id:'shot',order:1,title:'雨夜 · 走进咖啡馆',visual_prompt:'中景，镜头缓慢推进，林夏推开玻璃门。',narration:'',duration_seconds:4}]}}});expect(r.ok()).toBeTruthy();
 return {p:p.id as string,d,base:`/studio/${p.id}/${d}`,image:`${id}:qwen-image-2.0-pro`,video:`${id}:wan2.7-r2v`};
}
async function configure(page:Page,s:Awaited<ReturnType<typeof seed>>){
 await page.getByRole('combobox',{name:'视频模型',exact:true}).selectOption(s.video);await expect(page.getByRole('status').first()).toHaveText('已保存');
}
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
 const card=page.locator('.entity-card').nth(1);const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');await card.locator('input[type=file]').setInputFiles({name:'林夏.png',mimeType:'image/png',buffer:png});await expect(card.locator('.reference-tile')).toHaveCount(1);await expect(page.getByRole('status').first()).toHaveText('已保存');await card.locator('input[type=file]').first().setInputFiles({name:'林夏替换.png',mimeType:'image/png',buffer:png});await expect(card.locator('.reference-tile')).toHaveCount(1);await expect(page.getByRole('status').first()).toHaveText('已保存');
 await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:testInfo.outputPath('shot-detail-desktop.png'),fullPage:true});await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:testInfo.outputPath('shot-detail-mobile.png'),fullPage:true});await page.setViewportSize({width:1440,height:1000});await page.reload();await expect(page.getByRole('combobox',{name:'视频模型',exact:true})).toHaveValue(s.video);
 await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(/\/runs\/run-/);await expect(page.getByRole('heading',{name:'分镜图',exact:true})).toHaveCount(0);await expect(page.locator('.run-step')).toHaveCount(1);
 await expect(page.getByRole('heading',{name:'制作任务详情'})).toBeVisible();await expect(page.getByRole('region',{name:'本次生成设定'})).toContainText('4 秒');await expect(page.getByRole('button',{name:'按原输入重试'})).toBeVisible({timeout:15000});
 await page.screenshot({path:testInfo.outputPath('video-task-detail-desktop.png'),fullPage:true});await page.getByRole('button',{name:'按原输入重试'}).click();await expect(page.getByRole('button',{name:'按原输入重试'})).toBeVisible({timeout:15000});
 await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row')).toHaveCount(1);await page.getByLabel('镜头描述文本').fill('第二次生成的新镜头描述');await expect(page.getByRole('status').first()).toHaveText('已保存');
 await expect(page.getByRole('button',{name:'生成视频',exact:true})).toBeEnabled({timeout:15000});await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(/\/runs\/run-/);await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row')).toHaveCount(2);
 await page.getByRole('link',{name:'进入任务 →'}).first().click();await page.getByText('提交时的场景描述与引用素材').click();await expect(page.locator('.snapshot-prompt')).toContainText('中景，镜头缓慢推进');await expect(page.locator('.snapshot-prompt')).not.toContainText('第二次');
 await page.getByRole('link',{name:'分镜列表',exact:true}).click();await expect(page.locator('.shot-list-row')).toContainText('2 个制作任务');await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();await page.getByRole('link',{name:'进入详情 →'}).click();await page.getByRole('link',{name:'进入任务 →'}).nth(1).click();await page.getByRole('button',{name:'取消制作任务'}).click();await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row').nth(1)).toContainText('已取消');
});
test('lost submit response recovers same run without duplication',async({page,request})=>{
 const s=await seed(request);await page.goto(`${s.base}/shots/shot`);await configure(page,s);await page.getByRole('button',{name:'＋ 添加场景',exact:true}).click();const form=page.getByRole('dialog');await form.getByLabel('名称',{exact:true}).fill('参考场景');await form.locator('input[type=file]').setInputFiles({name:'场景.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64')});await expect(form.locator('.reference-tile')).toHaveCount(1);await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);const keys:string[]=[];
 await page.route('**/generation-runs',async route=>{if(route.request().method()!=='POST'){await route.continue();return}keys.push(route.request().postDataJSON().idempotency_key);if(keys.length===1){await route.fetch();await route.abort('failed')}else await route.continue()});
 await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await page.getByRole('button',{name:'生成视频',exact:true}).click();await expect(page).toHaveURL(/\/runs\/run-/);await page.getByRole('link',{name:'分镜详情',exact:true}).click();await expect(page.locator('.generation-run-row')).toHaveCount(1);expect(keys).toHaveLength(2);expect(keys[0]).toBe(keys[1]);await page.reload();await expect(page.locator('.generation-run-row')).toHaveCount(1);
});
