import { defineConfig, devices } from "@playwright/test";

const isCI = !!process.env.CI;

export default defineConfig({
  testDir: "./tests",
  testIgnore: "**/*.real.spec.ts",
  // @screenshots specs overwrite committed docs/screenshots assets — they are
  // capture tools, not tests, and a plain run must never touch tracked
  // binaries. Opt in with:
  //   E2E_SCREENSHOTS=1 npx playwright test --grep @screenshots
  grepInvert: process.env.E2E_SCREENSHOTS ? undefined : /@screenshots/,
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers: isCI ? 2 : undefined,
  reporter: isCI ? [["html", { open: "never" }], ["list"]] : [["list"]],
  globalSetup: "./global-setup.ts",
  globalTeardown: "./global-teardown.ts",
  use: {
    trace: "on-first-retry",
    // baseURL is injected per-test by the fixture in lib/test.ts from the
    // server started in global-setup (random free port).
  },
  // Tiered matrix: default = 2 projects (Chrome desktop + mobile) for fast PR feedback.
  // Set E2E_FULL_MATRIX=1 (push to main in CI) for the full 7-project matrix.
  projects: process.env.E2E_FULL_MATRIX === 'true'
    ? [
        { name: "Desktop Chrome", use: { ...devices["Desktop Chrome"] } },
        { name: "Desktop Firefox", use: { ...devices["Desktop Firefox"] } },
        { name: "Desktop Safari", use: { ...devices["Desktop Safari"] } },
        { name: "Mobile Chrome", use: { ...devices["Pixel 5"] } },
        { name: "Mobile Safari", use: { ...devices["iPhone 13"] } },
        // iPad portrait (810px) -> mobile layout; landscape (~1080px) -> desktop layout.
        { name: "iPad", use: { ...devices["iPad (gen 7)"] } },
        { name: "iPad landscape", use: { ...devices["iPad (gen 7) landscape"] } },
      ]
    : [
        { name: "Desktop Chrome", use: { ...devices["Desktop Chrome"] } },
        { name: "Mobile Chrome", use: { ...devices["Pixel 5"] } },
      ],
});
