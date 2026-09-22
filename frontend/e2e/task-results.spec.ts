import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

test("completed tasks preview media and download the real image", async ({
  page,
}, testInfo) => {
  const timestamp = "2026-09-17T00:00:00Z";
  const tasks = [
    {
      id: "image-1",
      kind: "image",
      status: "succeeded",
      progress: 100,
      stage: "completed",
      input: { prompt: "test" },
      result_url: "/media/generated-image-1.png",
      created_at: timestamp,
      updated_at: timestamp,
    },
    {
      id: "video-1",
      kind: "video",
      status: "succeeded",
      progress: 100,
      stage: "completed",
      input: {
        prompt: "test",
        source_image_url: "/media/generated-image-1.png",
      },
      result_url: "/media/generated-video-1.mp4",
      created_at: timestamp,
      updated_at: timestamp,
    },
    {
      id: "legacy-1",
      kind: "image",
      status: "succeeded",
      progress: 100,
      stage: "completed",
      input: { prompt: "test" },
      created_at: timestamp,
      updated_at: timestamp,
    },
  ];
  const png = readFileSync(join(process.cwd(), "public/favicon.png"));
  await page.route("**/api/tasks?limit=100", (route) =>
    route.fulfill({ json: { tasks, total: tasks.length } }),
  );
  await page.route("**/media/generated-image-1.png", (route) =>
    route.fulfill({ contentType: "image/png", body: png }),
  );
  await page.route("**/api/tasks/image-1/download", (route) =>
    route.fulfill({
      contentType: "image/png",
      headers: {
        "Content-Disposition": 'attachment; filename="frameflow-image-1.png"',
      },
      body: png,
    }),
  );

  await page.goto("/tasks");
  await expect(page.locator(".image-preview img")).toBeVisible();
  await expect
    .poll(() =>
      page
        .locator(".image-preview img")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await expect(page.locator(".video-preview video[controls]")).toBeVisible();
  await expect(page.locator(".video-preview video")).toHaveAttribute(
    "poster",
    /generated-image-1.png/,
  );
  const legacy = page.locator(".task-detail").filter({ hasText: "legacy-1" });
  await expect(
    legacy.getByText("此任务没有保存可预览的媒体文件。"),
  ).toBeVisible();
  await expect(legacy.getByRole("button", { name: "下载图片" })).toHaveCount(0);

  const download = page.waitForEvent("download");
  await page
    .locator(".task-detail")
    .filter({ hasText: "image-1" })
    .getByRole("button", { name: "下载图片" })
    .click();
  expect((await download).suggestedFilename()).toBe("frameflow-image-1.png");

  await page.screenshot({
    path: testInfo.outputPath("task-results-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("task-results-mobile.png"),
    fullPage: true,
  });
});

test("studio submits a generated image as the video source", async ({
  page,
  request,
}) => {
  const connection = await request.post("/api/connections", {
    data: {
      id: "video-source",
      name: "Video source",
      vendor: "bailian",
      base_url:
        "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
    },
  });
  expect(connection.ok()).toBeTruthy();
  const model = await request.post("/api/connections/video-source/models", {
    data: { model_id: "wan2.7-i2v", capabilities: ["video"] },
  });
  expect(model.ok()).toBeTruthy();
  const draft = {
    id: "draft-video",
    project_id: "project-video",
    name: "Video",
    story: {
      summary: "",
      body: "A red circle",
      scenes: [
        {
          id: "scene",
          order: 1,
          title: "Scene",
          visual_prompt: "",
          narration: "",
          duration_seconds: 10,
        },
      ],
      updated_at: "2026-09-17T00:00:00Z",
    },
    outputs: [],
  };
  const project = {
    id: "project-video",
    name: "Video",
    drafts: [draft],
    created_at: "2026-09-17T00:00:00Z",
  };
  const image = {
    id: "generated-1",
    project_id: project.id,
    draft_id: draft.id,
    kind: "image",
    status: "succeeded",
    progress: 100,
    stage: "completed",
    input: { prompt: "A red circle" },
    result_url: "/media/generated-generated-1.png",
    created_at: "2026-09-17T00:00:00Z",
    updated_at: "2026-09-17T00:00:00Z",
  };
  await page.route("**/api/tasks?limit=100", (route) =>
    route.fulfill({ json: { tasks: [image], total: 1 } }),
  );
  await page.route("**/api/projects/project-video", (route) =>
    route.fulfill({ json: project }),
  );
  await page.route("**/api/projects/project-video/drafts", (route) =>
    route.fulfill({ json: project }),
  );
  let submittedSource = "";
  await page.route("**/api/tasks", (route) => {
    if (route.request().method() !== "POST") return route.continue();
    submittedSource = route.request().postDataJSON().source_image_url;
    return route.fulfill({
      status: 202,
      json: {
        id: "video-new",
        kind: "video",
        status: "queued",
        progress: 0,
        stage: "queued",
        input: { prompt: "A red circle" },
      },
    });
  });
  await page.goto("/studio/project-video/draft-video");
  await page.getByLabel("已生成的首帧图片").selectOption(image.result_url);
  await page
    .getByLabel("视频生成 AI 接口")
    .selectOption("video-source:wan2.7-i2v");
  await page.getByRole("button", { name: "检查并生成视频" }).click();
  await expect.poll(() => submittedSource).toBe(image.result_url);
});
