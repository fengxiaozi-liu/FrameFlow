import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";
import AxeBuilder from "@axe-core/playwright";

test("project delete removes all drafts, supports cancel and stale-state retry, and preserves other projects", async ({
  page,
  request,
}, testInfo) => {
  const project = await (
    await request.post("/api/projects", { data: { name: "删除验收项目" } })
  ).json();
  const keep = await (
    await request.post("/api/projects", { data: { name: "保留项目" } })
  ).json();
  const ids = [randomUUID(), randomUUID()];
  for (let i = 0; i < 2; i++)
    expect(
      (
        await request.post(`/api/projects/${project.id}/drafts`, {
          data: {
            id: ids[i],
            name: i ? "第二份草稿" : "第一份草稿",
            story: { body: "正文", scenes: [] },
          },
        })
      ).ok(),
    ).toBeTruthy();
  await page.route("**/api/projects", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({
      json: {
        projects: data.projects.filter(
          (p: { id: string }) => p.id === project.id,
        ),
      },
    });
  });
  await page.goto("/studio");
  const remove = page.getByRole("button", {
    name: "删除项目 删除验收项目",
    exact: true,
  });
  await expect(page.locator(".workbench-project-item")).toHaveCount(1);
  page.once("dialog", async (dialog) => {
    expect(dialog.message()).toContain("全部 2 个草稿");
    await dialog.dismiss();
  });
  await remove.click();
  expect((await request.get(`/api/projects/${project.id}`)).ok()).toBeTruthy();
  let fail = true;
  await page.route(`**/api/projects/${project.id}`, (route) =>
    route.request().method() === "DELETE" && fail
      ? route.fulfill({
          status: 503,
          json: { error: { message: "删除暂时失败，请重试" } },
        })
      : route.continue(),
  );
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(page.getByRole("alert")).toContainText("删除暂时失败");
  await expect(remove).toBeEnabled();
  fail = false;
  // Editing another draft after the list was loaded invalidates the confirmation snapshot.
  const draft = await (
    await request.get(`/api/projects/${project.id}/drafts/${ids[0]}`)
  ).json();
  expect(
    (
      await request.put(`/api/projects/${project.id}/drafts/${ids[0]}`, {
        data: { body: "其他页面的新内容", expected_version: draft.version },
      })
    ).ok(),
  ).toBeTruthy();
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(page.getByRole("alert")).toContainText("项目已在其他页面更新");
  expect((await request.get(`/api/projects/${project.id}`)).ok()).toBeTruthy();
  await page.getByRole("button", { name: "刷新项目列表" }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("project-delete-mobile.png"),
    fullPage: true,
  });
  expect(
    (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze())
      .violations,
  ).toEqual([]);
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(page.getByRole("status")).toContainText(
    "项目“删除验收项目”已删除",
  );
  expect((await request.get(`/api/projects/${project.id}`)).status()).toBe(404);
  for (const id of ids)
    expect(
      (await request.get(`/api/projects/${project.id}/drafts/${id}`)).status(),
    ).toBe(404);
  expect((await request.get(`/api/projects/${keep.id}`)).ok()).toBeTruthy();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "开始你的第一个故事" }),
  ).toBeVisible();
});

