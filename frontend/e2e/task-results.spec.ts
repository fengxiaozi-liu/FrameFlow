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

test("failed video task shows field error and can retry", async ({ page }) => {
  const now = "2026-09-23T00:00:00Z";
  let status = "failed";
  let retried = false;
  await page.route("**/api/tasks?limit=100", (route) =>
    route.fulfill({ json: { tasks: [{
      id: "failed-video", kind: "video", scene_id: "scene-1", status,
      progress: status === "failed" ? 0 : 1, stage: status,
      error: status === "failed" ? "scene-1 / first_frame: invalid image" : "",
      input: { prompt: "scene" }, retry_count: retried ? 1 : 0,
      created_at: now, updated_at: now,
    }], total: 1 } }),
  );
  await page.route("**/api/tasks/failed-video/retry", (route) => {
    retried = true;
    status = "queued";
    return route.fulfill({ status: 202, json: { id: "failed-video", kind: "video", status, progress: 1, stage: status, input: { prompt: "scene" }, retry_count: 1, created_at: now, updated_at: now } });
  });
  await page.goto("/tasks");
  const card = page.locator(".task-detail").filter({ hasText: "failed-video" });
  await expect(card).toContainText("scene-1 / first_frame: invalid image");
  await card.getByRole("button", { name: /重新执行/ }).click();
  await expect.poll(() => retried).toBeTruthy();
  await expect(card).toContainText("queued");
});
