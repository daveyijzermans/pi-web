import { test, expect } from "../lib/test";
import { buildSession, realWorkingDir, uniqueSessionName, writeSession } from "../lib/sessions";

const center = (box: { y: number; height: number }) => box.y + box.height / 2;

// Icon alignment regression pins: inline icon svgs align middle; the project
// header's icon buttons center on the session-count text line; the FAB's Plus
// svg is geometrically centered in its circle.
test("icons align with text and center in controls", async ({ page, sessionsDir }, testInfo) => {
  test.skip(testInfo.project.name !== "Desktop Chrome", "layout is viewport-agnostic; run once");
  writeSession(
    sessionsDir,
    uniqueSessionName(testInfo, "icons"),
    buildSession({ cwd: realWorkingDir() }).entries,
  );
  await page.goto("/");

  const iconSvg = page.locator("svg[aria-hidden='true']").first();
  await expect(iconSvg).toBeAttached();
  expect(await iconSvg.evaluate((el) => getComputedStyle(el).verticalAlign)).toBe("middle");

  await page.locator('[data-layout-btn="projects"]').click();
  const header = page.locator(".project-header").first();
  await expect(header).toBeVisible();
  const countBox = await header.locator(".project-count").boundingBox();
  const newBtnBox = await header.locator(".project-new-btn").boundingBox();
  const archiveBox = await header.locator(".project-archive-btn").boundingBox();
  expect(Math.abs(center(countBox!) - center(newBtnBox!))).toBeLessThanOrEqual(1.5);
  expect(Math.abs(center(countBox!) - center(archiveBox!))).toBeLessThanOrEqual(1.5);

  const fab = page.locator("#newSessionBtn");
  const fabBox = await fab.boundingBox();
  const fabIconBox = await fab.locator("svg").boundingBox();
  expect(Math.abs(center(fabBox!) - center(fabIconBox!))).toBeLessThanOrEqual(1);
});

// Fold chevrons: the span box must fit its icon (a 12px icon in a 10px box
// overflowed and sat off its text line) and center on the toggle's text.
test("fold chevrons fit their box and align with their label", async ({
  page,
  sessionsDir,
}, testInfo) => {
  test.skip(testInfo.project.name !== "Desktop Chrome", "layout is viewport-agnostic; run once");
  const name = uniqueSessionName(testInfo, "chev");
  writeSession(sessionsDir, name, buildSession({ cwd: realWorkingDir() }).entries);
  const archived = buildSession({ cwd: realWorkingDir() });
  archived.entries.push({
    type: "archive",
    id: "arch-1",
    parentId: archived.lastId,
    timestamp: new Date().toISOString(),
    archived: true,
  });
  writeSession(sessionsDir, uniqueSessionName(testInfo, "chev-arch"), archived.entries);
  await page.goto("/");
  await page.locator('[data-layout-btn="projects"]').click();

  const chevron = page.locator(".project-toggle .project-chevron").first();
  await expect(chevron).toBeVisible();
  const spanBox = await chevron.boundingBox();
  const svgBox = await chevron.locator("svg").boundingBox();
  expect(spanBox!.height).toBeGreaterThanOrEqual(svgBox!.height - 0.5);
  expect(Math.abs(center(spanBox!) - center(svgBox!))).toBeLessThanOrEqual(0.5);

  const archToggle = page.locator(".archived-toggle").first();
  await expect(archToggle).toBeVisible();
  const toggleBox = await archToggle.boundingBox();
  const archChevron = await archToggle.locator(".project-chevron").boundingBox();
  expect(Math.abs(center(toggleBox!) - center(archChevron!))).toBeLessThanOrEqual(1);
});

// Back buttons: the ← used to be a unicode glyph whose vertical position
// depended on the interface font; as a Lucide svg it must center on its row.
test("back button arrow centers on its label", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "Desktop Chrome", "layout is viewport-agnostic; run once");
  await page.goto("/schedules");
  const back = page.locator(".session-header-back").first();
  await expect(back).toBeVisible();
  const backBox = await back.boundingBox();
  const arrowBox = await back.locator("svg").first().boundingBox();
  expect(Math.abs(center(backBox!) - center(arrowBox!))).toBeLessThanOrEqual(1);
});

// The mobile sheet-style modal back button (misaligned-on-mobile regression).
test("mobile modal back arrow centers on its bar", async ({ page, sessionsDir }, testInfo) => {
  test.skip(testInfo.project.name !== "Mobile Chrome", "mobile sheet header");
  writeSession(
    sessionsDir,
    uniqueSessionName(testInfo, "mback"),
    buildSession({ cwd: realWorkingDir() }).entries,
  );
  await page.goto("/");
  await page.locator("#newSessionBtn").click();
  const back = page.locator("#modalBackBtn");
  await expect(back).toBeVisible();
  const backBox = await back.boundingBox();
  const arrowBox = await back.locator("svg").first().boundingBox();
  expect(Math.abs(center(backBox!) - center(arrowBox!))).toBeLessThanOrEqual(1);
});

// Session-header controls share one height — mismatched control boxes read
// as misalignment as soon as hover/active paints a background.
test("session header controls share one height", async ({ page, sessionsDir }, testInfo) => {
  test.skip(testInfo.project.name !== "Desktop Chrome", "layout is viewport-agnostic; run once");
  const { entries } = buildSession({ cwd: realWorkingDir() });
  const id = writeSession(sessionsDir, uniqueSessionName(testInfo, "hgt"), entries);
  await page.goto(`/session?id=${encodeURIComponent(id)}`);
  const newBtn = page.locator(".session-header-new");
  await expect(newBtn).toBeVisible();
  const heights = await page.evaluate(() =>
    Array.from(
      document.querySelectorAll(
        ".session-header-bar .session-header-actions, .session-header-bar .session-header-new, .session-header-bar .session-header-shortcuts-help",
      ),
      (el) => el.getBoundingClientRect().height,
    ),
  );
  expect(heights.length).toBeGreaterThanOrEqual(4);
  for (const h of heights) expect(Math.abs(h - heights[0])).toBeLessThanOrEqual(0.5);
});
