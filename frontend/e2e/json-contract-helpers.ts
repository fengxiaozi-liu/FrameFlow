import type { Locator } from "@playwright/test";

export async function pasteContractJSON(
  drawer: Locator,
  title: string,
  example: string,
) {
  const editor = drawer.getByLabel(`${title} JSON 示例`, { exact: true });
  const toggle = drawer.getByLabel(`${title} 粘贴或修改 JSON`, { exact: true });
  if (!(await editor.isVisible())) await toggle.click();
  await editor.fill(example);
  // Replacement examples may remove seeded paths. Clear them explicitly as a user would.
  const stale = drawer.getByRole("button", {
    name: new RegExp(`^${title} 解除.+绑定$`),
  });
  while (await stale.count()) await stale.first().click();
  await toggle.click();
}
