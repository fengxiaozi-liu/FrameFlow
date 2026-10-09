import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { guideImages } from '../src/components/home/guideImages';

test('complete guide explains all four stages and remains navigable on small screens', async ({page},testInfo)=>{
  await page.emulateMedia({reducedMotion:'reduce'});
  let writes=0;
  page.on('request',request=>{if(request.url().includes('/api/') && request.method()!=='GET')writes++;});
  await page.goto('/');
  await expect(page.getByRole('heading',{name:'四步，完成第一支视频'})).toBeVisible();
  await expect(page.locator('.home-step')).toHaveCount(4);
  await expect(page.locator('.home-step').nth(1)).toContainText('可跳过');
  await page.getByRole('button',{name:'查看入门指南'}).click();
  const guide=page.getByRole('dialog');
  const titles=['配置模型','故事联想','生成分镜','合成视频'];
  for(const width of [1440,390]) {
    await page.setViewportSize({width,height:width===390?844:1000});
    for(let i=0;i<titles.length;i++) {
      await guide.getByRole('navigation',{name:'指南步骤'}).getByRole('button',{name:new RegExp(titles[i]!)}).click();
      await expect(guide.locator('.guide-instructions h3')).toContainText(titles[i]!);
      for(const heading of ['开始前准备','按步骤操作','完成检查','遇到问题']) await expect(guide.getByRole('heading',{name:heading,exact:true})).toBeAttached();
      await expect(guide.locator('.guide-action-list li')).toHaveCount(4);
      await expect(guide.locator('.guide-illustration')).toContainText('操作示意');
      const illustration=guide.locator('.guide-illustration');
      for(let n=0;n<guideImages[i]!.length;n++) {
        await illustration.getByRole('group',{name:'本步操作图片'}).getByRole('button').nth(n).click();
        const picture=illustration.locator('img');
        await expect(picture).toHaveAttribute('alt',guideImages[i]![n]!.alt);
        await expect.poll(()=>picture.evaluate((el:HTMLImageElement)=>el.complete&&el.naturalWidth>0)).toBe(true);
        const zoom=illustration.getByRole('button',{name:`放大查看：${guideImages[i]![n]!.title}`});
        await zoom.click();
        const preview=guide.getByRole('region',{name:'操作图片放大预览',exact:true});
        await expect(preview).toBeVisible();
        await expect(preview.getByRole('button',{name:'返回图文指南'})).toBeFocused();
        await expect(preview.locator('img')).toHaveAttribute('src',await picture.getAttribute('src') as string);
        if(width===390)expect(await guide.evaluate(el=>el.scrollWidth<=el.clientWidth)).toBeTruthy();
        if(i===2&&n===2)await page.screenshot({path:testInfo.outputPath(`guide-zoom-${width}.png`)});
        if(n===0)await page.keyboard.press('Escape');
        else await preview.getByRole('button',{name:'返回图文指南'}).click();
        await expect(preview).toHaveCount(0);
        await expect(zoom).toBeFocused();
      }
      await illustration.getByRole('group',{name:'本步操作图片'}).getByRole('button').first().click();
      await guide.locator('.home-guide-body').evaluate(el=>{el.scrollTop=0;});
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
      expect(await guide.evaluate(el=>el.scrollWidth<=el.clientWidth)).toBeTruthy();
      const footer=(await guide.locator('footer').boundingBox())!;
      expect(footer.y+footer.height).toBeLessThanOrEqual(page.viewportSize()!.height);
      await page.screenshot({path:testInfo.outputPath(`guide-${i+1}-${width}.png`)});
      const faq=guide.locator('.guide-faq details').first();
      await faq.locator('summary').click();
      await expect(faq.locator('p')).toBeVisible();
      if(width===1440)expect((await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
    }
  }
  await guide.getByRole('button',{name:'完成阅读'}).click();
  await expect(guide).toHaveCount(0);
  await expect(page.getByRole('button',{name:'查看入门指南'})).toBeFocused();
  expect(writes).toBe(0);
});

test('guide routes to configuration, manual story and existing project selection without generation', async ({page})=>{
  let generationCalls=0;
  page.on('request',request=>{if(request.method()==='POST' && /candidates|generation-runs|tasks/.test(request.url()))generationCalls++;});
  await page.goto('/');
  await page.getByRole('button',{name:'查看第1步：配置模型'}).click();
  await page.getByRole('dialog').getByRole('button',{name:'去配置模型'}).click();
  await expect(page).toHaveURL(/\/config$/);
  await expect(page.getByRole('heading',{name:'模型管理'})).toBeVisible();
  await page.goto('/');
  await page.evaluate(()=>sessionStorage.setItem('frameflow-idea','上一份示例'));
  await page.getByRole('button',{name:'查看第2步：故事联想'}).click();
  const guide=page.getByRole('dialog');
  await guide.getByRole('button',{name:'已有正文，跳过联想'}).click();
  await expect(page).toHaveURL(/\/studio\/new$/);
  await expect(page.getByLabel('故事正文')).toHaveValue('');
  await page.getByLabel('故事正文').fill('已有故事：雨夜，林夏走进咖啡馆，与老朋友重逢。');
  await expect(page.locator('.save-indicator')).toHaveText('已保存');
  await page.reload();
  await expect(page.getByLabel('故事正文')).toHaveValue(/已有故事/);
  for(const [step,action] of [['生成分镜','去工作台选择项目'],['合成视频','去工作台完成成片']]) {
    await page.goto('/');
    await page.getByRole('button',{name:new RegExp(`查看第.*步：${step}`)}).click();
    await page.getByRole('dialog').getByRole('button',{name:action,exact:true}).click();
    await expect(page).toHaveURL(/\/studio$/);
    await expect(page.getByRole('heading',{name:'创作工作台',exact:true})).toBeVisible();
    expect(await page.evaluate(()=>document.body.style.overflow)).not.toBe('hidden');
  }
  expect(generationCalls).toBe(0);
});