test("single-draft title reflects existing edits and persists new names; empty projects can be deleted", async ({
  page,
  request,
}, testInfo) => {
  const project = await (
    await request.post("/api/projects", { data: { name: "未命名视频" } })
  ).json();
  const id = randomUUID();
  expect(
    (
      await request.post(`/api/projects/${project.id}/drafts`, {
        data: { id, name: "测试视频", story: { body: "已有故事", scenes: [] } },
      })
    ).ok(),
  ).toBeTruthy();
  await page.route("**/api/projects", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({
      json: {
        projects: data.projects.filter(
          (p: { id: string }) => p.id === project.id,
        ),
      },
    });
  });
  await page.goto("/studio");
  await expect(page.locator(".workbench-project-info strong")).toHaveText(
    "测试视频",
  );
  await expect(
    page.getByRole("button", { name: "删除项目 测试视频", exact: true }),
  ).toBeVisible();
  await page.locator(".workbench-project-card").click();
  await page.getByLabel("草稿名称", { exact: true }).fill("测试视频更新");
  await expect(page.locator(".save-indicator")).toHaveText("已保存");
  await expect(page.locator(".studio-meta")).toContainText("测试视频更新");
  await expect
    .poll(
      async () =>
        (await (await request.get(`/api/projects/${project.id}`)).json()).name,
    )
    .toBe("测试视频更新");
  await page
    .getByRole("navigation", { name: "全局导航" })
    .getByRole("link", { name: "创作工作台", exact: true })
    .click();
  await expect(page.locator(".workbench-project-info strong")).toHaveText(
    "测试视频更新",
  );
  await page.reload();
  await expect(page.locator(".workbench-project-info strong")).toHaveText(
    "测试视频更新",
  );
  await page.screenshot({
    path: testInfo.outputPath("renamed-project.png"),
    fullPage: true,
  });
  // Legacy empty containers are visible and deletable rather than hidden forever.
  const empty = await (
    await request.post("/api/projects", { data: { name: "空项目" } })
  ).json();
  await page.unroute("**/api/projects");
  await page.route("**/api/projects", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({
      json: {
        projects: data.projects.filter(
          (p: { id: string }) => p.id === empty.id,
        ),
      },
    });
  });
  await page.reload();
  await expect(page.getByText("暂无草稿", { exact: true })).toBeVisible();
  expect(
    (await request.delete(`/api/projects/${empty.id}`, { data: {} })).status(),
  ).toBe(400);
  page.once("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "删除项目 空项目", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "开始你的第一个故事" }),
  ).toBeVisible();
  expect((await request.get(`/api/projects/${empty.id}`)).status()).toBe(404);
});

test("vendor delete cascades model settings and preserves the other connection", async ({
  page,
  request,
}, testInfo) => {
  const ids = [randomUUID(), randomUUID()];
  for (let i = 0; i < 2; i++) {
    expect(
      (
        await request.post("/api/connections", {
          data: {
            id: ids[i],
            name: i ? "保留厂商" : "待删除厂商",
            vendor: "bailian",
            base_url: "https://example.com",
            api_key: "isolated-test-key",
          },
        })
      ).ok(),
    ).toBeTruthy();
    expect(
      (
        await request.post(`/api/connections/${ids[i]}/models`, {
          data: { model_id: "qwen-plus", capabilities: ["story"] },
        })
      ).ok(),
    ).toBeTruthy();
  }
  await page.route("**/api/connections", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    await route.fulfill({
      json: {
        connections: data.connections.filter((c: { id: string }) =>
          ids.includes(c.id),
        ),
      },
    });
  });
  await page.goto("/config");
  await page.getByRole("button", { name: /待删除厂商.*百炼/ }).click();
  const remove = page.getByRole("button", { name: "删除厂商", exact: true });
  page.once("dialog", (dialog) => dialog.dismiss());
  await remove.click();
  await expect(
    page.getByRole("heading", { name: "待删除厂商", exact: true }),
  ).toBeVisible();
  let fail = true;
  await page.route(`**/api/connections/${ids[0]}`, (route) =>
    fail
      ? route.fulfill({
          status: 409,
          json: {
            error: {
              message: "该厂商有排队或进行中的任务，请完成或取消任务后再删除",
            },
          },
        })
      : route.continue(),
  );
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(page.getByRole("alert")).toContainText("进行中的任务");
  await expect(remove).toBeEnabled();
  fail = false;
  await page.screenshot({
    path: testInfo.outputPath("vendor-delete-desktop.png"),
    fullPage: true,
  });
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(
    page.getByRole("heading", { name: "保留厂商", exact: true }),
  ).toBeVisible();
  expect(
    (await (await request.get(`/api/models?connection_id=${ids[0]}`)).json())
      .models,
  ).toHaveLength(0);
  expect(
    (await (await request.get(`/api/models?connection_id=${ids[1]}`)).json())
      .models,
  ).toHaveLength(1);
  await page.reload();
  await expect(
    page.getByRole("button", { name: /待删除厂商.*百炼/ }),
  ).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("vendor-delete-mobile.png"),
    fullPage: true,
  });
  page.once("dialog", (dialog) => dialog.accept());
  await remove.click();
  await expect(
    page.getByRole("heading", { name: "连接模型，开始 AI 创作" }),
  ).toBeVisible();
});
