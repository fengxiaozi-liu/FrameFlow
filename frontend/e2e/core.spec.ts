import { expect, test, type APIRequestContext } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

async function seedProviders(request: APIRequestContext) {
  const connection = await request.post("/api/connections", {
    data: {
      id: "e2e-bailian",
      name: "百炼测试",
      vendor: "bailian",
      base_url:
        "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
    },
  });
  expect(connection.ok()).toBeTruthy();
  for (const [id, cap] of [
    ["qwen-plus", "story"],
    ["qwen-image-2.0", "image"],
    ["wan2.7-i2v", "video"],
  ]) {
    const saved = await request.post("/api/connections/e2e-bailian/models", {
      data: { model_id: id, capabilities: [cap] },
    });
    expect([201, 409]).toContain(saved.status());
    const model = await saved.json();
    if (saved.status() === 201) {
      const enabled = await request.patch(
        `/api/connections/e2e-bailian/models/${encodeURIComponent(model.id)}`,
        { data: { default: true } },
      );
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

async function createDraft(request: APIRequestContext) {
  const projectResponse = await request.post("/api/projects", {
    data: { name: "E2E Project" },
  });
  expect(projectResponse.ok()).toBeTruthy();
  const project = await projectResponse.json();
  const draftId = "draft-" + Date.now();
  const draftResponse = await request.post(
    "/api/projects/" + project.id + "/drafts",
    { data: { id: draftId, name: "E2E Draft" } },
  );
  expect(draftResponse.ok()).toBeTruthy();
  return { projectId: project.id as string, draftId };
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
  await expect(
    page.locator(".catalog-table tbody tr").filter({ hasText: "qwen-plus" }),
  ).toBeVisible();
  await page.goto("/studio");
  await page.getByLabel("故事 AI 接口").selectOption("e2e-bailian:qwen-plus");
  await page
    .getByLabel("生图 AI 接口")
    .selectOption("e2e-bailian:qwen-image-2.0");
  await page
    .getByLabel("视频生成 AI 接口")
    .selectOption("e2e-bailian:wan2.7-i2v");
  await expect(page.getByLabel("故事 AI 接口")).toHaveValue(
    "e2e-bailian:qwen-plus",
  );
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 844 });
    for (const path of ["/", "/studio", "/tasks", "/materials", "/config"]) {
      await page.goto(path);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBeTruthy();
    }
  }
});

