import { expect, test } from "@playwright/test";

import { installAlphaMocks } from "../helpers/alphaMocks";
import { browserBootstrapPath } from "../helpers/browserSession";

test("port registry matches the approved conflict view", async ({ page }) => {
  await installAlphaMocks(page);
  await page.route("**/api/v1/ports", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      json: {
        observedAt: "2026-07-16T12:00:00Z",
        warnings: [],
        facts: [
          {
            id: "declared-web",
            kind: "declaration",
            projectId: "api",
            projectName: "Stopped API",
            serviceId: "web",
            host: "0.0.0.0",
            port: 18081,
            protocol: "tcp",
            source: "manifest",
            evidence: "accepted manifest port web",
            observedAt: "2026-07-16T12:00:00Z",
          },
          {
            id: "bound-web",
            kind: "binding",
            projectId: "dashboard",
            projectName: "Running Dashboard",
            serviceId: "web",
            host: "127.0.0.1",
            port: 18081,
            protocol: "tcp",
            source: "process",
            evidence: "native process listener",
            observedAt: "2026-07-16T12:00:00Z",
          },
          {
            id: "compose-db",
            kind: "binding",
            projectId: "api",
            projectName: "Stopped API",
            serviceId: "database",
            host: "127.0.0.1",
            port: 15432,
            protocol: "tcp",
            source: "compose",
            evidence: "Docker published port",
            observedAt: "2026-07-16T12:00:00Z",
          },
        ],
        conflicts: [
          {
            id: "declared-vs-bound-18081",
            type: "DECLARED_VS_BOUND",
            port: 18081,
            summary:
              "Stopped API reserves a port currently bound by Running Dashboard",
            facts: [],
          },
        ],
      },
    });
  });

  await page.goto(browserBootstrapPath());
  await page.getByRole("link", { name: "Ports" }).click();
  await expect(
    page.getByRole("heading", { name: "Port registry" }),
  ).toBeVisible();
  await expect(page).toHaveScreenshot("port-registry.png", {
    animations: "disabled",
    fullPage: true,
  });
});
