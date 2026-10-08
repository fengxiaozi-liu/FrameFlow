import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { randomUUID } from 'node:crypto';

test('light workbench keeps dense references usable and global navigation responsive', async ({ page, request }, testInfo) => {
  test.setTimeout(90000);
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  const project = await (await request.post('/api/projects', {data:{name:'雨夜咖啡馆 · 专业工作台'}})).json();
  const draftID = randomUUID(), materialID = randomUUID();
  await request.post('/api/materials', {data:{id:materialID,name:'雨夜咖啡馆参考',kind:'scene',url:'/home/cafe.png',media:{format:'image/png',width:1536,height:1024}}});
  const entities = [
    {id:'cafe',kind:'location',name:'雨夜咖啡馆',description:'街角暖色灯光，落地玻璃窗映出雨中的街景。',revision:1,references:[{material_id:materialID,order:0,label:'街角'}]},
    {id:'lin',kind:'character',name:'林夏',description:'黑色短发、米色风衣，手持折叠雨伞。',revision:1,references:[]},
    {id:'chen',kind:'character',name:'陈默 · 窗边等待雨停的年轻摄影师',description:'穿深蓝色外套，坐在靠窗的位置。较长的角色设定用于验证紧凑卡片仍能保留完整信息，并且不会挤压其他素材或操作按钮。',revision:1,references:[]},
  ];
  let version=0;
  for (const entity of entities) {
    const response=await request.post(`/api/projects/${project.id}/entities`,{data:{expected_catalog_version:version,entity}});
    expect(response.ok()).toBeTruthy();version=(await response.json()).catalog_version;
  }
  const created=await request.post(`/api/projects/${project.id}/drafts`,{data:{id:draftID,name:'雨夜故事',story:{body:'雨夜，林夏推开咖啡馆的玻璃门。陈默收起相机，抬头看向她。\n\n窗外雨水与暖色灯光形成反差。',summary:'',scenes:[{id:'shot',order:1,title:'雨夜 · 走进咖啡馆',visual_prompt:'中景，镜头缓慢推进。林夏推开雨夜咖啡馆的玻璃门，陈默在窗边抬头。窗外雨水与暖色灯光形成反差。',duration_seconds:6,entity_bindings:entities.map((e,i)=>({id:'binding-'+i,entity_id:e.id,entity_revision:1,role:i?'cast':'primary'}))}]}}});
  expect(created.ok()).toBeTruthy();
  const base=`/studio/${project.id}/${draftID}`;
  await page.goto(`${base}/shots/shot`);
  await expect(page.locator('.entity-card-compact')).toHaveCount(3);
  await expect(page.getByLabel('分镜名称',{exact:true})).toHaveValue('雨夜 · 走进咖啡馆');
  await expect(page.getByRole('navigation',{name:'全局导航'}).getByRole('link',{name:'创作工作台',exact:true})).toHaveAttribute('aria-current','page');
  expect(await page.locator('.app-header').evaluate(el=>getComputedStyle(el).position)).toBe('fixed');
  for(const card of await page.locator('.entity-card-compact').all()) {
    expect((await card.boundingBox())!.height).toBeLessThan(255);
    await expect(card.locator('details')).not.toHaveAttribute('open');
  }
  const description=await page.getByLabel('镜头描述文本').boundingBox();
  expect(description!.y+description!.height).toBeLessThan(850);
  await page.screenshot({path:testInfo.outputPath('shot-desktop.png'),fullPage:true});
  const card=page.locator('.entity-card-compact').first();
  await card.locator('summary').click();await expect(card.getByRole('button',{name:'预览参考图 1'})).toBeVisible();
  await card.getByRole('button',{name:'预览参考图 1'}).click();await expect(page.getByRole('dialog',{name:'参考图预览'})).toBeVisible();await page.getByRole('button',{name:'关闭预览'}).click();await card.locator('summary').click();
  await expect(card.getByRole('button',{name:'预览参考图 1'})).not.toBeVisible();
  await page.getByRole('button',{name:'编辑项目设定',exact:true}).last().click();
  await expect(page.getByRole('dialog').getByLabel('参考描述',{exact:true})).toHaveValue(entities[2]!.description);
  await page.screenshot({path:testInfo.outputPath('drawer-desktop.png')});
  expect((await new AxeBuilder({page}).include('.entity-editor-drawer').withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
  await page.keyboard.press('Escape');
  expect((await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await page.screenshot({path:testInfo.outputPath('shot-mobile.png'),fullPage:true});
  await page.getByRole('button',{name:'＋ 添加人物',exact:true}).click();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await page.screenshot({path:testInfo.outputPath('drawer-mobile.png')});await page.keyboard.press('Escape');
  const destinations=[['分镜列表',`${base}/shots`],['正文',base],['素材管理','/materials'],['模型管理','/config'],['任务中心','/tasks'],['创作首页','/']] as const;
  for(const [name,url] of destinations){
    await page.setViewportSize({width:1440,height:1000});await page.goto(url);
    await expect(page.locator('main')).toBeVisible();
    if(name==='素材管理')await expect(page.locator('.material-card').first()).toBeVisible();
    if(name==='创作首页')await expect(page.locator('.home-showcase>img:visible')).toBeVisible();
    await page.screenshot({path:testInfo.outputPath(`${name}-desktop.png`),fullPage:true});
    await page.setViewportSize({width:390,height:844});
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),name).toBeTruthy();
    await page.screenshot({path:testInfo.outputPath(`${name}-mobile.png`),fullPage:true});
  }
  await page.getByRole('navigation',{name:'全局导航'}).getByRole('link',{name:'素材管理',exact:true}).click();await expect(page).toHaveURL(/\/materials$/);
  expect(errors).toEqual([]);
});
