import { test, expect, collapseScratchpad } from "../lib/test";
import { uniqueSessionName, writeSession } from "../lib/sessions";
import { buildBenchmarkSession } from "../lib/benchmark-session";

/**
 * Benchmark session E2E spec.
 *
 * Validates that a large, diverse, deliberately-messy session renders correctly
 * in the UI. Covers entry-type diversity at scale: tool calls, tool results,
 * thinking blocks, model switches, thinking-level changes, renames/auto-titles,
 * labels — the messy mix a real long session accumulates.
 *
 * Distinct from load-earlier.spec.ts which tests pagination with plain text.
 * This spec's value-add is entry-type rendering correctness at scale.
 */
test.describe("benchmark session rendering", () => {
  const WINDOW_TIMEOUT = 15_000;

  test("renders large diverse session without errors", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    // Collapse scratchpad before navigating (narrow-viewport click interception)
    await collapseScratchpad(page);

    // Build and write the benchmark session
    const entries = buildBenchmarkSession();
    const name = uniqueSessionName(testInfo, "bench");
    const id = writeSession(sessionsDir, name, entries);

    // Register page-error listener to catch uncaught errors during load/render
    const pageErrors: string[] = [];
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });

    // Navigate to the session
    await page.goto(`/session?id=${encodeURIComponent(id)}`);

    // Wait for the session view to render
    const messagesContainer = page.locator("#messages");
    await expect(messagesContainer).toBeVisible({ timeout: WINDOW_TIMEOUT });

    // Assert no uncaught page errors occurred during load/render
    expect(pageErrors, "no uncaught page errors during render").toEqual([]);

    // ---- Assert latest marker present (session loaded) ----
    await expect(page.locator("#messages")).toContainText("BENCH_LATEST_MARKER", {
      timeout: WINDOW_TIMEOUT,
    });

    // ---- Assert tool call rendered ----
    // Tool calls render inline as .tool-execution blocks (SessionEntry.svelte
    // → ToolCall.svelte; the chip/bottom-sheet UI was removed).
    const toolCalls = page.locator(".tool-execution");
    await expect(toolCalls.first()).toBeVisible({ timeout: WINDOW_TIMEOUT });
    expect(await toolCalls.count()).toBeGreaterThan(0);

    // Assert the tool call marker text is present
    await expect(page.locator("#messages")).toContainText("BENCH_TOOLCALL_MARKER", {
      timeout: WINDOW_TIMEOUT,
    });

    // ---- Assert thinking block rendered ----
    // Thinking renders inline as a .thinking-block with .thinking-text.
    await expect(page.locator(".thinking-block .thinking-text").first()).toBeAttached({
      timeout: WINDOW_TIMEOUT,
    });

    // ---- Assert model-change indicator rendered ----
    // Model changes render as <div class="model-change"> with a .model-name span
    // (SessionEntry.svelte line ~183)
    const modelChange = page.locator(".model-change");
    await expect(modelChange.first()).toBeVisible({ timeout: WINDOW_TIMEOUT });

    // The model-name span shows "provider/modelId"
    await expect(page.locator(".model-name")).toBeVisible({ timeout: WINDOW_TIMEOUT });

    // ---- Handle pagination (if session crosses truncation threshold) ----
    // The e2e server lowers PI_WEB_LARGE_SESSION_THRESHOLD=100, so a 400+ entry
    // session will show the load-earlier banner. We detect whether the banner
    // is present and click through if needed.
    const banner = page.locator("#load-earlier-banner");
    const bannerVisible = await banner.isVisible().catch(() => false);

    if (bannerVisible) {
      // Banner is visible — click through windows until earliest marker appears
      await expect(banner).toContainText(/Showing latest .* of .* messages/);

      for (let i = 0; i < 10 && (await banner.count()) > 0; i++) {
        await banner.getByRole("button").click();
        // Wait for this window to settle
        await page
          .waitForFunction(
            () => {
              const b = document.querySelector("#load-earlier-banner");
              const btn = b?.querySelector("button");
              return !b || (btn != null && !(btn as HTMLButtonElement).disabled);
            },
            { timeout: WINDOW_TIMEOUT },
          )
          .catch(() => {});
      }

      // Banner should be gone after clicking through
      await expect(banner).toHaveCount(0, { timeout: WINDOW_TIMEOUT });
    }

    // ---- Assert earliest marker rendered (after pagination if applicable) ----
    await expect(page.locator("#messages")).toContainText("BENCH_EARLIEST_MARKER", {
      timeout: WINDOW_TIMEOUT,
    });
  });
});
