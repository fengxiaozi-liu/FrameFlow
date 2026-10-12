import { test, expect } from "@playwright/test";
import { pasteContractJSON } from "./json-contract-helpers";
import { randomUUID } from "node:crypto";

test("manual unknown video model uses explicit protocol and parameter choices", async ({
  page,
  request,
}, testInfo) => {
  const id = randomUUID();
  expect(
    (
      await request.post("/api/connections", {
        data: {
          id,
          name: `兼容测试 ${id}`,
          vendor: "custom",
          base_url: "https://example.com",
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto("/config");
  await page.getByRole("button", { name: `兼容测试 ${id}` }).click();
  await page.getByRole("button", { name: "手动添加", exact: true }).click();
  await page.getByLabel("模型 ID", { exact: true }).fill("Wan3.0-video");
  await page.getByLabel("显示名称", { exact: true }).fill("自定义视频");
  await page
    .locator(".catalog-add")
    .getByLabel("图生视频", { exact: true })
    .check();
  await page
    .locator(".catalog-add")
    .getByRole("button", { name: "添加", exact: true })
    .click();
  const drawer = page.getByRole("dialog");
  await expect(drawer).toBeVisible();
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("dashscope.media-video.v1");
  await drawer.getByLabel("16:9", { exact: true }).check();
  await drawer.getByLabel("首尾帧模式", { exact: true }).check();
  const durations = drawer.getByRole("group", {
    name: "可选时长",
    exact: true,
  });
  const resolutions = drawer.getByRole("group", {
    name: "可选清晰度",
    exact: true,
  });
  await durations.getByLabel("最短时长（秒）", { exact: true }).fill("4");
  await durations.getByLabel("最长时长（秒）", { exact: true }).fill("8");
  await expect(durations).toContainText("4–8 秒");
  await resolutions.getByRole("button", { name: "720P", exact: true }).click();
  await resolutions.getByRole("button", { name: "1080P", exact: true }).click();
  await resolutions.getByLabel("添加可选清晰度", { exact: true }).fill("1440p");
  await resolutions
    .getByRole("button", { name: "添加可选清晰度值", exact: true })
    .click();
  await expect(
    resolutions.getByRole("button", { name: "移除1440P", exact: true }),
  ).toBeVisible();
  await resolutions
    .getByRole("button", { name: "移除1440P", exact: true })
    .click();
  await expect(resolutions.getByRole("listitem").locator("span")).toHaveText([
    "720P",
    "1080P",
  ]);
  await drawer.getByLabel("单张图片上限（MB）", { exact: true }).fill("10");
  await page.screenshot({
    path: testInfo.outputPath("model-protocol-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("model-protocol-mobile.png"),
    fullPage: true,
  });
  await durations.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("model-option-tags-mobile.png"),
  });
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "自定义视频" }),
  ).toContainText("配置就绪");
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page
    .getByRole("row")
    .filter({ hasText: "自定义视频" })
    .getByRole("button", { name: "协议与参数" })
    .click();
  await expect(
    durations.getByLabel("最短时长（秒）", { exact: true }),
  ).toHaveValue("4");
  await expect(
    durations.getByLabel("最长时长（秒）", { exact: true }),
  ).toHaveValue("8");
  await expect(resolutions.getByRole("listitem").locator("span")).toHaveText([
    "720P",
    "1080P",
  ]);
  await durations.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("model-option-tags-desktop.png"),
  });
  await drawer.getByRole("button", { name: "取消", exact: true }).click();

  const project = await (
    await request.post("/api/projects", { data: { name: "自定义模型测试" } })
  ).json();
  const draftID = randomUUID();
  expect(
    (
      await request.post(`/api/projects/${project.id}/drafts`, {
        data: {
          id: draftID,
          name: "草稿",
          story: {
            title: "",
            body: "测试",
            scenes: [
              {
                id: "shot",
                order: 1,
                title: "雨夜",
                visual_prompt: "雨夜街道",
                narration: "",
                duration_seconds: 6,
              },
            ],
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(`/studio/${project.id}/${draftID}/shots/shot`);
  const settings = page.getByRole("region", { name: "视频设置" });
  await settings
    .getByLabel("视频模型", { exact: true })
    .selectOption(`${id}:Wan3.0-video`);
  await expect(settings.getByLabel("时长", { exact: true })).toHaveValue("6");
  await expect(
    settings.getByLabel("时长", { exact: true }).locator("option"),
  ).toHaveText(["4 秒", "5 秒", "6 秒", "7 秒", "8 秒"]);
  await expect(
    settings.getByLabel("清晰度", { exact: true }).locator("option"),
  ).toHaveText(["720P", "1080P"]);
  await settings.getByLabel("时长", { exact: true }).selectOption("8");
  await settings.getByLabel("清晰度", { exact: true }).selectOption("1080P");
  await expect(settings).toContainText("请选择当前分镜引用中的首帧图片");
  await expect(page.getByRole("status").first()).toHaveText("已保存");
  await page.reload();
  await expect(settings.getByLabel("时长", { exact: true })).toHaveValue("8");
  await expect(settings.getByLabel("清晰度", { exact: true })).toHaveValue(
    "1080P",
  );
  await page.screenshot({
    path: testInfo.outputPath("configured-video-options.png"),
    fullPage: true,
  });
});

test("model without declared capabilities can be configured directly in the drawer", async ({
  page,
  request,
}, testInfo) => {
  const id = randomUUID();
  expect(
    (
      await request.post("/api/connections", {
        data: {
          id,
          name: `空能力 ${id}`,
          vendor: "custom",
          base_url: "https://example.com",
        },
      })
    ).ok(),
  ).toBeTruthy();
  expect(
    (
      await request.post(`/api/connections/${id}/models`, {
        data: { model_id: "unclassified-image", capabilities: [] },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto("/config");
  await page.getByRole("button", { name: `空能力 ${id}` }).click();
  const row = page.getByRole("row").filter({ hasText: "unclassified-image" });
  await row.getByRole("button", { name: "协议与参数" }).click();
  const drawer = page.getByRole("dialog");
  const capability = drawer.getByLabel("配置能力", { exact: true });
  const protocol = drawer.getByLabel("接口协议", { exact: true });
  await expect(capability).toHaveValue("");
  await expect(capability.locator("option")).toHaveText([
    "请选择需要配置的能力",
    "文本生成",
    "图片生成",
    "视频生成",
    "语音合成",
  ]);
  await expect(protocol).toBeDisabled();
  for (const [operation, protocolID] of [
    ["story", "openai.chat.v1"],
    ["image", "dashscope.image.v1"],
    ["video", "dashscope.media-video.v1"],
    ["speech", "dashscope.speech.v1"],
  ]) {
    await capability.selectOption(operation!);
    await expect(protocol.locator("option")).toHaveCount(3);
    await expect(protocol.locator("option").nth(1)).toHaveAttribute(
      "value",
      protocolID!,
    );
  }
  await capability.selectOption("image");
  await protocol.selectOption("dashscope.image.v1");
  await drawer.getByLabel("1:1", { exact: true }).check();
  await drawer.getByLabel("1:1 输出尺寸", { exact: true }).fill("1024*1024");
  await expect(drawer.getByLabel("文生图", { exact: true })).toBeChecked();
  await expect(drawer.getByLabel("图生图", { exact: true })).not.toBeChecked();
  await expect(drawer.getByLabel("最多参考图片数", { exact: true })).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath("unclassified-model-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("unclassified-model-mobile.png"),
    fullPage: true,
  });
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(row.getByLabel("图片生成", { exact: true })).toBeChecked();
  await expect(row.getByLabel("文本生成", { exact: true })).not.toBeChecked();
  await expect(row.getByLabel("启用", { exact: true })).not.toBeChecked();
  await row.getByLabel("启用", { exact: true }).check();
  await expect(row).toContainText("配置就绪");
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(capability).toHaveValue("image");
  await expect(protocol).toHaveValue("dashscope.image.v1");
});

test("custom interface replaces path-only editing and persists its format", async ({
  page,
  request,
}, testInfo) => {
  const id = randomUUID();
  await request.post("/api/connections", {
    data: {
      id,
      name: `自定义格式 ${id}`,
      vendor: "custom",
      base_url: "https://example.com",
    },
  });
  await request.post(`/api/connections/${id}/models`, {
    data: { model_id: "qwen-plus", capabilities: ["story"] },
  });
  await page.goto("/config");
  await page.getByRole("button", { name: `自定义格式 ${id}` }).click();
  const row = page.getByRole("row").filter({ hasText: "qwen-plus" });
  await row.getByRole("button", { name: "协议与参数" }).click();
  const drawer = page.getByRole("dialog");
  await expect(
    drawer.getByLabel("实际请求路径", { exact: true }),
  ).toContainText("/compatible-mode/v1/chat/completions");
  await expect(drawer.getByRole("radio")).toHaveCount(0);
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("custom.json.v1");
  await expect(drawer.getByLabel("实际请求路径", { exact: true })).toHaveCount(
    0,
  );
  await drawer.getByLabel("请求路径", { exact: true }).fill("/v1/custom/chat");
  const template = JSON.stringify(
    {
      engine: "{{model}}",
      question: "{{prompt}}",
      context: "{{system_prompt}}",
      temperature: 0.5,
    },
    null,
    2,
  );
  await pasteContractJSON(drawer, "请求", template);
  await drawer
    .getByLabel("请求 engine 绑定参数", { exact: true })
    .selectOption("model");
  await drawer
    .getByLabel("请求 question 绑定参数", { exact: true })
    .selectOption("prompt");
  await drawer
    .getByLabel("请求 context 绑定参数", { exact: true })
    .selectOption("system_prompt");
  await pasteContractJSON(drawer, "响应", '{"payload":{"text":"demo"}}');
  await drawer
    .getByLabel("响应 payload.text 绑定参数", { exact: true })
    .selectOption("result");

  await page.screenshot({
    path: testInfo.outputPath("custom-interface-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("custom-interface-mobile.png"),
    fullPage: true,
  });
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await page.reload();
  await page.getByRole("button", { name: `自定义格式 ${id}` }).click();
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(drawer.getByLabel("接口协议", { exact: true })).toHaveValue(
    "custom.json.v1",
  );
  await expect(
    drawer.getByLabel("请求 JSON 示例", { exact: true }),
  ).toHaveValue(template);
  await expect(drawer.getByLabel("请求路径", { exact: true })).toHaveValue(
    "/v1/custom/chat",
  );

  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("openai.chat.v1");
  await expect(
    drawer.getByLabel("请求 JSON 示例", { exact: true }),
  ).toHaveCount(0);
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  const result = await (await request.get("/api/models")).json();
  const model = result.models.find(
    (m: { connection_id: string }) => m.connection_id === id,
  );
  expect(model.operations.story.custom).toBeUndefined();
  expect(model.operations.story.endpoint_mode).toBe("default");
});

test("legacy builtin path remains explicit and can restore default", async ({
  page,
  request,
}) => {
  const id = randomUUID();
  await request.post("/api/connections", {
    data: {
      id,
      name: `历史路径 ${id}`,
      vendor: "custom",
      base_url: "https://example.com",
    },
  });
  await request.post(`/api/connections/${id}/models`, {
    data: { model_id: "qwen-plus", capabilities: ["story"] },
  });
  const all = await (await request.get("/api/models")).json();
  const m = all.models.find(
    (m: { connection_id: string }) => m.connection_id === id,
  );
  expect(
    (
      await request.put(`/api/models/${encodeURIComponent(m.id)}/operations`, {
        data: {
          expected_revision: m.revision,
          operations: {
            story: {
              protocol: "openai.chat.v1",
              source: "manual",
              endpoint: "/legacy/chat",
            },
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto("/config");
  await page.getByRole("button", { name: `历史路径 ${id}` }).click();
  await page
    .getByRole("row")
    .filter({ hasText: "qwen-plus" })
    .getByRole("button", { name: "协议与参数" })
    .click();
  const drawer = page.getByRole("dialog");
  await expect(drawer).toContainText("历史路径配置");
  await expect(
    drawer.getByLabel("实际请求路径", { exact: true }),
  ).toContainText("/legacy/chat");
  await drawer.getByRole("button", { name: "恢复协议默认路径" }).click();
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  const updated = await (await request.get("/api/models")).json();
  expect(
    updated.models.find((entry: { id: string }) => entry.id === m.id).operations
      .story.endpoint,
  ).toBeUndefined();
});

test("custom async video fields persist and duplicate terminal states are rejected", async ({
  page,
  request,
}, testInfo) => {
  const id = randomUUID();
  await request.post("/api/connections", {
    data: {
      id,
      name: `自定义异步 ${id}`,
      vendor: "custom",
      base_url: "https://example.com",
    },
  });
  await request.post(`/api/connections/${id}/models`, {
    data: { model_id: "opaque-video", capabilities: ["video"] },
  });
  await page.goto("/config");
  await page.getByRole("button", { name: `自定义异步 ${id}` }).click();
  const row = page.getByRole("row").filter({ hasText: "opaque-video" });
  await row.getByRole("button", { name: "协议与参数" }).click();
  const drawer = page.getByRole("dialog");
  await drawer
    .getByLabel("接口协议", { exact: true })
    .selectOption("custom.json.v1");
  await drawer.getByLabel("请求路径", { exact: true }).fill("/renders");
  await expect(
    drawer.getByLabel("请求 images 绑定参数", { exact: true }),
  ).toHaveValue("images");
  await expect(
    drawer
      .getByLabel("请求 images 绑定参数", { exact: true })
      .locator("option:checked"),
  ).toHaveText("智能参考图片（多图）");

  await drawer
    .getByLabel("执行方式", { exact: true })
    .selectOption("async_polling");
  await pasteContractJSON(
    drawer,
    "请求",
    '{"model":"demo","prompt":"demo","images":["https://cdn.example.com/image.png"]}',
  );
  await drawer
    .getByLabel("请求 model 绑定参数", { exact: true })
    .selectOption("model");
  await drawer
    .getByLabel("请求 prompt 绑定参数", { exact: true })
    .selectOption("prompt");
  await drawer
    .getByLabel("请求 images 绑定参数", { exact: true })
    .selectOption("images");
  await expect(
    drawer.getByLabel("请求 images.0 绑定参数", { exact: true }),
  ).toBeDisabled();
  await pasteContractJSON(drawer, "受理响应", '{"job":{"id":"demo"}}');
  await drawer
    .getByLabel("受理响应 job.id 绑定参数", { exact: true })
    .selectOption("task_id");
  await pasteContractJSON(
    drawer,
    "查询响应",
    '{"status":"done","result":{"video":"https://cdn.example.com/video.mp4"}}',
  );
  await drawer
    .getByLabel("查询响应 result.video 绑定参数", { exact: true })
    .selectOption("result");
  await drawer
    .getByLabel("查询路径", { exact: true })
    .fill("/renders/{task_id}");

  for (const [label, value] of [["失败状态值", "succeeded"]]) {
    const input = drawer.getByLabel(`添加${label}`, { exact: true });
    await input.fill(value!);
    await input.press("Enter");
  }
  await drawer.getByLabel("16:9", { exact: true }).check();
  await drawer.getByLabel("智能参考（多图）", { exact: true }).check();
  await drawer.getByLabel("最多参考图片数", { exact: true }).fill("2");
  await drawer.getByLabel("单张图片上限（MB）", { exact: true }).fill("10");
  await drawer.getByLabel("最短时长（秒）", { exact: true }).fill("4");
  await drawer.getByLabel("最长时长（秒）", { exact: true }).fill("4");
  await drawer.getByRole("button", { name: "720P", exact: true }).click();
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer.getByRole("alert")).toContainText(
    "状态值不能为空或在多个分组重复",
  );
  const failed = drawer.getByRole("group", {
    name: "失败状态值",
    exact: true,
  });
  await failed
    .getByRole("button", { name: "移除succeeded", exact: true })
    .click();
  await failed.getByLabel("添加失败状态值", { exact: true }).fill("failed");
  await failed.getByLabel("添加失败状态值", { exact: true }).press("Enter");
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(drawer.getByLabel("执行方式", { exact: true })).toHaveValue(
    "async_polling",
  );
  await expect(drawer.getByLabel("查询路径", { exact: true })).toHaveValue(
    "/renders/{task_id}",
  );
  await drawer
    .getByLabel("查询响应 status 绑定参数", { exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("custom-async-video.png"),
  });
});
