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

test("storyboard tab renders an existing draft with an empty history snapshot", async ({ page, request }, testInfo) => {
  const { projectId, draftId } = await createDraft(request);
  await page.route(`**/api/projects/${projectId}`, async (route) => {
    const response = await route.fetch();
    const project = await response.json();
    const draft = project.drafts.find((item: { id: string }) => item.id === draftId);
    draft.story.body = "A short story";
    draft.story.scenes = Array.from({ length: 6 }, (_, index) => ({
      id: `scene-${index + 1}`,
      order: index + 1,
      title: `Scene ${index + 1}`,
      visual_prompt: "A quiet street",
      narration: "",
      duration_seconds: 10,
    }));
    draft.storyboard_snapshots = [{ id: "before-first-storyboard", scenes: null }];
    await route.fulfill({ response, json: project });
  });
  const pageErrors: Error[] = [];
  page.on("pageerror", (error) => pageErrors.push(error));
  await page.goto(`/studio/${projectId}/${draftId}`);
  await page.getByRole("button", { name: /分镜 6/ }).click();
  await expect(page.locator(".scene-list .scene-row")).toHaveCount(6);
  await expect(page.locator(".history-strip")).toContainText("0 个镜头 · 恢复");
  await expect(page.locator(".scene-board")).toBeVisible();
  await expect(page.locator(".scene-inspector")).toBeVisible();
  const board = await page.locator(".scene-board").boundingBox();
  const inspector = await page.locator(".scene-inspector").boundingBox();
  expect(board!.x + board!.width).toBeLessThan(inspector!.x);
  await page.screenshot({ path: testInfo.outputPath("storyboard-desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".scene-inspector")).toBeVisible();
  await expect(page.locator(".scene-board")).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: testInfo.outputPath("storyboard-mobile.png"), fullPage: true });
  expect(pageErrors).toEqual([]);
});

test("a storyboard card can delete its own scene after the settings panel is closed", async ({ page }) => {
  await page.goto("/studio");
  await page.getByRole("button", { name: /分镜/ }).first().click();
  await page.getByRole("button", { name: "新增分镜" }).click();
  await page.getByRole("button", { name: "新增分镜" }).click();
  await page.getByRole("button", { name: "关闭镜头设置" }).click();
  const cards = page.locator(".scene-list .scene-row-wrap");
  await expect(cards).toHaveCount(2);
  const deleteButton = cards.nth(1).getByRole("button", { name: /删除镜头/ });
  page.once("dialog", (dialog) => dialog.dismiss());
  await deleteButton.click();
  await expect(cards).toHaveCount(2);
  page.once("dialog", (dialog) => {
    expect(dialog.message()).toContain("镜头 2");
    return dialog.accept();
  });
  await deleteButton.click();
  await expect(cards).toHaveCount(1);
  await expect(cards.first().getByRole("button", { name: /删除镜头/ })).toHaveAttribute("aria-label", /镜头 1/);
  await page.reload();
  await page.getByRole("button", { name: /分镜/ }).first().click();
  await expect(cards).toHaveCount(1);
});

test("a completed storyboard task reveals its candidate and applying it opens the scene list", async ({ page, request }) => {
  const { projectId, draftId } = await createDraft(request);
  const draftPath = `/api/projects/${projectId}/drafts/${draftId}`;
  const draftResponse = await request.get(draftPath);
  expect(draftResponse.ok()).toBeTruthy();
  const draft = await draftResponse.json();
  expect(draft.id).toBe(draftId);
  draft.story.body = "清晨的街道上，艾琳走进书店。";
  draft.story.scenes = [];
  const taskId = "storyboard-navigation-task";
  const scene = { id: "generated-scene", order: 1, title: "走进书店", visual_prompt: "清晨的书店", narration: "", duration_seconds: 5 };
  const candidate = { id: taskId, task_id: taskId, target: "storyboard", source_version: draft.version || 0, source_body: draft.story.body, scenes: [scene], status: "preview" };
  let candidateReady = false;
  await page.route(`**/api/projects/${projectId}`, async (route) => {
    const response = await route.fetch();
    const project = await response.json();
    project.drafts = project.drafts.map((item: { id: string }) => item.id === draftId ? { ...draft, candidates: candidateReady ? [candidate] : [] } : item);
    await route.fulfill({ response, json: project });
  });
  await page.route(`**${draftPath}`, (route) => {
    if (route.request().method() !== "GET") return route.continue();
    return route.fulfill({ json: { ...draft, candidates: candidateReady ? [candidate] : [] } });
  });
  await page.route(`**${draftPath}/candidates/${taskId}/apply`, (route) => {
    candidateReady = false;
    draft.story.scenes = [scene];
    draft.candidates = [{ ...candidate, status: "applied" }];
    draft.version = (draft.version || 0) + 1;
    return route.fulfill({ json: draft });
  });
  await page.route("**/api/tasks?*", (route) => route.fulfill({ json: { tasks: [{ id: taskId, project_id: projectId, draft_id: draftId, kind: "story", status: "running", progress: 50, stage: "generating", updated_at: new Date(0).toISOString() }], total: 1 } }));
  let sendTaskEvent: ((payload: string) => void) | undefined;
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => { sendTaskEvent = (payload) => socket.send(payload); });
  await page.goto(`/studio/${projectId}/${draftId}`);
  await expect(page.getByLabel("故事正文")).toHaveValue("清晨的街道上，艾琳走进书店。");
  await expect(page.locator(".editor-card .candidate-card")).toHaveCount(0);
  await expect.poll(() => !!sendTaskEvent).toBe(true);
  candidateReady = true;
  sendTaskEvent!(JSON.stringify({ task_id: taskId, project_id: projectId, draft_id: draftId, status: "succeeded", progress: 100, stage: "completed", at: new Date().toISOString() }));
  const card = page.locator(".editor-card .candidate-card");
  await expect(card).toContainText("走进书店");
  await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(0);
  await expect.poll(async () => {
    const button = (await card.getByRole("button", { name: "应用分镜" }).boundingBox())!;
    return button.y + button.height <= page.viewportSize()!.height;
  }).toBe(true);
  await card.getByRole("button", { name: "应用分镜" }).click();
  await expect(page.locator(".workspace-tabs button.active")).toContainText("分镜");
  await expect(page.locator(".scene-list .scene-row")).toContainText("走进书店");
  await expect.poll(async () => (await page.locator(".scene-board").boundingBox())!.y).toBeLessThan(150);
});

