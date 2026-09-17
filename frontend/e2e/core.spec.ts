import { expect, test, type APIRequestContext } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function seedProviders(request: APIRequestContext) {
  const connection = await request.post("/api/connections", { data: { id: "e2e-bailian", name: "百炼测试", vendor: "bailian", base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions" } });
  expect(connection.ok()).toBeTruthy();
  for (const [id, cap] of [["qwen-plus", "story"], ["qwen-image-2.0", "image"], ["wan2.7-i2v", "video"]]) {
    const saved = await request.post("/api/connections/e2e-bailian/models", { data: { model_id: id, capabilities: [cap] } });
    expect([201, 409]).toContain(saved.status());
    const model = await saved.json();
    if (saved.status() === 201) {
      const enabled = await request.patch(`/api/connections/e2e-bailian/models/${encodeURIComponent(model.id)}`, { data: { default: true } });
      expect(enabled.ok()).toBeTruthy();
    }
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
  await page.getByLabel("首帧图片 URL").fill("https://example.com/first.png");
  await page.getByRole("button", { name: "检查并生成视频" }).click();
  await expect(
    page.getByText("任务已进入后台队列，可以继续编辑。"),
  ).toBeVisible();
  await expect.poll(() => upgraded).toBeTruthy();
  await page.getByRole("link", { name: /后台任务/ }).click();
  await expect(page.getByRole("heading", { name: "任务中心" })).toBeVisible();
  await expect(page.locator(".task-detail").first()).toContainText("失败", {
    timeout: 5000,
  });
});

test("provider switching and responsive layout", async ({ page }) => {
  await page.goto("/config");
  await expect(page.getByRole("heading", { name: "配置中心" })).toBeVisible();
  await page.getByRole("button", { name: /百炼测试/ }).click();
  await expect(page.locator(".catalog-table tbody tr").filter({ hasText: "qwen-plus" })).toBeVisible();
  await page.goto("/studio");
  await page.getByLabel("故事 AI 接口").selectOption("e2e-bailian:qwen-plus");
  await page.getByLabel("生图 AI 接口").selectOption("e2e-bailian:qwen-image-2.0");
  await page.getByLabel("视频生成 AI 接口").selectOption("e2e-bailian:wan2.7-i2v");
  await expect(page.getByLabel("故事 AI 接口")).toHaveValue("e2e-bailian:qwen-plus");
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

test("configuration center adds a model without exposing protocol selection", async ({ page }) => {
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接" }).click();
  await page.getByLabel("名称").fill("新建百炼连接");
  await page.getByLabel("API 完整地址").fill("https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(page.getByRole("heading", { name: "新建百炼连接" })).toBeVisible();
  await page.getByRole("button", { name: "手动添加" }).click();
  await page.getByLabel("模型 ID").fill("qwen-image-2.0");
  await page.getByRole("group", { name: "支持的能力" }).getByLabel("文生图").check();
  await page.getByRole("button", { name: "添加", exact: true }).click();
  await expect(page.locator(".catalog-table tbody tr").filter({ hasText: "qwen-image-2.0" })).toBeVisible();
  await page.getByRole("button", { name: "手动添加" }).click();
  await page.getByLabel("模型 ID").fill("unknown-model");
  await page.getByRole("button", { name: "添加", exact: true }).click();
  await expect(page.locator(".catalog-table").getByText("待配置能力")).toBeVisible();
  const unknown = page.locator(".catalog-table tbody tr").filter({ hasText: "unknown-model" });
  await unknown.getByLabel("文本生成").check();
  await expect(unknown.getByLabel("启用")).toBeVisible();
  await expect(page.getByText("调用适配器")).toHaveCount(0);
});

test("Bailian connection saves a complete HTTPS endpoint", async ({ page }, testInfo) => {
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接" }).click();
  await page.getByLabel("名称").fill("自定义百炼连接");
  await page.getByLabel("API 完整地址").fill("https://custom.example.com/?token=unsafe");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(page.locator(".notice.error")).toContainText("API 地址必须使用 HTTPS");
  await page.screenshot({ path: testInfo.outputPath("connection-error.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: testInfo.outputPath("connection-error-mobile.png"), fullPage: true });
  await page.getByLabel("API 完整地址").fill("https://custom.example.com/compatible-mode/v1/chat/completions");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(page.getByRole("heading", { name: "自定义百炼连接" })).toBeVisible();
  await expect(page.getByLabel("API 完整地址")).toHaveValue("https://custom.example.com/compatible-mode/v1/chat/completions");
});

test("empty material library opens the picker and uploads a local file", async ({
  page,
  request,
}) => {
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
      model_id: "e2e-bailian:wan2.7-i2v",
      prompt: "测试失败任务",
      source_image_url: "https://example.com/first.png",
    },
  });
  const task = await created.json();
  await expect.poll(() => taskField(request, task.id, "status")).toBe("failed");
  const previousRetries = await taskField(request, task.id, "retry_count");
  await page.goto("/tasks");
  const card = page.locator(".task-detail").filter({ hasText: task.id });
  await card.getByRole("button", { name: "重新执行" }).click();
  await expect
    .poll(() => taskField(request, task.id, "retry_count"))
    .toBeGreaterThan(previousRetries);
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
  await page.getByLabel("首帧图片 URL").fill("https://example.com/first.png");
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
