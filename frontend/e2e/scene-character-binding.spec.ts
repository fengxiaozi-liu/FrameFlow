import {test,expect,type APIRequestContext} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
async function seed(request:APIRequestContext){
 const p=await(await request.post('/api/projects',{data:{name:'Binding E2E'}})).json();const d='d-'+Date.now();await request.post(`/api/projects/${p.id}/drafts`,{data:{id:d,name:'Binding test',story:{body:'咖啡馆里的约定',summary:'',scenes:[{id:'s',order:1,title:'镜头一',visual_prompt:'咖啡馆交谈',narration:'',duration_seconds:6}]}}});return {p:p.id as string,d,url:`/studio/${p.id}/${d}?scene=s`};
}
test('group descriptions, duplicate names, references, reload and local scope',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();
 for(const label of ['＋ 添加场景','＋ 添加人物','＋ 添加人物']){await work.getByRole('button',{name:label,exact:true}).click();const form=page.getByRole('dialog').locator('form');await form.getByLabel('名称',{exact:true}).fill('同名');await form.getByLabel('参考描述',{exact:true}).fill(label.includes('场景')?'午后咖啡馆':'蓝外套，短发');await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存')}
 await expect(work.locator('.entity-card')).toHaveCount(3);
 await expect(page.getByRole('region',{name:'任务结构'})).toContainText('暂无制作任务');
 await expect(work.locator('.shot-entity-grid .entity-card')).toHaveCount(3);
 await expect(work.getByRole('button',{name:'＠ 插入场景或人物'})).toHaveCount(0);
 await work.getByLabel('镜头描述文本').first().fill('中文输入：微笑，手持咖啡。');await expect(work.getByRole('status').first()).toHaveText('已保存');
 const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[0].entity_bindings).toHaveLength(3);
 await page.reload();await expect(work.getByLabel('镜头描述文本').first()).toHaveText('中文输入：微笑，手持咖啡。');await expect(work.locator('.entity-card')).toHaveCount(3);
 await page.setViewportSize({width:390,height:844});await expect(work.getByRole('button',{name:'＋ 添加人物',exact:true})).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();
 const results=await new AxeBuilder({page}).include('.binding-workbench').withTags(['wcag2a','wcag2aa']).analyze();expect(results.violations).toEqual([]);
});
test('multiple uploads retain successes, retry failure, reorder and remove without deleting material',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await work.getByRole('button',{name:'＋ 添加人物',exact:true}).click();const form=page.getByRole('dialog').locator('form');await form.getByLabel('名称',{exact:true}).fill('多图人物');
 let calls=0;await page.route('**/api/materials/upload',async route=>{calls++;if(calls===2)await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{message:'测试上传失败'}})});else await route.continue()});
 const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');
 await form.locator('input[type=file]').setInputFiles([{name:'正面.png',mimeType:'image/png',buffer:png},{name:'侧面.png',mimeType:'image/png',buffer:png}]);await expect(form.getByRole('button',{name:'重试失败图片'})).toBeVisible();await form.getByRole('button',{name:'重试失败图片'}).click();await expect(form.locator('.reference-tile')).toHaveCount(2);expect(calls).toBe(3);
 await form.getByRole('button',{name:'参考图后移'}).first().click();await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存');
 await work.getByRole('button',{name:'移除参考图绑定'}).first().click();await expect(work.getByRole('status').first()).toHaveText('已保存');const catalog=await(await request.get(`/api/projects/${s.p}/entities`)).json();expect(catalog.entities[0].references).toHaveLength(2);const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[0].entity_bindings[0].reference_material_ids).toHaveLength(1);
});
test('version conflict keeps local text and offers explicit resolution',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();await request.put(`/api/projects/${s.p}/drafts/${s.d}`,{data:{expected_version:d.version,body:'另一个窗口的修改'}});
 await work.getByLabel('镜头描述文本').fill('保留我的本地文字');await expect(work.getByText('此草稿已在其他位置更新。本地输入已保留，请比较后选择。')).toBeVisible();await expect(work.getByLabel('镜头描述文本')).toHaveText('保留我的本地文字');await work.getByRole('button',{name:'保留本地镜头内容并保存到最新版本'}).click();await expect(work.getByRole('status').first()).toHaveText('已保存');const saved=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(saved.story.body).toBe('另一个窗口的修改');expect(saved.story.scenes[0].visual_prompt).toBe('保留我的本地文字');
});