test("story editor follows the wide card layout and stays usable on mobile", async ({ page }, testInfo) => {
  await page.goto("/studio");
  const card = page.locator(".editor-card");
  await expect(card).toBeVisible();
  const desktop = await card.boundingBox();
  expect(desktop?.width).toBeGreaterThan(1200);
  const projectBar = await page.locator(".studio-project-bar").boundingBox();
  const tabs = await page.locator(".workspace-tabs").boundingBox();
  expect(Math.abs(tabs!.x + tabs!.width / 2 - (projectBar!.x + projectBar!.width / 2))).toBeLessThan(2);
  await expect(page.locator(".workspace-tabs button").first()).toHaveCSS("font-size", "18px");
  await expect(page.locator(".body-editor")).toHaveCSS("border-top-width", "0px");
  await page.screenshot({ path: testInfo.outputPath("studio-desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await expect(card).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("studio-mobile.png"), fullPage: true });
});

test("story drawer restores keyboard focus and mobile material picker keeps a retry path", async ({ page }, testInfo) => {
  await page.goto("/studio");
  const opener = page.getByRole("button", { name: "故事联想" });
  await opener.focus();
  await opener.press("Enter");
  const dialog = page.getByRole("dialog", { name: "故事联想" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "关闭故事联想" })).toBeFocused();
  const resizeHandle = dialog.getByRole("separator", { name: "调整故事联想宽度" });
  const initialWidth = (await dialog.boundingBox())!.width;
  const grip = (await resizeHandle.boundingBox())!;
  const gripX = grip.x + grip.width / 2;
  const gripY = grip.y + grip.height / 2;
  await page.mouse.move(gripX, gripY);
  await page.mouse.down();
  await page.mouse.move(gripX - 130, gripY, { steps: 8 });
  await page.mouse.up();
  await expect.poll(async () => (await dialog.boundingBox())!.width).toBeGreaterThan(initialWidth + 100);
  const draggedWidth = (await dialog.boundingBox())!.width;
  await resizeHandle.focus();
  await resizeHandle.press("ArrowRight");
  await expect.poll(async () => (await dialog.boundingBox())!.width).toBeLessThan(draggedWidth);
  await page.screenshot({ path: testInfo.outputPath("story-drawer-resized.png"), fullPage: true });
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(opener).toBeFocused();

  await opener.click();
  await expect.poll(async () => (await dialog.boundingBox())!.width).toBeGreaterThan(initialWidth + 60);
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 390, height: 844 });
  await opener.click();
  await expect(dialog).toHaveCSS("width", "390px");
  await expect(resizeHandle).toBeHidden();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect.poll(async () => (await dialog.boundingBox())!.width).toBeGreaterThan(initialWidth + 60);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.keyboard.press("Escape");
  await page.goto("/materials");
  await page.route("**/api/materials/upload", (route) => route.fulfill({ status: 500, json: { error: { message: "上传失败，请重试" } } }));
  await page.getByLabel("选择素材文件").setInputFiles({ name: "sample.png", mimeType: "image/png", buffer: Buffer.from("invalid") });
  await expect(page.getByRole("alert")).toContainText("上传失败，请重试");
  await expect(page.getByRole("button", { name: "上传素材" })).toBeEnabled();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
});

