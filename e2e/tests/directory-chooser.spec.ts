import { test, expect } from "../lib/test";

test.describe("directory chooser", () => {
  // The chooser is a desktop, mouse-driven flow. WebKit's touch emulation
  // (iPad / Mobile Safari) doesn't deliver dblclick-to-descend, and the mobile
  // sheet layout doesn't surface the recent-location chips, so skip those
  // projects. Desktop Chrome/Firefox/Safari and Mobile Chrome still run.
  test.beforeEach(({}, testInfo) => {
    test.skip(
      /iPad|Mobile Safari/.test(testInfo.project.name),
      "directory chooser is a desktop mouse flow; skip WebKit touch projects",
    );
  });

  test("opens directory browser instead of text input", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Directory browser should be present
    await expect(page.locator(".directory-browser")).toBeVisible();

    // Old text input should NOT be present
    await expect(page.locator("#sessionPath")).not.toBeVisible();
  });

  test("shows entries from home directory on open", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load (at least one entry expected)
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Path display should show the current path
    await expect(page.locator(".path-value")).toBeVisible();
  });

  test("search filters entries in real-time", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Get initial entry count
    const initialCount = await page.locator(".browser-entry").count();

    // Type a search query that won't match anything
    await page.locator(".search-input").fill("xyznonexistent123");

    // Should show no entries
    await expect(page.locator(".browser-entry")).toHaveCount(0);

    // Clear search — entries should return
    await page.locator(".search-input").fill("");
    await expect(page.locator(".browser-entry")).toHaveCount(initialCount);
  });

  test("shows breadcrumbs for navigation", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Breadcrumbs should be visible
    const breadcrumbCount = await page.locator(".breadcrumb-item").count();
    expect(breadcrumbCount).toBeGreaterThan(0);
  });

  test("clicking a directory navigates into it", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Get the first directory entry (has .dir class)
    const dirEntries = page.locator(".browser-entry.dir");
    const dirCount = await dirEntries.count();

    if (dirCount > 0) {
      const firstDirName = (
        await dirEntries.first().locator(".entry-name").textContent()
      )?.trim();
      await dirEntries.first().dblclick();

      // Breadcrumbs should eventually include the subdirectory (the reload is
      // async; poll instead of a fixed wait so slower engines pass too).
      await expect
        .poll(
          async () =>
            (await page.locator(".breadcrumb-item").allTextContents()).map(
              (s) => s.trim(),
            ),
          { timeout: 10000 },
        )
        .toContain(firstDirName);
    }
  });

  test("recent location chips are visible", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Recent location chips load asynchronously; wait for the first one.
    await expect(page.locator(".recent-chip").first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("clicking a recent chip sets the path", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Click a recent chip if available
    const chips = page.locator(".recent-chip");
    const chipCount = await chips.count();

    if (chipCount > 0) {
      const chipText = await chips.first().textContent();
      await chips.first().click();

      // Path display should update
      await expect(page.locator(".path-value")).toContainText(chipText!, {
        ignoreCase: true,
      });
    }
  });

  test("create button is enabled when path is selected", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for create button to be enabled (path auto-selected)
    await expect(page.locator("#createBtn")).toBeEnabled({ timeout: 10000 });
  });

  test("full flow: browse → select → create → navigate to session", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    // Take screenshot: session overview
    await page.screenshot({
      path: ".shots/directory-chooser-01-overview.png",
    });

    // Click new session
    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for directory browser to load entries
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Wait for create button to be enabled (path auto-selected)
    await expect(page.locator("#createBtn")).toBeEnabled({ timeout: 10000 });

    // Take screenshot: directory browser open
    await page.screenshot({
      path: ".shots/directory-chooser-02-browser-open.png",
    });

    // Verify browser is visible
    await expect(page.locator(".directory-browser")).toBeVisible();

    // Select first entry (click to select)
    const firstEntry = page.locator(".browser-entry").first();
    await firstEntry.click();

    // Take screenshot: entry selected
    await page.screenshot({
      path: ".shots/directory-chooser-03-entry-selected.png",
    });

    // Verify path display shows selected path
    await expect(page.locator(".path-value")).toBeVisible();

    // Click Create
    await page.locator("#createBtn").click();

    // Should navigate to session page
    await expect(page).toHaveURL(/\/session\?id=/, { timeout: 15000 });

    // Take screenshot: new session loaded
    await page.screenshot({
      path: ".shots/directory-chooser-04-session-loaded.png",
    });

    // Verify session page elements are visible
    await expect(page.locator("#pi-chat-composer")).toBeVisible({
      timeout: 15000,
    });
  });

  test("keyboard navigation: arrow keys highlight entries", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Focus the directory browser
    await page.locator(".directory-browser").focus();

    // Press ArrowDown to highlight first entry
    await page.keyboard.press("ArrowDown");

    // First entry should be highlighted
    await expect(page.locator(".browser-entry.highlighted")).toHaveCount(1);

    // Press ArrowDown again
    await page.keyboard.press("ArrowDown");

    // Still one highlighted (second entry)
    await expect(page.locator(".browser-entry.highlighted")).toHaveCount(1);

    // Press ArrowUp to go back
    await page.keyboard.press("ArrowUp");

    // Should highlight first entry again
    await expect(page.locator(".browser-entry.highlighted")).toHaveCount(1);
  });

  test("breadcrumb click navigates up", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for entries to load
    await expect(page.locator(".browser-entry")).not.toHaveCount(0, {
      timeout: 10000,
    });

    // Navigate into first directory
    const dirEntries = page.locator(".browser-entry.dir");
    const dirCount = await dirEntries.count();

    if (dirCount > 0) {
      await dirEntries.first().dblclick();
      await page.waitForTimeout(500);

      // Click first breadcrumb to navigate up
      const firstBreadcrumb = page.locator(".breadcrumb-item").first();
      await firstBreadcrumb.click();
      await page.waitForTimeout(500);

      // Path should be visible (navigated)
      await expect(page.locator(".path-value")).toBeVisible();
    }
  });

  test("full flow: create session and type first message", async ({ page }) => {
    await page.goto("/");
    await page.locator("[data-sessions-content].index-layout-ready").waitFor();

    // Click new session
    await page.locator("#newSessionBtn").click();
    await expect(page.locator("#modalOverlay")).toBeVisible();

    // Wait for directory browser to load and auto-select path
    await expect(page.locator("#createBtn")).toBeEnabled({ timeout: 10000 });

    // Click Create (home directory is auto-selected)
    await page.locator("#createBtn").click();

    // Should navigate to session page
    await expect(page).toHaveURL(/\/session\?id=/, { timeout: 15000 });

    // Wait for composer to load
    await page.locator("#pi-chat-composer").waitFor({ state: "visible" });

    // Take screenshot: composer ready
    await page.screenshot({
      path: ".shots/directory-chooser-05-composer-ready.png",
    });

    // Type a message
    await page.locator("#pi-chat-composer textarea").fill("Hello, world!");

    // Take screenshot: message typed
    await page.screenshot({
      path: ".shots/directory-chooser-06-message-typed.png",
    });

    // Verify message is in the textarea
    await expect(page.locator("#pi-chat-composer textarea")).toHaveValue(
      "Hello, world!",
    );
  });
});
