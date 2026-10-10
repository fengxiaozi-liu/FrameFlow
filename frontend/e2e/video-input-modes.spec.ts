import { test, expect } from "@playwright/test";
import { randomUUID } from "node:crypto";

test("dual-mode configuration, explicit frame roles, switching and frozen submission", async ({
  page,
  request,
}, testInfo) => {
  const id = randomUUID();
  expect(
    (
      await request.post("/api/connections", {
        data: {
          id,
          name: id,
          vendor: "custom",
          base_url: "https://example.invalid",
        },
      })
    ).ok(),
  ).toBeTruthy();
  expect(
    (
      await request.post(`/api/connections/${id}/models`, {
        data: {
          model_id: "dual-video",
          name: "双模式视频",
          capabilities: ["video"],
        },
      })
    ).ok(),
  ).toBeTruthy();
  const model = (await (await request.get("/api/models")).json()).models.find(
    (m: { id: string }) => m.id === `${id}:dual-video`,
  );
  expect(
    (
      await request.put(`/api/models/${model.id}/operations`, {
        data: {
          expected_revision: model.revision,
          capabilities: ["video"],
          operations: {
            video: {
              protocol: "dashscope.media-video.v1",
              source: "manual",
              input_modes: ["first_frame"],
              last_frame: true,
              ratios: ["16:9"],
              durations: [4, 5, 6],
              resolutions: ["720P"],
              max_references: 1,
              max_image_bytes: 10485760,
            },
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  await page.goto("/config");
  await page.getByRole("button", { name: id, exact: false }).click();
  const row = page.getByRole("row").filter({ hasText: "双模式视频" });
  await row.getByLabel("启用", { exact: true }).check();
  await row.getByRole("button", { name: "协议与参数" }).click();
  const drawer = page.getByRole("dialog");
  await expect(drawer.getByLabel("首尾帧模式", { exact: true })).toBeChecked();
  await drawer.getByLabel("智能参考（多图）", { exact: true }).check();
  await drawer.getByLabel("最多参考图片数", { exact: true }).fill("3");
  await drawer.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(drawer).toHaveCount(0);
  await row.getByRole("button", { name: "协议与参数" }).click();
  await expect(drawer.getByLabel("首尾帧模式", { exact: true })).toBeChecked();
  await expect(
    drawer.getByLabel("智能参考（多图）", { exact: true }),
  ).toBeChecked();
  await drawer
    .getByRole("group", { name: "支持的生成模式（可多选）", exact: true })
    .screenshot({ path: testInfo.outputPath("model-modes.png") });
  await drawer.getByRole("button", { name: "取消", exact: true }).click();
  const project = await (
    await request.post("/api/projects", { data: { name: "双模式验收" } })
  ).json();
  const images: { id: string; name: string }[] = [];
  for (const name of ["起始画面.png", "结束画面.png", "人物参考.png"]) {
    const result = await request.post("/api/materials/upload", {
      multipart: {
        kind: "scene",
        name,
        project_id: project.id,
        reference_only: "true",
        file: {
          name,
          mimeType: "image/png",
          buffer: Buffer.from(
            "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=",
            "base64",
          ),
        },
      },
    });
    expect(result.ok()).toBeTruthy();
    images.push(await result.json());
  }
  expect(
    (
      await request.post(`/api/projects/${project.id}/entities`, {
        data: {
          expected_catalog_version: 0,
          entity: {
            id: "location",
            kind: "location",
            name: "场景",
            description: "日出",
            revision: 1,
            references: images.map((image, order) => ({
              material_id: image.id,
              order,
              label: image.name,
            })),
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  const draft = randomUUID();
  expect(
    (
      await request.post(`/api/projects/${project.id}/drafts`, {
        data: {
          id: draft,
          name: "双模式",
          story: {
            body: "日出",
            scenes: [
              {
                id: "shot",
                title: "日出",
                order: 1,
                visual_prompt: "从夜晚过渡到清晨",
                duration_seconds: 4,
                entity_bindings: [
                  {
                    id: "binding",
                    entity_id: "location",
                    entity_revision: 1,
                    role: "primary",
                  },
                ],
              },
            ],
          },
        },
      })
    ).ok(),
  ).toBeTruthy();
  const url = `/studio/${project.id}/${draft}/shots/shot`;
  await page.goto(url);
  const settings = page.getByRole("region", { name: "视频设置" });
  await settings.getByLabel("视频模型", { exact: true }).selectOption(model.id);
  await expect(
    page.getByRole("button", { name: "生成视频", exact: true }),
  ).toBeDisabled();
  await settings
    .getByRole("button", { name: "首尾帧模式", exact: false })
    .click();
  // Deliberately reverse the list order: roles must come from explicit selection.
  await settings
    .getByLabel("首帧图片", { exact: true })
    .selectOption(images[1]!.id);
  await settings
    .getByLabel("尾帧图片", { exact: true })
    .selectOption(images[0]!.id);
  await settings
    .getByRole("button", { name: "智能参考模式", exact: false })
    .click();
  await expect(settings.getByLabel("首帧图片", { exact: true })).toHaveCount(0);
  await settings
    .getByRole("button", { name: "首尾帧模式", exact: false })
    .click();
  await expect(settings.getByLabel("首帧图片", { exact: true })).toHaveValue(
    images[1]!.id,
  );
  await expect(page.getByRole("status").first()).toHaveText("已保存");
  await page.reload();
  await expect(settings.getByLabel("尾帧图片", { exact: true })).toHaveValue(
    images[0]!.id,
  );
  await settings
    .locator(".video-input-picker")
    .screenshot({ path: testInfo.outputPath("frame-picker-desktop.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await settings
    .locator(".video-input-picker")
    .screenshot({ path: testInfo.outputPath("frame-picker-mobile.png") });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole("button", { name: "生成视频", exact: true }).click();
  await expect(page).toHaveURL(url);
  await expect(page.locator(".generation-run-row")).not.toHaveCount(0);
  await page.getByRole("link", { name: "进入任务 →" }).last().click();
  await expect(page).toHaveURL(/\/runs\/run-/);
  await expect(
    page.getByRole("img", { name: "提交时的首帧", exact: true }),
  ).toBeVisible();
  let runs = (
    await (
      await request.get(
        `/api/projects/${project.id}/drafts/${draft}/scenes/shot/generation-runs`,
      )
    ).json()
  ).runs;
  expect(runs[0].plan[0].video_input_mode).toBe("first_frame");
  expect(runs[0].plan[0].first_frame_material_id).toBe(images[1]!.id);
  expect(runs[0].plan[0].last_frame_material_id).toBe(images[0]!.id);
  expect(runs[0].plan[0].entities[0].references).toHaveLength(2);
  await page.getByRole("button", { name: "取消制作任务" }).click();
  await page.goto(url);
  await settings
    .getByRole("button", { name: "智能参考模式", exact: false })
    .click();
  await expect(page.getByRole("status").first()).toHaveText("已保存");
  await page.getByRole("button", { name: "生成视频", exact: true }).click();
  await expect(page).toHaveURL(url);
  await expect(page.locator(".generation-run-row")).not.toHaveCount(0);
  await page.getByRole("link", { name: "进入任务 →" }).last().click();
  await expect(page).toHaveURL(/\/runs\/run-/);
  runs = (
    await (
      await request.get(
        `/api/projects/${project.id}/drafts/${draft}/scenes/shot/generation-runs`,
      )
    ).json()
  ).runs;
  const reference = runs.find(
    (run: { plan: { video_input_mode: string }[] }) =>
      run.plan[0]?.video_input_mode === "reference_images",
  ).plan[0];
  expect(reference.first_frame_material_id).toBeUndefined();
  expect(reference.last_frame_material_id).toBeUndefined();
  expect(reference.entities[0].references).toHaveLength(3);
});
