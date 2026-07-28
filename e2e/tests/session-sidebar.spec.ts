import { test, expect, collapseScratchpad } from "../lib/test";
import {
  buildSession,
  realWorkingDir,
  uniqueSessionName,
  writeSession,
} from "../lib/sessions";

test.describe("session sidebar", () => {
  test("renders surfaced cards and marks a running session with runcat", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "card styling and running status are viewport-agnostic; run once",
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
    await expect(activeCard).toHaveCSS("border-radius", "7px");
    await expect(idleCard).toHaveCSS("border-radius", "7px");

    const [activeStyle, idleStyle] = await Promise.all([
      activeCard.evaluate((element) => {
        const style = getComputedStyle(element);
        return { background: style.backgroundColor, border: style.borderColor };
      }),
      idleCard.evaluate((element) => {
        const style = getComputedStyle(element);
        return { background: style.backgroundColor, border: style.borderColor };
      }),
    ]);
    expect(activeStyle.background).not.toBe(idleStyle.background);
    expect(activeStyle.border).not.toBe(idleStyle.border);
    expect(idleStyle.background).not.toBe("rgba(0, 0, 0, 0)");

    await page
      .locator("#pi-chat-message")
      .fill("Keep the sidebar cat running [[slow:30000]]");
    await page.locator("#pi-chat-send").click();

    await expect(activeCard).toHaveClass(/sidebar-session-row--running/, {
      timeout: 20000,
    });
    const spinner = activeCard.locator("[data-running-spinner]");
    await expect(spinner).toBeVisible();
    await expect(spinner).toHaveCSS("font-family", /runcat/);
    await expect(spinner).not.toHaveText("");
  });
});
