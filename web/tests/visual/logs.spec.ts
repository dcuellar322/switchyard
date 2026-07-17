import { expect, test } from "@playwright/test";

import { installAlphaMocks } from "../helpers/alphaMocks";
import { browserBootstrapPath } from "../helpers/browserSession";

test("fleet logs expose bounded filters and configurable auto refresh", async ({ page }) => {
  await installAlphaMocks(page);
  await page.goto(browserBootstrapPath("/logs"));

  await expect(page.getByRole("heading", { name: "Logs" })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Auto refresh" })).toBeChecked();
  await expect(page.getByRole("combobox", { name: "Refresh interval" })).toHaveValue("5000");
  await expect(page.getByText("connection pool recovered after 120ms").first()).toBeVisible();
  await expect(page).toHaveScreenshot("fleet-logs.png", {
    animations: "disabled",
    fullPage: true,
  });
});
