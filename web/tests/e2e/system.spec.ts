import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

import { browserBootstrapPath } from "../helpers/browserSession";

test("renders the alpha shell, keyboard palette, and live daemon settings", async ({
  page,
}) => {
  await page.goto(browserBootstrapPath());
  await expect(
    page.getByRole("heading", { name: "Your development yard" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Projects", exact: true }),
  ).toBeVisible();
  const accessibility = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  expect(
    accessibility.violations.filter((violation) =>
      ["serious", "critical"].includes(violation.impact ?? ""),
    ),
  ).toEqual([]);

  await page.keyboard.press("Meta+k");
  await expect(
    page.getByRole("dialog", { name: "Command palette" }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Type a command or project" })
    .fill("scan");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/discovery$/);
  await expect(
    page.getByRole("heading", { name: "Bring a repository into the yard." }),
  ).toBeVisible();

  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByText("Switchyard daemon")).toBeVisible();
  await expect(page.getByText("ready", { exact: true })).toBeVisible();
  await expect(page.getByText("API / schema")).toBeVisible();
  await expect(page.getByText(/Revision \d+/)).toBeVisible();

  const logAge = page.getByLabel(/Log age/);
  const originalLogDays = await logAge.inputValue();
  const changedLogDays = originalLogDays === "8" ? "9" : "8";
  await logAge.fill(changedLogDays);
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText(/Restart the daemon to apply/)).toContainText(
    "retention",
  );
  await logAge.fill(originalLogDays);
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText(/Restart the daemon to apply/)).toBeHidden();
});
