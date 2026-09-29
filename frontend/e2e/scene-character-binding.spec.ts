import {test,expect,type APIRequestContext} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
async function seed(request:APIRequestContext){
 const p=await(await request.post('/api/projects',{data:{name:'Binding E2E'}})).json();const d='d-'+Date.now();await request.post(`/api/projects/${p.id}/drafts`,{data:{id:d,name:'Binding test',story:{body:'咖啡馆里的约定',summary:'',scenes:[{id:'s',order:1,title:'镜头一',visual_prompt:'咖啡馆交谈',narration:'',duration_seconds:6}]}}});return {p:p.id as string,d,url:`/studio/${p.id}/${d}?scene=s`};
}
test('group descriptions, duplicate names, references, reload and local scope',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();
 for(const label of ['＋ 添加场景','＋ 添加人物','＋ 添加人物']){await work.getByRole('button',{name:label,exact:true}).click();const form=work.locator('form');await form.getByLabel('名称',{exact:true}).fill('同名');await form.getByLabel('基础描述',{exact:true}).fill(label.includes('场景')?'午后咖啡馆':'蓝外套，短发');await form.getByRole('button',{name:'保存设定并用于此镜头'}).click();await expect(form).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存')}
 await expect(work.locator('.entity-card')).toHaveCount(3);
 await expect(work.getByRole('region',{name:'场景绑定'})).toContainText('1 个');
 await expect(work.getByRole('region',{name:'人物绑定'})).toContainText('2 个');
 await expect(work.getByRole('region',{name:'当前镜头制作任务'})).toContainText('制作任务');
 await expect(page.locator('.scene-list .scene-row').first()).toContainText('绑定：1 个场景 · 2 个人物');
 await work.getByRole('button',{name:'＠ 插入场景或人物'}).click();await work.getByRole('option').nth(2).click();await expect(work.locator('.mention-chip')).toContainText('@同名');
 await work.getByLabel('本镜头补充').nth(1).fill('中文输入：微笑，手持咖啡。');await expect(work.getByRole('status').first()).toHaveText('已保存');
 const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[0].prompt_document.nodes.some((n:{binding_id?:string})=>n.binding_id===d.story.scenes[0].entity_bindings[2].id)).toBeTruthy();
 await page.reload();await expect(work.getByLabel('本镜头补充').nth(1)).toHaveValue('中文输入：微笑，手持咖啡。');await expect(work.locator('.entity-card')).toHaveCount(3);
 await page.setViewportSize({width:390,height:844});await expect(work.getByRole('button',{name:'＋ 添加人物',exact:true})).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();
 const results=await new AxeBuilder({page}).include('.binding-workbench').withTags(['wcag2a','wcag2aa']).analyze();expect(results.violations).toEqual([]);
});
test('multiple uploads retain successes, retry failure, reorder and remove without deleting material',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await work.getByRole('button',{name:'＋ 添加人物',exact:true}).click();const form=work.locator('form');await form.getByLabel('名称',{exact:true}).fill('多图人物');
 let calls=0;await page.route('**/api/materials/upload',async route=>{calls++;if(calls===2)await route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:{message:'测试上传失败'}})});else await route.continue()});
 const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=','base64');
 await form.locator('input[type=file]').setInputFiles([{name:'正面.png',mimeType:'image/png',buffer:png},{name:'侧面.png',mimeType:'image/png',buffer:png}]);await expect(form.getByRole('button',{name:'重试失败图片'})).toBeVisible();await form.getByRole('button',{name:'重试失败图片'}).click();await expect(form.locator('.reference-tile')).toHaveCount(2);expect(calls).toBe(3);
 await form.getByRole('button',{name:'参考图后移'}).first().click();await form.getByRole('button',{name:'保存设定并用于此镜头'}).click();await expect(form).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存');
 await work.getByRole('button',{name:'移除参考图绑定'}).first().click();await expect(work.getByRole('status').first()).toHaveText('已保存');const catalog=await(await request.get(`/api/projects/${s.p}/entities`)).json();expect(catalog.entities[0].references).toHaveLength(2);const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[0].entity_bindings[0].selected_material_ids).toHaveLength(1);
});
test('version conflict keeps local text and offers explicit resolution',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();await request.put(`/api/projects/${s.p}/drafts/${s.d}`,{data:{expected_version:d.version,body:'另一个窗口的修改'}});
 await work.getByLabel('镜头描述文本').fill('保留我的本地文字');await expect(work.getByText('此草稿已在其他位置更新。本地输入已保留，请比较后选择。')).toBeVisible();await expect(work.getByLabel('镜头描述文本')).toHaveValue('保留我的本地文字');await work.getByRole('button',{name:'保留本地镜头内容并保存到最新版本'}).click();await expect(work.getByRole('status').first()).toHaveText('已保存');const saved=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(saved.story.body).toBe('另一个窗口的修改');expect(saved.story.scenes[0].visual_prompt).toBe('保留我的本地文字');
});

test('editing increments entity version, keyboard mentions target exact identity and quick scene switching saves',async({page,request})=>{
 const s=await seed(request);await page.goto(s.url);const work=page.getByRole('region',{name:'场景人物工作台'});await expect(work).toBeVisible();
 for(const name of ['人物甲','人物乙']){await work.getByRole('button',{name:'＋ 添加人物',exact:true}).click();await work.getByLabel('名称',{exact:true}).fill(name);await work.getByRole('button',{name:'保存设定并用于此镜头'}).click();await expect(work.locator('form')).toHaveCount(0);await expect(work.getByRole('status').first()).toHaveText('已保存')}
 await work.getByRole('button',{name:'编辑项目设定',exact:true}).nth(1).click();await work.getByLabel('名称',{exact:true}).fill('人物乙新名称');await work.getByLabel('基础描述',{exact:true}).fill('新版本人物描述');await work.getByRole('button',{name:'保存设定并用于此镜头'}).click();await expect(work.locator('form')).toHaveCount(0);await expect(work.locator('.entity-card').nth(1)).toContainText('v2');
 await work.getByRole('button',{name:'＠ 插入场景或人物'}).click();await expect(work.getByRole('option').first()).toBeFocused();await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');await expect(work.locator('.mention-chip')).toContainText('人物乙新名称');
 await work.getByRole('button',{name:'＠ 插入场景或人物'}).click();await work.getByRole('button',{name:'取消',exact:true}).focus();await page.keyboard.press('Enter');await expect(work.locator('.mention-chip')).toHaveCount(1);
 await page.getByRole('button',{name:'复制',exact:true}).click();await expect(page.locator('.scene-row')).toHaveCount(2);await work.getByLabel('本镜头补充').nth(1).fill('快速输入不能丢失');await page.locator('.scene-row').first().click();await page.locator('.scene-row').nth(1).click();await expect(work.getByLabel('本镜头补充').nth(1)).toHaveValue('快速输入不能丢失');
 const d=await(await request.get(`/api/projects/${s.p}/drafts/${s.d}`)).json();expect(d.story.scenes[1].entity_bindings[1].description_supplement).toBe('快速输入不能丢失');expect(d.story.scenes[0].entity_bindings[1].description_supplement||'').toBe('');
});
