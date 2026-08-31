import { test, expect } from "../lib/test";
import { buildSession, realWorkingDir, uniqueSessionName, writeSession } from "../lib/sessions";

// Manage Projects dialog. Regression guard for the filter-off dimming: with
// the master "Filter projects" toggle OFF, only the controls it gates (the
// per-project enable checkboxes and Select all) may dim — the project names
// and the delete/remove action buttons stay fully opaque and enabled, because
// those actions work regardless of the filter. This has regressed before:
// dimming the whole list made working buttons look disabled.
test.describe("manage projects dialog", () => {
  test("filter off dims only the filter controls, never names or actions", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "dialog styling is viewport-agnostic; run once",
    );

    // A real project row to assert against.
    const cwd = realWorkingDir();
    writeSession(sessionsDir, uniqueSessionName(testInfo, "mp"), buildSession({ cwd }).entries);

    await page.goto("/");
    await page.locator("#web-menu-btn").click();
    await page.locator("#manage-projects-btn").click();
    await expect(page.locator("#projectsModalOverlay")).toHaveClass(/open/);

    const config = page.locator("#projectsConfig");
    const toggle = page.locator("#projectsFilterToggle");
    // The checkbox input sits under the styled .switch-slider, which
    // intercepts pointer events — click the visible switch instead.
    const toggleSwitch = page.locator(".projects-filter-switch .switch");
    const row = page.locator(".project-row").first();
    await expect(row).toBeVisible();

    // Force the filter OFF (state persists server-side across tests).
    if (await toggle.isChecked()) {
      await toggleSwitch.click();
    }
    await expect(config).toHaveClass(/filter-off/);

    const opacity = (locator: import("@playwright/test").Locator) =>
      locator.evaluate((el) => getComputedStyle(el).opacity);

    // Gated filter controls read as inactive…
    expect(await opacity(row.locator('input[type="checkbox"]'))).toBe("0.5");
    expect(await opacity(page.locator("#projectsToggleAllBtn"))).toBe("0.5");
    // …but names and management actions do not.
    expect(await opacity(row.locator(".project-row-name"))).toBe("1");
    const action = row.locator(".project-row-remove").first();
    await expect(action).toBeVisible();
    await expect(action).toBeEnabled();
    expect(await opacity(action)).toBe("1");

    // Filter back ON clears the inactive styling entirely.
    await toggleSwitch.click();
    await expect(config).not.toHaveClass(/filter-off/);
    expect(await opacity(row.locator('input[type="checkbox"]'))).toBe("1");
  });
});
