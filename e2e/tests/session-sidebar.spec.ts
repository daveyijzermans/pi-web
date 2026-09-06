import { test, expect, collapseScratchpad } from "../lib/test";
import {
  buildSession,
  realWorkingDir,
  uniqueSessionName,
  writeSession,
} from "../lib/sessions";

test.describe("session sidebar", () => {
  test("highlights the active session and marks a running session with runcat", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "active styling and running status are viewport-agnostic; run once",
    );

    const cwd = realWorkingDir();
    const activeName = uniqueSessionName(testInfo, "sidebar-active");
    const idleName = uniqueSessionName(testInfo, "sidebar-idle");
    const activeId = writeSession(
      sessionsDir,
      activeName,
      buildSession({ cwd }).entries,
    );
    const idleId = writeSession(
      sessionsDir,
      idleName,
      buildSession({ cwd }).entries,
    );

    await collapseScratchpad(page);
    await page.goto(`/session?id=${encodeURIComponent(activeId)}`);
    await expect(page.locator("#pi-chat-composer")).toHaveAttribute(
      "data-chat-available",
      "true",
    );

    const activeCard = page.locator(
      `.sidebar-session-row[href="/session?id=${encodeURIComponent(activeId)}"]`,
    );
    const idleCard = page.locator(
      `.sidebar-session-row[href="/session?id=${encodeURIComponent(idleId)}"]`,
    );

    await expect(activeCard).toBeVisible();
    await expect(idleCard).toBeVisible();
    await expect(activeCard).toHaveAttribute("aria-current", "page");

    const [activeBackground, idleBackground] = await Promise.all([
      activeCard.evaluate((element) => getComputedStyle(element).backgroundColor),
      idleCard.evaluate((element) => getComputedStyle(element).backgroundColor),
    ]);
    expect(activeBackground).not.toBe(idleBackground);
    expect(activeBackground).not.toBe("rgba(0, 0, 0, 0)");

    await page
      .locator("#pi-chat-message")
      .fill("Keep the sidebar cat running [[slow:30000]]");
    await page.locator("#pi-chat-send").click();

    await expect(activeCard).toHaveClass(/sidebar-session-row--running/, {
      timeout: 20000,
    });
    const indicator = activeCard.locator(".sidebar-session-indicator");
    await expect(indicator).toBeVisible();
    await expect(indicator).toHaveClass(/sidebar-session-indicator--running/);
    await expect(indicator).toHaveCSS("font-family", /runcat/);
    await expect(indicator).not.toHaveText("");
  });

  test("groups archived sessions under a collapsed toggle", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "grouping is viewport-agnostic; run once",
    );
    const cwd = realWorkingDir();
    const activeName = uniqueSessionName(testInfo, "sidebar-live");
    const archivedName = uniqueSessionName(testInfo, "sidebar-archived");
    const activeId = writeSession(sessionsDir, activeName, buildSession({ cwd }).entries);
    const archived = buildSession({ cwd });
    archived.entries.push({
      type: "archive",
      id: "arch-sidebar",
      parentId: archived.lastId,
      timestamp: new Date().toISOString(),
      archived: true,
    });
    const archivedId = writeSession(sessionsDir, archivedName, archived.entries);

    await collapseScratchpad(page);
    await page.goto(`/session?id=${encodeURIComponent(activeId)}`);

    const archivedRow = page.locator(
      `.sidebar-session-row[href="/session?id=${encodeURIComponent(archivedId)}"]`,
    );
    const toggle = page.locator(".sidebar-archived-toggle");
    await expect(toggle).toBeVisible();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(archivedRow).toHaveCount(0);

    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await expect(archivedRow).toBeVisible();
  });

  test("archives the open session from the overflow menu", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "menu action is viewport-agnostic; run once",
    );
    const cwd = realWorkingDir();
    const id = writeSession(
      sessionsDir,
      uniqueSessionName(testInfo, "menu-archive"),
      buildSession({ cwd }).entries,
    );
    await collapseScratchpad(page);
    await page.goto(`/session?id=${encodeURIComponent(id)}`);
    await expect(page.locator("#pi-chat-composer")).toHaveAttribute(
      "data-chat-available",
      "true",
    );

    await page.locator("#command-menu-btn").click();
    const item = page.locator('#command-menu-popover [data-action="archive"]');
    await expect(item).toHaveText(/Archive/);
    await item.click();
    await expect(page.locator("#command-menu-toast")).toContainText("Session archived");

    await page.locator("#command-menu-btn").click();
    await expect(item).toHaveText(/Restore from archive/);

    // The current session now lives in the sidebar's archived group, which
    // auto-opens because it holds the open session.
    const row = page.locator(
      `.sidebar-session-group--archived .sidebar-session-row[href="/session?id=${encodeURIComponent(id)}"]`,
    );
    await expect(row).toBeVisible({ timeout: 10000 });
  });
});
