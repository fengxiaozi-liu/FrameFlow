import { test, expect } from "@playwright/test";
import { pasteContractJSON } from "./json-contract-helpers";

test("custom discovery is connection-scoped and model JSON contracts persist", async ({
  page,
}, testInfo) => {
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接", exact: true }).click();
  const settings = page.locator("#connection-settings");
  await settings.getByLabel("名称", { exact: true }).fill("自定义契约测试");
  await settings.getByLabel("厂商", { exact: true }).selectOption("custom");
  await settings
    .getByLabel("基础 URL", { exact: true })
    .fill("https://ai-token-admin.xtt.xyz");
  await settings.getByLabel("认证方式").selectOption("none");
  await settings
    .getByLabel("响应 JSON 示例")
    .fill('{"items":[{"id":"demo-video","name":"示例模型"}]}');
  await settings.getByLabel("模型列表 Key").fill("items");
  await expect(settings.getByRole("status")).toContainText("已解析");
  await expect(settings.getByLabel("最终请求地址")).toContainText("GET");
  await page.screenshot({
    path: testInfo.outputPath("connection-discovery.png"),
    fullPage: true,
  });
  await settings.getByRole("button", { name: "保存连接" }).click();
  await expect(page.getByRole("status")).toContainText("连接已保存");
  await expect(settings).not.toBeVisible();
  await expect(page.getByLabel("同步路径")).not.toBeVisible();
  await page.getByRole("button", { name: "连接设置", exact: true }).click();
  await expect(settings.getByLabel("模型列表 Key")).toHaveValue("items");
  await settings.getByRole("button", { name: "收起连接设置" }).click();
  await page.getByRole("button", { name: "手动添加", exact: true }).click();
  await page.getByLabel("模型 ID", { exact: true }).fill("arbitrary-text-id");
  await page.getByRole("button", { name: "添加", exact: true }).click();
  const drawer = page.getByRole("dialog");
  const resizeHandle = drawer.getByRole("separator", {
    name: "调整协议与参数面板宽度",
  });
  const initialWidth = (await drawer.boundingBox())!.width;
  const handle = (await resizeHandle.boundingBox())!;
  await page.mouse.move(
    handle.x + handle.width / 2,
    handle.y + handle.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(handle.x - 140, handle.y + handle.height / 2, {
    steps: 8,
  });
  await page.mouse.up();
  expect((await drawer.boundingBox())!.width).toBeGreaterThan(
    initialWidth + 100,
  );
  await resizeHandle.focus();
  await page.keyboard.press("Home");
  expect((await drawer.boundingBox())!.width).toBeCloseTo(480, 0);
  await page.keyboard.press("End");
  const largestWidth = (await drawer.boundingBox())!.width;
  await page.keyboard.press("ArrowRight");
  expect((await drawer.boundingBox())!.width).toBeCloseTo(largestWidth - 20, 0);
  await drawer.getByLabel("配置能力").selectOption("story");
  await drawer.getByLabel("接口协议").selectOption("custom.json.v1");
  await drawer.getByLabel("请求路径", { exact: true }).fill("/generate");
  await pasteContractJSON(
    drawer,
    "请求",
    '{"model":"demo","input":"demo","temperature":0.5}',
  );
  await drawer
    .getByLabel("请求 model 绑定参数", { exact: true })
    .selectOption("model");
  await drawer
    .getByLabel("请求 input 绑定参数", { exact: true })
    .selectOption("prompt");
  await expect(
    drawer
      .getByLabel("请求 input 绑定参数", { exact: true })
      .locator("option:checked"),
  ).toHaveText("提示词");
  await drawer
    .getByLabel("请求 input 绑定参数", { exact: true })
    .selectOption("");
  await expect(
    drawer.getByLabel("请求 input 固定值", { exact: true }),
  ).toHaveValue("demo");
  await drawer
    .getByLabel("请求 input 绑定参数", { exact: true })
    .selectOption("prompt");
  await drawer
    .getByLabel("请求 temperature 固定值", { exact: true })
    .fill("0.8");
  await drawer
    .getByLabel("请求 temperature 固定值", { exact: true })
    .press("Tab");
  await expect(
    drawer.getByLabel("请求 temperature 绑定参数", { exact: true }),
  ).toHaveValue("");
  await pasteContractJSON(
    drawer,
    "响应",
    '{"answer":"demo","debug":"ignored"}',
  );
  await drawer
    .getByLabel("响应 answer 绑定参数", { exact: true })
    .selectOption("result");
  await expect(
    drawer.getByLabel("响应 debug 绑定参数", { exact: true }),
  ).toHaveValue("");
  await drawer
    .getByLabel("请求 model 绑定参数", { exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("contract-desktop.png"),
    fullPage: true,
  });
  await drawer.getByRole("button", { name: "保存配置" }).click();
  await expect(drawer).not.toBeVisible();
  await page
    .locator(".catalog-table tr")
    .filter({ hasText: "arbitrary-text-id" })
    .getByRole("button", { name: "协议与参数" })
    .click();
  expect(
    JSON.parse(
      await drawer.getByLabel("请求 JSON 示例", { exact: true }).inputValue(),
    ),
  ).toEqual({ model: "demo", input: "demo", temperature: 0.8 });
  await expect(
    drawer.getByLabel("请求 input 绑定参数", { exact: true }),
  ).toHaveValue("prompt");
  await expect(
    drawer.getByLabel("请求 temperature 绑定参数", { exact: true }),
  ).toHaveValue("");
  await expect(
    drawer.getByLabel("响应 debug 绑定参数", { exact: true }),
  ).toHaveValue("");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(resizeHandle).toHaveCount(0);
  await expect.poll(async () => (await drawer.boundingBox())!.width).toBe(390);
  await drawer
    .getByLabel("请求 model 绑定参数", { exact: true })
    .scrollIntoViewIfNeeded();
  expect(
    await drawer.evaluate(
      (element) => element.scrollWidth <= element.clientWidth,
    ),
  ).toBeTruthy();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("contract-mobile.png"),
    fullPage: true,
  });
  await page.keyboard.press("Escape");
  await expect(drawer).not.toBeVisible();
  await expect(
    page
      .locator(".catalog-table tr")
      .filter({ hasText: "arbitrary-text-id" })
      .getByRole("button", { name: "协议与参数" }),
  ).toBeFocused();
});

