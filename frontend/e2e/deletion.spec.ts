import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import AxeBuilder from '@axe-core/playwright';

test('draft delete supports cancel, retry, individual drafts and persistence',async({page,request},testInfo)=>{
  const project=await(await request.post('/api/projects',{data:{name:'删除验收项目'}})).json();
  const ids=[randomUUID(),randomUUID()];
  for(let i=0;i<2;i++)expect((await request.post(`/api/projects/${project.id}/drafts`,{data:{id:ids[i],name:i?'第二份草稿':'第一份草稿',story:{body:'保留正文',scenes:[]}}})).ok()).toBeTruthy();
  await page.route('**/api/projects',async route=>{const response=await route.fetch();const data=await response.json();await route.fulfill({json:{projects:data.projects.filter((p:{id:string})=>p.id===project.id)}});});
  await page.goto('/studio');
  await expect(page.locator('.workbench-project-item')).toHaveCount(1);
  await page.locator('.workbench-other-drafts summary').click();
  const remove=page.getByRole('button',{name:'删除草稿 第二份草稿',exact:true});
  page.once('dialog',dialog=>dialog.dismiss());
  await remove.click();
  expect((await request.get(`/api/projects/${project.id}/drafts/${ids[1]}`)).ok()).toBeTruthy();
  let fail=true;
  await page.route(`**/api/projects/${project.id}/drafts/${ids[1]}?*`,route=>fail?route.fulfill({status:503,json:{error:{message:'删除暂时失败，请重试'}}}):route.continue());
  page.once('dialog',dialog=>dialog.accept());await remove.click();
  await expect(page.getByRole('alert')).toContainText('删除暂时失败');
  await expect(remove).toBeEnabled();
  fail=false;
  page.once('dialog',dialog=>dialog.accept());await remove.click();
  await expect(page.getByRole('status')).toContainText('第二份草稿');
  await expect(page.locator('.workbench-other-drafts')).toHaveCount(0);
  expect((await request.get(`/api/projects/${project.id}/drafts/${ids[0]}`)).ok()).toBeTruthy();
  expect((await request.get(`/api/projects/${project.id}/drafts/${ids[1]}`)).status()).toBe(404);
  await page.reload();
  await expect(page.getByRole('button',{name:'删除草稿 第一份草稿',exact:true})).toBeVisible();
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await page.screenshot({path:testInfo.outputPath('draft-delete-mobile.png'),fullPage:true});
  expect((await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
  page.once('dialog',dialog=>dialog.accept());await page.getByRole('button',{name:'删除草稿 第一份草稿',exact:true}).click();
  await expect(page.getByRole('heading',{name:'开始你的第一个故事'})).toBeVisible();
  await page.reload();await expect(page.getByRole('heading',{name:'开始你的第一个故事'})).toBeVisible();
});

test('vendor delete cascades model settings and preserves the other connection',async({page,request},testInfo)=>{
  const ids=[randomUUID(),randomUUID()];
  for(let i=0;i<2;i++) {
    expect((await request.post('/api/connections',{data:{id:ids[i],name:i?'保留厂商':'待删除厂商',vendor:'bailian',base_url:'https://example.com/v1/chat/completions',api_key:'isolated-test-key'}})).ok()).toBeTruthy();
    expect((await request.post(`/api/connections/${ids[i]}/models`,{data:{model_id:'qwen-plus',capabilities:['story']}})).ok()).toBeTruthy();
  }
  await page.route('**/api/connections',async route=>{const response=await route.fetch();const data=await response.json();await route.fulfill({json:{connections:data.connections.filter((c:{id:string})=>ids.includes(c.id))}});});
  await page.goto('/config');
  await page.getByRole('button',{name:/待删除厂商.*百炼/}).click();
  const remove=page.getByRole('button',{name:'删除厂商',exact:true});
  page.once('dialog',dialog=>dialog.dismiss());await remove.click();
  await expect(page.getByRole('heading',{name:'待删除厂商',exact:true})).toBeVisible();
  let fail=true;
  await page.route(`**/api/connections/${ids[0]}`,route=>fail?route.fulfill({status:409,json:{error:{message:'该厂商有排队或进行中的任务，请完成或取消任务后再删除'}}}):route.continue());
  page.once('dialog',dialog=>dialog.accept());await remove.click();
  await expect(page.getByRole('alert')).toContainText('进行中的任务');
  await expect(remove).toBeEnabled();
  fail=false;
  await page.screenshot({path:testInfo.outputPath('vendor-delete-desktop.png'),fullPage:true});
  page.once('dialog',dialog=>dialog.accept());await remove.click();
  await expect(page.getByRole('heading',{name:'保留厂商',exact:true})).toBeVisible();
  expect((await(await request.get(`/api/models?connection_id=${ids[0]}`)).json()).models).toHaveLength(0);
  expect((await(await request.get(`/api/models?connection_id=${ids[1]}`)).json()).models).toHaveLength(1);
  await page.reload();await expect(page.getByRole('button',{name:/待删除厂商.*百炼/})).toHaveCount(0);
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await page.screenshot({path:testInfo.outputPath('vendor-delete-mobile.png'),fullPage:true});
  page.once('dialog',dialog=>dialog.accept());await remove.click();
  await expect(page.getByRole('heading',{name:'连接模型，开始 AI 创作'})).toBeVisible();
});