test('editing increments entity version, existing references remain readable and quick scene switching saves',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();
 for(const name of ['人物甲','人物乙']){await work.getByRole('button',{name:'＋ 添加人物',exact:true}).click();await page.getByRole('dialog').getByLabel('名称',{exact:true}).fill(name);await page.getByRole('dialog').getByRole('button',{name:'保存并引用'}).click();await expect(page.getByRole('dialog').locator('form')).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存')}
 await work.getByRole('button',{name:'编辑项目设定',exact:true}).nth(1).click();await page.getByRole('dialog').getByLabel('名称',{exact:true}).fill('人物乙新名称');await page.getByRole('dialog').getByLabel('参考描述',{exact:true}).fill('新版本人物描述');await page.getByRole('dialog').getByRole('button',{name:'保存并引用'}).click();await expect(page.getByRole('dialog').locator('form')).toHaveCount(0);await expect(work.locator('.entity-card').nth(1)).toContainText('v2');
 const previous=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();const scene=previous.story.scenes[0];
 const restored=await request.put(`/api/projects/${s.p}/drafts/${s.d}/scenes/s/context`,{data:{expected_version:previous.version,entity_bindings:scene.entity_bindings,utterances:scene.utterances||[],prompt_document:{schema_version:1,nodes:[{type:'text',text:'已有描述 '},{type:'entity_ref',binding_id:scene.entity_bindings[1].id},{type:'text',text:''}]}}});expect(restored.ok()).toBeTruthy();await page.reload();await expect(work.locator('.mention-chip')).toContainText('人物乙新名称');await expect(work.getByRole('button',{name:'＠ 插入场景或人物'})).toHaveCount(0);
 await page.getByRole('link',{name:'分镜列表',exact:true}).click();await page.getByRole('button',{name:'复制',exact:true}).click();await expect(page.locator('.shot-list-row')).toHaveCount(2);await page.getByRole('link',{name:'进入详情 →'}).nth(1).click();await work.getByLabel('镜头描述文本').first().fill('快速输入不能丢失');await page.getByRole('link',{name:'分镜列表',exact:true}).click();await page.getByRole('link',{name:'进入详情 →'}).first().click();await page.getByRole('link',{name:'分镜列表',exact:true}).click();await page.getByRole('link',{name:'进入详情 →'}).nth(1).click();await expect(work.getByLabel('镜头描述文本').first()).toHaveText('快速输入不能丢失');
 const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[1].visual_prompt).toContain('快速输入不能丢失');expect(d.story.scenes[0].visual_prompt).not.toContain('快速输入不能丢失');
});


test('typing @ opens bound scene and character candidates without an insert button',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);
 const work=page.getByRole('region',{name:'场景人物工作台'});
 for(const [button,name] of [['＋ 添加场景','1111'],['＋ 添加人物','林夏']]){
  await work.getByRole('button',{name:button,exact:true}).click();const form=page.getByRole('dialog');await form.getByLabel('名称',{exact:true}).fill(name!);await form.getByRole('button',{name:'保存并引用'}).click();await expect(form).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存');
 }
 await expect(work.getByRole('button',{name:'＠ 插入场景或人物'})).toHaveCount(0);
 const text=work.getByLabel('镜头描述文本').first();await text.fill('走进门口');await text.press('Home');await text.press('ArrowRight');await text.press('ArrowRight');await text.press('@');
 const options=work.getByRole('listbox',{name:'引用候选'}).getByRole('option');await expect(options).toHaveCount(2);await expect(options.first()).toContainText('1111');await expect(options.last()).toContainText('林夏');
 await options.first().press('Escape');await expect(work.getByRole('listbox')).toHaveCount(0);await expect(text).toBeFocused();await expect(text).toHaveText('走进门口');
 await text.press('@');await options.first().press('Enter');await expect(work.locator('.mention-chip')).toContainText('@1111');await expect(work.getByRole('status').first()).toHaveText('已保存');
 await page.reload();await expect(work.locator('.mention-chip')).toContainText('@1111');await expect(work.getByLabel('镜头描述文本')).toHaveCount(1);await expect(work.getByLabel('镜头描述文本')).toHaveText('走进@1111×门口');
 const draft=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();const scene=draft.story.scenes[0];expect(scene.prompt_document.nodes[1]).toEqual({type:'entity_ref',binding_id:scene.entity_bindings[0].id});expect(scene.visual_prompt).toBe('走进1111门口');
 await text.press('Control+End');await text.press('Enter');await text.pressSequentially('第二行');await work.getByRole('button',{name:'移除描述引用'}).click();await expect(work.locator('.mention-chip')).toHaveCount(0);
 await text.press('Control+z');await expect(work.locator('.mention-chip')).toHaveCount(1);await text.press('Control+y');await expect(work.locator('.mention-chip')).toHaveCount(0);await text.press('Control+z');await expect(work.locator('.mention-chip')).toHaveCount(1);
 await expect(work.getByRole('status').first()).toHaveText('已保存');await page.reload();await expect(text).toContainText('第二行');await expect(work.locator('.mention-chip')).toContainText('1111');
 await text.press('Control+End');await text.press('@');await options.last().press('Enter');await expect(work.locator('.mention-chip')).toHaveCount(2);await text.press('Backspace');await expect(work.locator('.mention-chip')).toHaveCount(1);
 await text.evaluate(el=>{const data=new DataTransfer();data.setData('text/plain',' 粘贴内容');data.setData('text/html','<img src=x onerror=alert(1)>');el.dispatchEvent(new ClipboardEvent('paste',{clipboardData:data,bubbles:true,cancelable:true}))});await expect(text.locator('img')).toHaveCount(0);await expect(text).toContainText('粘贴内容');
 await expect(work.getByRole('status').first()).toHaveText('已保存');
 await page.locator('.shot-prompt-field').screenshot({path:'../specs/shot-entry-and-voice/artifacts/inline-mention-editor.png'});
 const accessibility=await new AxeBuilder({page}).include('.shot-prompt-field').withTags(['wcag2a','wcag2aa']).analyze();expect(accessibility.violations).toEqual([]);
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.locator('.shot-prompt-field').screenshot({path:'../specs/shot-entry-and-voice/artifacts/inline-mention-editor-mobile.png'});

});
