import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("vendor determines synchronization without an extra setting", async ({
  page,
  request,
}, testInfo) => {
  const name = `基础地址测试 ${randomUUID()}`;
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接" }).click();
  await page.getByLabel("名称", { exact: true }).fill(name);
  const address = page.getByLabel("基础 URL", { exact: true });
  await address.fill("https://example.com:8443/v1/chat/completions");
  await expect(page.getByRole("alert")).toContainText("不要包含接口路径");
  await address.fill("https://example.com:8443/");
  await expect(page.getByLabel("模型同步方式")).toHaveCount(0);
  await expect(page.getByLabel("最终请求地址")).toHaveCount(0);
  await page.getByLabel("API Key", { exact: true }).fill("isolated-test-key");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "同步模型", exact: true })).toBeEnabled();
  await expect(page.getByRole("button", { name: "测试连接", exact: true })).toBeEnabled();
  const connections = (await (await request.get("/api/connections")).json()).connections;
  const connection = connections.find((c: { name: string }) => c.name === name);
  expect(connection.base_url).toBe("https://example.com:8443");
  expect(connection.vendor).toBe("bailian");
  expect(connection).not.toHaveProperty("discovery_protocol");
  await page.route(`**/api/connections/${connection.id}/models/sync`, route => route.fulfill({ json: { synced: 2 } }));
  await page.getByRole("button", { name: "同步模型", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("同步完成，发现 2 个模型");
  await page.getByRole("button", { name: "手动添加", exact: true }).click();
  await page.getByLabel("模型 ID", { exact: true }).fill("unfamiliar-model");
  await page
    .getByRole("group", { name: "支持的能力" })
    .getByLabel("图生视频")
    .check();
  await page.getByRole("button", { name: "添加", exact: true }).click();
  const drawer = page.getByRole("dialog");
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("dashscope.media-video.v1");
  await expect(drawer.getByLabel("最终请求地址")).toContainText(
    "https://example.com:8443/api/v1/services/aigc/video-generation/video-synthesis",
  );
  await expect(drawer.getByLabel("最终请求地址")).toContainText(
    "https://example.com:8443/api/v1/tasks/{task_id}",
  );
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("custom.json.v1");
  await drawer.getByLabel("生成请求路径", { exact: true }).fill("/v2/render");
  await drawer.getByLabel("返回方式", { exact: true }).selectOption("async");
  await drawer
    .getByLabel("查询请求路径 · GET", { exact: true })
    .fill("/v2/jobs/{task_id}");
  await expect(drawer.getByLabel("最终请求地址")).toContainText(
    "https://example.com:8443/v2/render",
  );
  await expect(drawer.getByLabel("最终请求地址")).toContainText(
    "https://example.com:8443/v2/jobs/{task_id}",
  );
  await drawer.getByLabel("最终请求地址").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("custom-address-preview.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await drawer.getByLabel("最终请求地址").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("custom-address-mobile.png"),
    fullPage: true,
  });
  await drawer.getByRole("button", { name: "取消", exact: true }).click();
  await page.getByRole("button", { name: "连接设置", exact: true }).click();
  await expect(page.getByLabel("模型同步方式")).toHaveCount(0);
  await expect(page.getByLabel("基础 URL", { exact: true })).toHaveValue("https://example.com:8443");
  await page.getByRole("button", { name: "保存连接" }).click();
  await page.reload();
  await page.getByRole("button", { name, exact: false }).click();
  await page.getByRole("button", { name: "连接设置", exact: true }).click();
  await expect(page.getByLabel("模型同步方式")).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("origin-settings-mobile.png"),
    fullPage: true,
  });
});