test("unknown remote acceptance disables retry and explains recovery", async ({
  page,
}) => {
  await page.route("**/api/tasks?**", (route) =>
    route.fulfill({
      json: {
        tasks: [
          {
            id: "unknown-acceptance",
            kind: "video",
            status: "failed",
            progress: 0,
            stage: "failed",
            remote_acceptance_unknown: true,
            retry_count: 0,
            input: { prompt: "远端状态验证" },
            error: "远端受理状态未知",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ],
      },
    }),
  );
  await page.goto("/tasks");
  await expect(
    page.getByRole("button", { name: "重新执行", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByText(
      "远端受理状态未知，请先在服务商确认任务状态，避免重复生成。",
      { exact: true },
    ),
  ).toBeVisible();
});

test("nested JSON fields keep parent groups, local keys and persisted bindings", async ({
  page,
  request,
}, testInfo) => {
  const id = `nested-${Date.now()}`;
  await request.post("/api/connections", {
    data: {
      id,
      name: "嵌套字段测试",
      vendor: "custom",
      base_url: "https://example.com",
    },
  });
  await request.post(`/api/connections/${id}/models`, {
    data: {
      model_id: "nested-model",
      capabilities: ["story"],
    },
  });
  await page.goto("/config");
  await page
    .getByRole("button", {
      name: "嵌套字段测试 custom · https://example.com",
      exact: true,
    })
    .click();
  const row = page.getByRole("row").filter({ hasText: "nested-model" });
  await row.getByRole("button", { name: "协议与参数" }).click();
  const drawer = page.getByRole("dialog");
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("custom.json.v1");
  await drawer.getByLabel("请求路径", { exact: true }).fill("/generate");
  await pasteContractJSON(
    drawer,
    "请求",
    JSON.stringify({
      model: "demo",
      list: { a: { c: "d" }, b: "" },
      items: [{ note: "示例" }],
    }),
  );
  const list = drawer.locator('[data-field-pointer="/list"]');
  const child = list.locator('[data-field-pointer="/list/a/c"]');
  await expect(child.locator(".leaf-key")).toHaveText("c");
  await drawer
    .getByLabel("请求 list.a.c 固定值", { exact: true })
    .fill("示例提示词");
  await drawer.getByLabel("请求 list.a.c 固定值", { exact: true }).press("Tab");
  await drawer
    .getByLabel("请求 list.a.c 绑定参数", { exact: true })
    .selectOption("prompt");
  await drawer
    .getByLabel("请求 list.b 固定值", { exact: true })
    .fill("固定内容");
  await drawer.getByLabel("请求 list.b 固定值", { exact: true }).press("Tab");
  await drawer
    .getByRole("button", { name: "请求 list.a 收起", exact: true })
    .press("Enter");
  await expect(child).not.toBeVisible();
  await expect(list.locator('[data-field-pointer="/list/b"]')).toBeVisible();
  await drawer
    .getByRole("button", { name: "请求 list.a 展开", exact: true })
    .press("Space");
  await expect(child).toBeVisible();
  await drawer
    .getByRole("button", { name: "请求 list 收起", exact: true })
    .click();
  await expect(child).not.toBeVisible();
  await drawer
    .getByRole("button", { name: "请求 list 展开", exact: true })
    .click();
  await expect(child).toBeVisible();
  await expect(
    drawer.locator('[data-field-pointer="/items/0"] .branch-toggle code'),
  ).toHaveText("[0]");
  await pasteContractJSON(drawer, "响应", '{"payload":{"answer":"demo"}}');
  await drawer
    .getByLabel("响应 payload.answer 绑定参数", { exact: true })
    .selectOption("result");
  const requestFields = drawer.getByRole("group", {
    name: "请求字段",
    exact: true,
  });
  await requestFields.screenshot({
    path: testInfo.outputPath("nested-fields-desktop.png"),
  });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await drawer.evaluate(
      (element) => element.scrollWidth <= element.clientWidth,
    ),
  ).toBeTruthy();
  await requestFields.screenshot({
    path: testInfo.outputPath("nested-fields-mobile.png"),
  });
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(
    drawer.getByLabel("请求 list.a.c 绑定参数", { exact: true }),
  ).toHaveValue("prompt");
  await expect(
    drawer.getByLabel("请求 list.b 固定值", { exact: true }),
  ).toHaveValue("固定内容");
  await expect(
    drawer.getByLabel("响应 payload.answer 绑定参数", { exact: true }),
  ).toHaveValue("result");
  const example = JSON.parse(
    await drawer.getByLabel("请求 JSON 示例", { exact: true }).inputValue(),
  );
  expect(example.list).toEqual({ a: { c: "示例提示词" }, b: "固定内容" });
});