test("configuration center adds a model without exposing protocol selection", async ({
  page,
}) => {
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接" }).click();
  await page.getByLabel("名称").fill("新建百炼连接");
  await page
    .getByLabel("API 完整地址")
    .fill("https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(
    page.getByRole("heading", { name: "新建百炼连接" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "手动添加" }).click();
  await page.getByLabel("模型 ID").fill("qwen-image-2.0");
  await page
    .getByRole("group", { name: "支持的能力" })
    .getByLabel("文生图")
    .check();
  await page.getByRole("button", { name: "添加", exact: true }).click();
  await expect(
    page
      .locator(".catalog-table tbody tr")
      .filter({ hasText: "qwen-image-2.0" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "手动添加" }).click();
  await page.getByLabel("模型 ID").fill("unknown-model");
  await page.getByRole("button", { name: "添加", exact: true }).click();
  await expect(
    page.locator(".catalog-table").getByText("待配置能力"),
  ).toBeVisible();
  const unknown = page
    .locator(".catalog-table tbody tr")
    .filter({ hasText: "unknown-model" });
  await unknown.getByLabel("文本生成").check();
  await expect(unknown.getByLabel("启用")).toBeVisible();
  await expect(page.getByText("调用适配器")).toHaveCount(0);
});

test("Bailian connection saves a complete HTTPS endpoint", async ({
  page,
}, testInfo) => {
  await page.goto("/config");
  await page.getByRole("button", { name: "添加厂商连接" }).click();
  await page.getByLabel("名称").fill("自定义百炼连接");
  await page
    .getByLabel("API 完整地址")
    .fill("https://custom.example.com/?token=unsafe");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(page.locator(".notice.error")).toContainText(
    "API 地址必须使用 HTTPS",
  );
  await page.screenshot({
    path: testInfo.outputPath("connection-error.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBeTruthy();
  await page.screenshot({
    path: testInfo.outputPath("connection-error-mobile.png"),
    fullPage: true,
  });
  await page
    .getByLabel("API 完整地址")
    .fill("https://custom.example.com/compatible-mode/v1/chat/completions");
  await page.getByRole("button", { name: "保存连接" }).click();
  await expect(
    page.getByRole("heading", { name: "自定义百炼连接" }),
  ).toBeVisible();
  await expect(page.getByLabel("API 完整地址")).toHaveValue(
    "https://custom.example.com/compatible-mode/v1/chat/completions",
  );
});

test("empty material library opens the picker and uploads a local file", async ({
  page,
  request,
}) => {
  await page.goto("/materials");
  await page.getByRole("button", { name: "角色", exact: true }).click();
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
  const material = page
    .locator(".material-card")
    .filter({ hasText: "character.png" });
  await expect(material).toBeVisible();
  await expect(material).toHaveClass(/selected/);

  const saved = await request.get("/api/materials?kind=character");
  const uploaded = (await saved.json()).materials.find(
    (item: { name: string }) => item.name === "character.png",
  );
  expect(uploaded?.url).toMatch(/^\/media\//);
  expect((await request.get(uploaded.url)).ok()).toBeTruthy();
  page.once("dialog", (dialog) => dialog.accept());
  await material.getByRole("button", { name: "删除 character.png" }).click();
  await expect(page.getByText("素材已删除。")).toBeVisible();
  await expect(material).toHaveCount(0);
});

test("failed task can be retried from task center", async ({
  page,
  request,
}) => {
  const scope = await createDraft(request);
  const created = await request.post("/api/tasks", {
    data: {
      project_id: scope.projectId,
      draft_id: scope.draftId,
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
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => socket.close());
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
  for (const path of ["/", "/studio", "/tasks", "/materials", "/config"]) {
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

test("completed story task fills the scoped planning body", async ({
  page,
}) => {
  const now = "2026-09-22T00:00:00Z";
  let savedDraft: Record<string, unknown> | undefined;
  let taskCreated = false;
  await page.route("**/api/projects", async (route) => {
    if (route.request().method() === "POST") {
      await route.fulfill({
        status: 201,
        json: {
          id: "project-story",
          name: "Story",
          drafts: [],
          created_at: now,
        },
      });
    } else await route.continue();
  });
  await page.route("**/api/projects/project-story/drafts", async (route) => {
    savedDraft = route.request().postDataJSON();
    await route.fulfill({
      json: {
        id: "project-story",
        name: "Story",
        created_at: now,
        drafts: [savedDraft],
      },
    });
  });
  await page.route("**/api/projects/project-story", async (route) => {
    const generated = structuredClone(savedDraft) as Record<string, any>;
    generated.story.body = "生成后的策划正文";
    generated.outputs = [
      {
        task_id: "story-task",
        kind: "story",
        text: "生成后的策划正文",
        created_at: now,
      },
    ];
    await route.fulfill({
      json: {
        id: "project-story",
        name: "Story",
        created_at: now,
        drafts: [generated],
      },
    });
  });
  await page.route("**/api/tasks**", async (route) => {
    if (route.request().method() !== "POST") {
      const items = taskCreated
        ? [
            {
              id: "story-task",
              project_id: "project-story",
              draft_id: (savedDraft as any).id,
              kind: "story",
              status: "succeeded",
              progress: 100,
              stage: "completed",
              input: { prompt: "一个故事" },
              result_text: "生成后的策划正文",
              created_at: now,
              updated_at: now,
            },
          ]
        : [];
      return route.fulfill({ json: { tasks: items, total: items.length } });
    }
    const body = route.request().postDataJSON();
    taskCreated = true;
    await route.fulfill({
      status: 202,
      json: {
        id: "story-task",
        project_id: body.project_id,
        draft_id: body.draft_id,
        kind: "story",
        status: "queued",
        progress: 0,
        stage: "queued",
        input: { prompt: body.prompt },
        created_at: now,
        updated_at: now,
      },
    });
  });
  await page.goto("/studio");
  await page.getByLabel("故事 AI 接口").selectOption({ index: 1 });
  await page.getByLabel("视频想法 / 修改要求").fill("一个故事");
  await page.getByRole("button", { name: "生成故事与分镜" }).click();
  await expect(page.getByLabel("策划正文")).toHaveValue("生成后的策划正文");
  await expect(page.getByText("生成结果已填充到策划正文。")).toBeVisible();
});
