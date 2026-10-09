import { test, expect } from "@playwright/test";
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
          base_url: "https://example.com/v1",
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
  await drawer
    .getByLabel("图片输入方式", { exact: true })
    .selectOption("first_frame");
  await drawer.getByLabel("可选时长（秒）", { exact: true }).fill("4, 8");
  await drawer.getByLabel("可选清晰度", { exact: true }).fill("720P, 1080P");
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
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await expect(
    page.getByRole("row").filter({ hasText: "自定义视频" }),
  ).toContainText("配置就绪");
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
  await expect(settings.getByLabel("时长", { exact: true })).toHaveValue("4");
  await expect(
    settings.getByLabel("时长", { exact: true }).locator("option"),
  ).toHaveText(["4 秒", "8 秒"]);
  await expect(
    settings.getByLabel("清晰度", { exact: true }).locator("option"),
  ).toHaveText(["720P", "1080P"]);
  await settings.getByLabel("时长", { exact: true }).selectOption("8");
  await settings.getByLabel("清晰度", { exact: true }).selectOption("1080P");
  await expect(settings).toContainText("一张引用图片作为首帧");
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
          base_url: "https://example.com/v1",
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
    await expect(protocol.locator("option")).toHaveCount(2);
    await expect(protocol.locator("option").nth(1)).toHaveAttribute(
      "value",
      protocolID!,
    );
  }
  await capability.selectOption("image");
  await protocol.selectOption("dashscope.image.v1");
  await drawer.getByLabel("1:1", { exact: true }).check();
  await drawer.getByLabel("1:1 输出尺寸", { exact: true }).fill("1024*1024");
  await drawer.getByLabel("最多参考图片数", { exact: true }).fill("0");
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
  await expect(row.getByLabel("文生图", { exact: true })).toBeChecked();
  await expect(row.getByLabel("文本生成", { exact: true })).not.toBeChecked();
  await expect(row.getByLabel("启用", { exact: true })).not.toBeChecked();
  await row.getByLabel("启用", { exact: true }).check();
  await expect(row).toContainText("配置就绪");
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(capability).toHaveValue("image");
  await expect(protocol).toHaveValue("dashscope.image.v1");
});
