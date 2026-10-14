import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("custom ratio options persist and adaptive reaches the generation snapshot", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(60000);
  const connectionID = randomUUID();
  expect(
    (
      await request.post("/api/connections", {
        data: {
          id: connectionID,
          name: `比例验收 ${connectionID}`,
          vendor: "custom",
          base_url: "https://example.invalid",
        },
      })
    ).ok(),
  ).toBeTruthy();
  const model = await (
    await request.post(`/api/connections/${connectionID}/models`, {
      data: {
        model_id: "unfamiliar-video",
        name: "自定义比例验收",
        capabilities: ["video"],
      },
    })
  ).json();
  const operation = {
    protocol: "custom.json.v1",
    source: "manual",
    input_modes: ["first_frame"],
    ratios: ["16:9"],
    durations: [5],
    resolutions: ["720P"],
    max_references: 1,
    max_image_bytes: 10485760,
    custom: { path: "/v1/videos", auth_mode: "connection" },
    contract: {
      version: 1,
      execution: "sync_json",
      request: {
        example: JSON.stringify({
          model: "demo",
          prompt: "move",
          seconds: 5,
          metadata: {
            ratio: "16:9",
            resolution: "720p",
            content: [
              {
                type: "image_url",
                role: "first_frame",
                image_url: { url: "https://example.invalid/frame.png" },
              },
            ],
          },
        }),
        bindings: {
          model: "/model",
          prompt: "/prompt",
          duration: "/seconds",
          ratio: "/metadata/ratio",
          first_frame: "/metadata/content/0/image_url/url",
        },
      },
      response: {
        example: '{"url":"https://example.invalid/result.mp4"}',
        bindings: { result: "/url" },
      },
    },
  };
  expect(
    (
      await request.put(
        `/api/models/${encodeURIComponent(model.id)}/operations`,
        {
          data: {
            expected_revision: model.revision,
            operations: { video: operation },
            capabilities: ["video"],
          },
        },
      )
    ).ok(),
  ).toBeTruthy();
  expect(
    (
      await request.patch(
        `/api/connections/${connectionID}/models/${encodeURIComponent(model.id)}`,
        { data: { enabled: true } },
      )
    ).ok(),
  ).toBeTruthy();
  await page.goto("/config");
  await page.getByRole("button", { name: `比例验收 ${connectionID}` }).click();
  const row = page.getByRole("row").filter({ hasText: "自定义比例验收" });
  await row.getByRole("button", { name: "协议与参数", exact: true }).click();
  const drawer = page.getByRole("dialog");
  const ratios = drawer.getByRole("group", {
    name: "支持的画面比例",
    exact: true,
  });
  await ratios.getByRole("button", { name: "移除16:9", exact: true }).click();
  const input = ratios.getByLabel("添加支持的画面比例", { exact: true });
  await input.fill("adaptive");
  await input.press("Enter");
  await input.fill("21:9");
  await input.press("Enter");
  await input.fill("adaptive");
  await input.press("Enter");
  await expect(ratios.getByRole("listitem")).toHaveCount(2);
  await ratios.getByRole("button", { name: "移除21:9", exact: true }).click();
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await page.reload();
  await page.getByRole("button", { name: `比例验收 ${connectionID}` }).click();
  await row.getByRole("button", { name: "协议与参数", exact: true }).click();
  await expect(ratios.getByRole("listitem").locator("span")).toHaveText([
    "自适应（adaptive）",
  ]);
  await page.setViewportSize({ width: 390, height: 844 });
  await ratios.scrollIntoViewIfNeeded();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("custom-ratio-config-mobile.png"),
  });
  await drawer.getByRole("button", { name: "取消", exact: true }).click();
  const configured = (
    await (await request.get("/api/models")).json()
  ).models.find((entry: { id: string }) => entry.id === model.id);
  expect(configured.operations.video.ratios).toEqual(["adaptive"]);
  expect(configured.operations.video.contract.request.bindings.ratio).toBe(
    "/metadata/ratio",
  );

  const project = await (
    await request.post("/api/projects", { data: { name: "比例验收项目" } })
  ).json();
  const draftID = randomUUID();
  expect(
    (
      await request.post(`/api/projects/${project.id}/drafts`, {
        data: {
          id: draftID,
          name: "比例验收",
          story: {
            body: "测试",
            scenes: [
              {
                id: "shot",
                order: 1,
                title: "比例验收镜头",
                visual_prompt: "缓慢推进",
                duration_seconds: 5,
              },
            ],
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto(`/studio/${project.id}/${draftID}/shots/shot`);
  const settings = page.getByRole("region", { name: "视频设置" });
  await settings.getByLabel("视频模型", { exact: true }).selectOption(model.id);
  await expect(settings.getByLabel("画面比例", { exact: true })).toHaveValue(
    "adaptive",
  );
  await expect(
    settings.getByLabel("画面比例", { exact: true }).locator("option"),
  ).toHaveText(["自适应（adaptive）"]);
  await page.getByRole("button", { name: "＋ 添加场景", exact: true }).click();
  await drawer
    .locator('input[type="file"]')
    .setInputFiles("public/home/cafe.png");
  await drawer
    .getByRole("button", { name: "使用这张图片", exact: true })
    .click();
  await drawer.getByRole("button", { name: "保存并引用", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await settings
    .getByLabel("首帧图片", { exact: true })
    .selectOption({ label: "cafe.png" });
  await expect(page.getByRole("status").first()).toHaveText("已保存");
  await page.reload();
  await expect(settings.getByLabel("画面比例", { exact: true })).toHaveValue(
    "adaptive",
  );
  await settings.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("adaptive-shot-mobile.png"),
  });
  const submitted = page.waitForResponse(
    (response) =>
      response.url().endsWith("/generation-runs") &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "生成视频", exact: true }).click();
  const response = await submitted;
  expect(response.status()).toBe(202);
  const run = await response.json();
  expect(run.plan[0].aspect_ratio).toBe("adaptive");
  expect(run.plan[0].model_config.operations.video.ratios).toEqual([
    "adaptive",
  ]);
  // No real credential was configured: the task fails locally without a paid call.
  await expect(page.locator(".generation-run-row")).toHaveCount(1);
  const latestDraft = await (
    await request.get(`/api/projects/${project.id}/drafts/${draftID}`)
  ).json();
  const denied = await request.post(
    `/api/projects/${project.id}/drafts/${draftID}/scenes/shot/generation-runs`,
    {
      data: {
        expected_version: latestDraft.version,
        idempotency_key: randomUUID(),
        model_id: model.id,
        video_model_id: model.id,
        path: "reference_video",
        video_input_mode: "first_frame",
        first_frame_material_id: run.plan[0].first_frame_material_id,
        aspect_ratio: "16:9",
        duration_seconds: 5,
        resolution: "720P",
      },
    },
  );
  expect(denied.status()).toBe(400);
  expect((await denied.json()).error.message).toContain("比例");
});
