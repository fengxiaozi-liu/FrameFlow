import { expect, test, type APIRequestContext } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const providers = [
  {
    code: "story-test",
    name: "故事测试接口",
    capability: "story",
    model: "mock-story",
    base_url: "mock://local",
    enabled: true,
    status: "healthy",
  },
  {
    code: "image-test",
    name: "生图测试接口",
    capability: "image",
    model: "mock-image",
    base_url: "mock://local",
    enabled: true,
    status: "healthy",
  },
  {
    code: "video-test",
    name: "视频测试接口",
    capability: "video",
    model: "mock-video",
    base_url: "mock://local",
    enabled: true,
    status: "healthy",
  },
  {
    code: "video-fail",
    name: "失败测试接口",
    capability: "video",
    model: "mock-fail-video",
    base_url: "mock://local",
    enabled: true,
    status: "healthy",
  },
];

async function seedProviders(request: APIRequestContext) {
  for (const item of providers) {
    const response = await request.post("/api/providers", { data: item });
    expect(response.ok()).toBeTruthy();
  }
}

async function taskField(
  request: APIRequestContext,
  id: string,
  field: string,
) {
  const response = await request.get(`/api/tasks/${id}`);
  if (!response.ok()) return undefined;
  return (await response.json())[field];
}

test.beforeEach(async ({ request }) => seedProviders(request));

test("home, studio and WebSocket asynchronous task flow", async ({ page }) => {
  let upgraded = false;
  page.on("websocket", (socket) => {
    if (socket.url().includes("/ws")) upgraded = true;
  });
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "开始下一段视频创作" }),
  ).toBeVisible();
  await page.getByPlaceholder(/例如：用 30 秒/).fill("记录城市的一天");
  await page.getByRole("button", { name: "开始创作" }).click();
  await expect(page).toHaveURL(/studio/);
  await page.getByRole("button", { name: "新增分镜" }).click();
  await page.getByRole("button", { name: "检查并生成视频" }).click();
  await expect(
    page.getByText("任务已进入后台队列，可以继续编辑。"),
  ).toBeVisible();
  await expect.poll(() => upgraded).toBeTruthy();
  await page.getByRole("link", { name: /后台任务/ }).click();
  await expect(page.getByRole("heading", { name: "任务中心" })).toBeVisible();
  await expect(page.locator(".task-detail").first()).toContainText("已完成", {
    timeout: 5000,
  });
});

test("provider switching and responsive layout", async ({ page }) => {
  await page.goto("/config");
  await expect(page.getByRole("heading", { name: "配置中心" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "故事测试接口" }),
  ).toBeVisible();
  await page.goto("/studio");
  await page.getByLabel("故事 AI 接口").selectOption("story-test");
  await page.getByLabel("生图 AI 接口").selectOption("image-test");
  await page.getByLabel("视频生成 AI 接口").selectOption("video-test");
  await expect(page.getByLabel("故事 AI 接口")).toHaveValue("story-test");
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 844 });
    for (const path of ["/", "/studio", "/tasks", "/config"]) {
      await page.goto(path);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBeTruthy();
    }
  }
});

test("empty material library opens the picker and uploads a local file", async ({
  page,
  request,
}) => {
  const existing = await request.get("/api/materials?kind=character");
  for (const item of (await existing.json()).materials) {
    await request.delete(`/api/materials/${item.id}`);
  }

  await page.goto("/studio");
  const fileChooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: /当前素材库为空/ }).click();
  await (
    await fileChooser
  ).setFiles({
    name: "character.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAEElEQVR42mNk+M/wHwAF/gL+3MxZ5wAAAABJRU5ErkJggg==",
      "base64",
    ),
  });

  await expect(
    page.getByText("素材“character.png”已上传并选中。"),
  ).toBeVisible();
  const material = page.getByRole("button", { name: /character.png/ });
  await expect(material).toBeVisible();
  await expect(material).toHaveClass(/selected/);

  const saved = await request.get("/api/materials?kind=character");
  const uploaded = (await saved.json()).materials.find(
    (item: { name: string }) => item.name === "character.png",
  );
  expect(uploaded?.url).toMatch(/^\/media\//);
  expect((await request.get(uploaded.url)).ok()).toBeTruthy();
});

test("failed task can be retried from task center", async ({
  page,
  request,
}) => {
  const created = await request.post("/api/tasks", {
    data: {
      kind: "video",
      provider_code: "video-fail",
      prompt: "测试失败任务",
    },
  });
  const task = await created.json();
  await expect.poll(() => taskField(request, task.id, "status")).toBe("failed");
  await page.goto("/tasks");
  const card = page.locator(".task-detail").filter({ hasText: task.id });
  await card.getByRole("button", { name: "重新执行" }).click();
  await expect
    .poll(() => taskField(request, task.id, "retry_count"))
    .toBeGreaterThan(0);
  await expect.poll(() => taskField(request, task.id, "status")).toBe("failed");
});

test("WebSocket disconnect falls back to HTTP task refresh", async ({
  page,
}) => {
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) =>
    socket.close(),
  );
  const fallback = page.waitForRequest((request) =>
    request.url().includes("/api/tasks?limit=100"),
  );
  await page.goto("/studio");
  await page.getByRole("button", { name: "新增分镜" }).click();
  await page.getByRole("button", { name: "检查并生成视频" }).click();
  await fallback;
});

test("core pages have no serious accessibility violations", async ({
  page,
}) => {
  for (const path of ["/", "/studio", "/tasks", "/config"]) {
    await page.goto(path);
    const result = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    const severe = result.violations.filter((item) =>
      ["serious", "critical"].includes(item.impact || ""),
    );
    expect(
      severe,
      `${path}: ${severe.map((item) => item.id).join(", ")}`,
    ).toEqual([]);
  }
});
