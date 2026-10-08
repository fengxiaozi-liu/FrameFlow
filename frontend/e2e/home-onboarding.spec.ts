import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

test("home carousel, keyboard guide, example handoff and new blank creation", async ({
  page,
}, testInfo) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.route("**/api/projects", (route) =>
    route.fulfill({ json: { projects: [] } }),
  );
  await page.route("**/api/models*", (route) =>
    route.fulfill({ json: { models: [] } }),
  );
  let writes = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/") && request.method() === "POST")
      writes++;
  });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: /把你的故事，\s*变成画面/ }),
  ).toBeVisible();
  await expect(page.getByRole("heading", { name: "继续创作" })).toHaveCount(0);
  await expect(page.locator(".home-model-hint")).toContainText("首次使用");
  await expect(page.getByRole("button", { name: "播放轮播" })).toBeVisible();
  await expect
    .poll(() =>
      page
        .locator(".home-showcase > img:visible")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await page.screenshot({
    path: testInfo.outputPath("home-desktop.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "上一个示例" }).click();
  await expect(page.locator(".home-slide-caption h2")).toHaveText(
    "森林里的小旅人",
  );
  await page.getByRole("button", { name: "下一个示例" }).click();
  await expect(page.locator(".home-slide-caption h2")).toHaveText("雨夜咖啡馆");
  await page.getByRole("button", { name: "查看示例：城市的一天" }).click();
  await expect(page.locator(".home-slide-caption h2")).toHaveText("城市的一天");
  expect(
    (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
      .violations,
  ).toEqual([]);
  const trigger = page.getByRole("button", { name: "查看第2步：组织分镜" });
  await trigger.click();
  const guide = page.getByRole("dialog");
  await expect(guide.getByRole("heading", { name: "组织分镜" })).toBeVisible();
  await expect(
    guide.getByRole("button", { name: "关闭入门指南" }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(guide.getByRole("button", { name: "下一步" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(guide).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await page.getByRole("button", { name: "查看入门指南" }).click();
  await expect(guide.getByRole("heading", { name: "写下故事" })).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("home-guide-desktop.png"),
  });
  expect(
    (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
      .violations,
  ).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("home-guide-mobile.png"),
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await page.screenshot({
    path: testInfo.outputPath("home-mobile.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "查看第3步：生成视频" }).click();
  await guide.getByRole("button", { name: "使用示例想法" }).click();
  await expect(page).toHaveURL(/\/studio\/new$/);
  await page.getByRole("button", { name: "故事联想", exact: true }).click();
  await expect(page.getByLabel("你的想法或修改要求")).toHaveValue(/城市的一天/);
  expect(writes).toBe(0);
  await page.getByRole("button", { name: "关闭故事联想" }).click();
  await page.goto("/");
  await page.getByRole("button", { name: "开始创作", exact: true }).click();
  await page.getByRole("button", { name: "故事联想", exact: true }).click();
  await expect(page.getByLabel("你的想法或修改要求")).toHaveValue("");
});

test("home stays focused on guidance; workbench project failures recover and preserve drafts", async ({
  page,
}, testInfo) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  let fail = true;
  let projectRequests = 0;
  page.on("request", request => {if(new URL(request.url()).pathname === "/api/projects") projectRequests++;});
  const draft = (id: string, date: string) => ({
    id,
    name: `草稿 ${id}`,
    story: { body: "已保存的正文", scenes: [], updated_at: date },
    updated_at: date,
  });
  const projects = Array.from({ length: 5 }, (_, i) => ({
    id: `home-${i}`,
    name: i === 0 ? "雨夜咖啡馆" : `城市创作 ${i}`,
    created_at: "2026-09-01",
    drafts: [draft("old", "2026-09-01"), draft("latest", `2026-10-0${5 - i}`)],
  }));
  await page.route("**/api/projects", (route) =>
    fail
      ? route.fulfill({ status: 503, json: {} })
      : route.fulfill({ json: { projects } }),
  );
  await page.route("**/api/models*", (route) =>
    fail
      ? route.fulfill({ status: 503, json: {} })
      : route.fulfill({
          json: {
            models: [
              {
                id: "ready",
                enabled: true,
                supported: true,
                capabilities: ["story"],
              },
              {
                id: "disabled",
                enabled: false,
                supported: true,
                capabilities: ["image", "video"],
              },
            ],
          },
        }),
  );
  await page.goto("/");
  await expect(page.getByRole("heading", {name:"继续创作"})).toHaveCount(0);
  await expect(page.locator(".home-model-hint")).toContainText("暂时无法读取");
  await expect(
    page.getByRole("button", { name: "开始创作", exact: true }),
  ).toBeEnabled();
  fail = false;
  await page.getByRole("button", { name: "重新检查" }).click();

  await expect(page.locator(".home-model-hint")).toContainText(
    "已准备好 故事辅助；参考图生成、视频生成可按需配置",
  );
  expect(projectRequests).toBe(0);
  await page.screenshot({path:testInfo.outputPath('home-without-projects.png'),fullPage:true});
  fail = true;
  await page.getByRole('navigation',{name:'全局导航'}).getByRole('link',{name:'创作工作台',exact:true}).click();
  await expect(page).toHaveURL(/\/studio$/);
  await expect(page.getByText('暂时无法加载草稿，请重试。')).toBeVisible();
  fail = false;
  await page.getByRole('button',{name:'重新加载草稿'}).click();
  await expect(page.locator(".workbench-project-card")).toHaveCount(4);
  await expect(page.locator(".workbench-project-card").first()).toHaveAttribute(
    "href",
    "/studio/home-0/latest",
  );
  await expect(page.locator(".workbench-project-card").first()).toContainText(
    "2 个草稿 · 0 个分镜",
  );
  await page.screenshot({
    path: testInfo.outputPath("workbench-projects-desktop.png"),
    fullPage: true,
  });
  await page.getByRole("button", { name: "全部项目（5）" }).click();
  await expect(page.locator(".workbench-project-card")).toHaveCount(5);
  await page.getByRole("button", { name: "收起项目" }).click();
  await expect(page.locator(".workbench-project-card")).toHaveCount(4);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.route("**/api/projects/home-0", (route) =>
    route.fulfill({ json: projects[0] }),
  );
  await page.screenshot({path:testInfo.outputPath('workbench-projects-mobile.png'),fullPage:true});
  expect((await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa']).analyze()).violations).toEqual([]);
  await page.locator(".workbench-project-card").first().click();
  await expect(page).toHaveURL(/\/studio\/home-0\/latest$/);
  await expect(page.getByLabel("故事正文")).toHaveValue("已保存的正文");
  await page.reload();
  await expect(page.getByLabel("故事正文")).toHaveValue("已保存的正文");
  await page.getByRole('navigation',{name:'全局导航'}).getByRole('link',{name:'创作工作台',exact:true}).click();
  await page.getByRole('button',{name:'＋ 新建创作',exact:true}).click();
  await expect(page).toHaveURL(/\/studio\/new$/);
  await expect(page.getByLabel('故事正文')).toHaveValue('');
});

test("carousel respects pause, focus and reduced motion without background writes", async ({
  page,
}) => {
  await page.clock.install();
  await page.route("**/api/projects", (route) =>
    route.fulfill({ json: { projects: [] } }),
  );
  await page.route("**/api/models*", (route) =>
    route.fulfill({ json: { models: [] } }),
  );
  await page.goto("/");
  const title = page.locator(".home-slide-caption h2");
  await page.clock.fastForward(7100);
  await expect(title).toHaveText("城市的一天");
  await page.getByRole("button", { name: "下一个示例" }).focus();
  await page.clock.fastForward(7100);
  await expect(title).toHaveText("城市的一天");
  await page.getByRole("button", { name: "开始创作", exact: true }).focus();
  await page.clock.fastForward(7100);
  await expect(title).toHaveText("森林里的小旅人");
  await page.getByRole("button", { name: "暂停轮播" }).click();
  await page.getByRole("button", { name: "开始创作", exact: true }).focus();
  await page.mouse.move(0, 0);
  await page.clock.fastForward(7100);
  await expect(title).toHaveText("森林里的小旅人");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.reload();
  await expect(page.getByRole("button", { name: "播放轮播" })).toBeVisible();
  await page.clock.fastForward(14000);
  await expect(title).toHaveText("雨夜咖啡馆");
});

test('empty workbench offers creation without writing a project on navigation', async ({page})=>{
  let writes=0;
  page.on('request',request=>{if(request.method()!=='GET')writes++;});
  await page.route('**/api/projects',route=>route.fulfill({json:{projects:[]}}));
  await page.goto('/studio');
  await expect(page.getByRole('heading',{name:'开始你的第一个故事'})).toBeVisible();
  await page.reload();
  await expect(page.getByRole('heading',{name:'开始你的第一个故事'})).toBeVisible();
  await page.getByRole('button',{name:'开始创作',exact:true}).click();
  await expect(page).toHaveURL(/\/studio\/new$/);
  await expect(page.getByLabel('故事正文')).toHaveValue('');
  expect(writes).toBe(0);
});
