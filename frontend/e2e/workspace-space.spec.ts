import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import type { ProviderConnection } from '../src/api/types';

test('work pages use wide windows and the story panel reaches the bottom', async ({ page }, testInfo) => {
  for (const viewport of [{width:1920,height:1080},{width:2560,height:1440},{width:1366,height:768}]) {
    await page.setViewportSize(viewport);
    await page.goto('/studio/new');
    const panel=page.locator('.editor-card');
    await expect(panel).toBeVisible();
    const box=(await panel.boundingBox())!;
    const nav=(await page.locator('.app-header').boundingBox())!;
    expect(Math.abs(box.x-nav.width-24)).toBeLessThan(2);
    expect(Math.abs(viewport.width-box.x-box.width-24)).toBeLessThan(2);
    expect(Math.abs(viewport.height-box.y-box.height-24)).toBeLessThan(2);
    expect((await page.getByLabel('故事正文').boundingBox())!.width).toBeLessThanOrEqual(1100);
    await page.screenshot({path:testInfo.outputPath(`story-${viewport.width}.png`),fullPage:true});
    for(const url of ['/materials','/tasks','/config']) {
      await page.goto(url);
      const main=(await page.locator('main').boundingBox())!;
      expect(Math.abs(main.width-(viewport.width-nav.width))).toBeLessThan(2);
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
    }
  }
  await page.goto('/');
  expect((await page.locator('main').boundingBox())!.width).toBeLessThanOrEqual(1380);
  await page.setViewportSize({width:390,height:844});
  await page.goto('/studio/new');
  await page.getByLabel('故事正文').fill('窄屏正文仍然可以编辑和保存。');
  await expect(page.locator('.save-indicator')).toHaveText('已保存');
  await page.reload();
  await expect(page.getByLabel('故事正文')).toHaveValue('窄屏正文仍然可以编辑和保存。');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
  await page.screenshot({path:testInfo.outputPath('story-mobile.png'),fullPage:true});
});

test('connection empty state and optional settings release unused columns', async ({page},testInfo) => {
  let connections:ProviderConnection[]=[];
  await page.route('**/api/connections',async route=>{
    if(route.request().method()==='POST') {
      const item=route.request().postDataJSON();
      const saved={...item,credential_set:!!item.api_key};
      delete saved.api_key;
      connections.push(saved);
      await route.fulfill({json:saved});
    } else await route.fulfill({json:{connections}});
  });
  await page.route('**/api/connections/*',async route=>{
    const item=route.request().postDataJSON();
    const saved={...item,credential_set:false};delete saved.api_key;
    connections=connections.map(c=>c.id===saved.id?saved:c);
    await route.fulfill({json:saved});
  });
  await page.route('**/api/models?*',route=>route.fulfill({json:{models:[]}}));
  await page.setViewportSize({width:1920,height:1080});
  await page.goto('/config');
  await expect(page.locator('.catalog-welcome')).toBeVisible();
  expect((await page.locator('.catalog-welcome').boundingBox())!.height).toBeLessThan(280);
  await expect(page.locator('.catalog-layout')).toHaveCount(0);
  await page.screenshot({path:testInfo.outputPath('models-empty.png'),fullPage:true});
  await page.getByRole('button',{name:'添加第一个连接'}).click();
  await page.getByLabel('名称',{exact:true}).fill('取消的连接');
  await page.getByRole('button',{name:'取消',exact:true}).click();
  await expect(page.locator('.catalog-welcome')).toBeVisible();
  await page.getByRole('button',{name:'添加第一个连接'}).click();
  await page.getByLabel('名称',{exact:true}).fill('百炼 · 北京');
  await page.getByLabel('基础 URL').fill('https://example.com');
  await page.getByRole('button',{name:'保存连接'}).click();
  await expect(page.getByRole('heading',{name:'百炼 · 北京'})).toBeVisible();
  const settings=page.locator('.catalog-settings');
  await expect(settings).toBeHidden();
  const full=(await page.locator('.catalog-main').boundingBox())!;
  await page.getByRole('button',{name:'连接设置',exact:true}).click();
  await expect(settings).toBeVisible();
  expect((await page.locator('.catalog-main').boundingBox())!.width).toBeLessThan(full.width-250);
  await page.getByLabel('名称',{exact:true}).fill('百炼 · 编辑后');
  await page.getByRole('button',{name:'收起连接设置'}).click();
  await expect(page.getByRole('button',{name:'连接设置',exact:true})).toBeFocused();
  expect((await page.locator('.catalog-main').boundingBox())!.width).toBeCloseTo(full.width,0);
  await page.getByRole('button',{name:'连接设置',exact:true}).click();
  await expect(page.getByLabel('名称',{exact:true})).toHaveValue('百炼 · 编辑后');
  await page.getByRole('button',{name:'保存连接'}).click();
  await expect(page.getByRole('heading',{name:'百炼 · 编辑后'})).toBeVisible();
  await page.screenshot({path:testInfo.outputPath('models-collapsed.png'),fullPage:true});
  await page.getByRole('button',{name:'连接设置',exact:true}).click();
  await page.screenshot({path:testInfo.outputPath('models-settings.png'),fullPage:true});
  expect((await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
  for(const width of [1366,1024,390]) {
    await page.setViewportSize({width,height:844});
    await expect(settings).toBeVisible();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
    await page.screenshot({path:testInfo.outputPath(`models-settings-${width}.png`),fullPage:true});
  }
  await page.getByRole('button',{name:'添加厂商连接'}).click();
  await page.getByRole('button',{name:'取消',exact:true}).click();
  await expect(page.getByRole('heading',{name:'百炼 · 编辑后'})).toBeVisible();
});
