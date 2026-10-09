import { test, expect, type APIRequestContext } from "@playwright/test";
import { randomUUID } from "node:crypto";

async function seed(request: APIRequestContext) {
  const id = randomUUID();
  expect(
    (
      await request.post("/api/connections", {
        data: {
          id,
          name: `选择测试 ${id}`,
          vendor: "custom",
          base_url: "https://example.com/v1",
        },
      })
    ).ok(),
  ).toBeTruthy();
  for (const [model, capability] of [
    ["qwen-plus", "story"],
    ["qwen-image", "image"],
  ]) {
    expect(
      (
        await request.post(`/api/connections/${id}/models`, {
          data: { model_id: model, capabilities: [capability] },
        })
      ).ok(),
    ).toBeTruthy();
  }
  return id;
}

test("clicking the selected type filter again restores all models", async ({
  page,
  request,
}) => {
  const id = await seed(request);
  await page.goto("/config");
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  const filters = page.getByLabel("模型类型", { exact: true });
  const text = filters.getByRole("button", { name: "文本生成", exact: true });
  const image = page.getByRole("row").filter({ hasText: "qwen-image" });
  await expect(image).toBeVisible();
  await text.click();
  await expect(text).toHaveAttribute("aria-pressed", "true");
  await expect(image).toHaveCount(0);
  await text.click();
  await expect(text).toHaveAttribute("aria-pressed", "false");
  await expect(
    filters.getByRole("button", { name: "全部", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(image).toBeVisible();
});

test("capability, enabled and default checkboxes can be cleared and stay cleared on reload", async ({
  page,
  request,
}) => {
  const id = await seed(request);
  await page.goto("/config");
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  const row = page.getByRole("row").filter({ hasText: "qwen-plus" });
  const enabled = row.getByLabel("启用", { exact: true });
  const isDefault = row.getByLabel("设为默认模型", { exact: true });
  await isDefault.check();
  await expect(isDefault).toBeEnabled();
  await isDefault.uncheck();
  await expect(isDefault).toBeEnabled();
  await page.reload();
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  await expect(isDefault).not.toBeChecked();
  await enabled.uncheck();
  await expect(enabled).toBeEnabled();
  await page.reload();
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  await expect(enabled).not.toBeChecked();
  const capability = row.getByLabel("文本生成", { exact: true });
  await capability.uncheck();
  await expect(capability).toBeEnabled();
  await page.reload();
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  await expect(capability).not.toBeChecked();
  await expect(row).toContainText("待配置能力");
  const { models } = await (
    await request.get(`/api/models?connection_id=${id}`)
  ).json();
  const saved = models.find(
    (model: { remote_id: string }) => model.remote_id === "qwen-plus",
  );
  expect(saved.capabilities).toEqual([]);
  expect(saved.capabilities_configured).toBe(true);
  expect(saved.enabled).toBe(false);
  expect(saved.default).toBe(false);
});

test("failed deselection restores the saved checkbox state and reports the error", async ({
  page,
  request,
}) => {
  const id = await seed(request);
  await page.goto("/config");
  await page.getByRole("button", { name: `选择测试 ${id}` }).click();
  await page.route(`**/api/connections/${id}/models/**`, async (route) => {
    if (route.request().method() === "PATCH") {
      await route.fulfill({
        status: 409,
        json: { error: { message: "配置更新冲突，请重试" } },
      });
    } else {
      await route.continue();
    }
  });
  const enabled = page
    .getByRole("row")
    .filter({ hasText: "qwen-plus" })
    .getByLabel("启用", { exact: true });
  await enabled.uncheck();
  await expect(page.getByRole("alert")).toContainText("配置更新冲突，请重试");
  await expect(enabled).toBeChecked();
});