test("home, studio scene editing and WebSocket connection", async ({ page }) => {
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
  await page.getByRole("button", { name: /分镜/ }).first().click();
  await page.getByRole("button", { name: "新增分镜" }).click();
  await expect(page.locator(".scene-inspector")).toBeVisible();
  await page.locator(".scene-inspector").getByLabel("标题").fill("城市清晨");
  await page.locator(".scene-inspector").getByLabel("画面描述").fill("清晨街道与行人");
  await page.getByRole("button", { name: "保存镜头" }).click();
  await expect(page.locator(".scene-row").filter({ hasText: "城市清晨" })).toBeVisible();
  await page.getByRole("button", { name: "新增分镜" }).click();
  await expect(page.locator(".scene-inspector").getByRole("heading", { name: "镜头 02" })).toBeVisible();
  await page.locator(".scene-inspector").getByLabel("标题").fill("城市傍晚");
  await page.locator(".scene-inspector").getByLabel("画面描述").fill("傍晚街道与车流");
  await page.getByRole("button", { name: "保存镜头" }).click();
  await expect(page.locator(".scene-list .scene-row").nth(1)).toContainText("城市傍晚");
  const moveUp = page.locator(".scene-inspector").getByRole("button", { name: "上移" });
  await moveUp.focus();
  await moveUp.press("Enter");
  await expect(page.locator(".scene-list .scene-row").first()).toContainText("城市傍晚");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".scene-list")).toBeHidden();
  await page.getByRole("button", { name: "返回镜头列表" }).click();
  await expect(page.locator(".scene-list")).toBeVisible();
  await expect.poll(() => upgraded).toBeTruthy();
  await page.locator(".app-header").getByRole("link", { name: "任务中心" }).click();
  await expect(page.getByRole("heading", { name: "任务中心" })).toBeVisible();
});

test("provider switching and responsive layout", async ({ page }) => {
  await page.goto("/config");
  await expect(page.getByRole("heading", { name: "模型管理" })).toBeVisible();
  await page.getByRole("button", { name: /百炼测试/ }).click();
  await expect(
    page.locator(".catalog-table tbody tr").filter({ hasText: "qwen-plus" }),
  ).toBeVisible();
  await page.goto("/studio");
  await page.getByRole("button", { name: "故事联想" }).click();
  await expect(page.getByRole("dialog", { name: "故事联想" }).getByLabel("故事模型")).toHaveValue("e2e-bailian:qwen-plus");
  await page.getByRole("button", { name: "关闭故事联想" }).click();
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
}, testInfo) => {
  await page.route("**/api/materials?*", (route) =>
    route.fulfill({ status: 200, json: { materials: [] } }),
  );
  await page.goto("/materials");
  await expect(page.locator(".material-empty")).toBeVisible();
  expect((await page.locator(".material-empty").boundingBox())!.width).toBeGreaterThan(900);
  await page.screenshot({ path: testInfo.outputPath("materials-empty-desktop.png"), fullPage: true });
  await page.unroute("**/api/materials?*");
  await page.getByRole("button", { name: "角色", exact: true }).click();
  await page.getByLabel("上传素材类别").selectOption("character");
  await page.getByLabel("选择素材文件").setInputFiles({
    name: "character.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAEElEQVR42mNk+M/wHwAF/gL+3MxZ5wAAAABJRU5ErkJggg==",
      "base64",
    ),
  });

  await expect(
    page.getByText("已上传“character.png”。"),
  ).toBeVisible();
  const material = page
    .locator(".material-card")
    .filter({ hasText: "character.png" });
  await expect(material).toBeVisible();
  await expect(material).toHaveClass(/selected/);
  await expect(page.locator(".material-detail")).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("materials-selected-desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".material-detail")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
  await page.screenshot({ path: testInfo.outputPath("materials-selected-mobile.png"), fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1000 });

  const saved = await request.get("/api/materials?kind=character");
  const uploaded = (await saved.json()).materials.find(
    (item: { name: string }) => item.name === "character.png",
  );
  expect(uploaded?.url).toMatch(/^\/media\//);
  expect((await request.get(uploaded.url)).ok()).toBeTruthy();
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "删除", exact: true }).click();
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
  let refreshes = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/tasks?limit=100")) refreshes++;
  });
  await page.goto("/tasks");
  await expect.poll(() => refreshes, { timeout: 10000 }).toBeGreaterThan(1);
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

test("failed story generation preserves the saved body and retry control", async ({ page }) => {
  await page.route("**/api/projects/*/drafts/*/candidates", (route) =>
    route.fulfill({ status: 503, json: { message: "生成服务暂不可用" } }),
  );
  await page.goto("/studio");
  await page.getByLabel("故事正文").fill("用户已写的正文");
  await expect(page.locator(".editor-footer")).toContainText("已保存");
  await page.getByRole("button", { name: "故事联想" }).click();
  const dialog = page.getByRole("dialog", { name: "故事联想" });
  await dialog.getByLabel("你的想法或修改要求").fill("继续写作");
  await dialog.getByRole("button", { name: "生成正文" }).click();
  await expect(page.getByRole("status")).toContainText("生成服务暂不可用");
  await expect(dialog.getByRole("button", { name: "生成正文" })).toBeEnabled();
  await expect(page.getByLabel("故事正文")).toHaveValue("用户已写的正文");
});
