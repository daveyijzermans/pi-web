import { test, expect } from "../lib/test";
import {
  buildSession,
  realWorkingDir,
  uniqueSessionName,
  writeSession,
} from "../lib/sessions";

// Streamed content must never disappear mid-turn. The stub pi's
// "[[tooly:GAP]]" turn mirrors real pi's event + flush order: each assistant
// message streams its text, then its tool-call args stream *silently* (no
// preview events), and the entry only flushes at message_end — so freshly
// streamed text exists ONLY in the preview until the args finish. A jsonl
// reload landing in that window (watcher debounce + slow fetch on a phone)
// must not clear the preview: the canonical copy of that text isn't on disk
// yet, so clearing it makes the text vanish until the message fully lands.
//
// A MutationObserver records every DOM change where a previously-fully-visible
// phrase is absent from the page; the tests assert no such moment occurred.

const PHRASE_1 = "Alpha beta gamma.";
const PHRASE_2 = "Delta epsilon zeta.";
const PHRASE_3 = "Omega finale.";
const ALL_PHRASES = [PHRASE_1, PHRASE_2, PHRASE_3];

declare global {
  interface Window {
    __piVanished?: { phrase: string; at: number }[];
  }
}

async function installVanishRecorder(page, phrases: string[]) {
  await page.evaluate((phrases: string[]) => {
    window.__piVanished = [];
    const seen = new Set<string>();
    const check = () => {
      const text = document.body.innerText;
      for (const phrase of phrases) {
        if (text.includes(phrase)) {
          seen.add(phrase);
        } else if (seen.has(phrase)) {
          window.__piVanished!.push({ phrase, at: Date.now() });
          seen.delete(phrase); // re-arm so one regression records once per gap
        }
      }
    };
    const observer = new MutationObserver(check);
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      characterData: true,
    });
  }, phrases);
}

async function openStubSession(page, sessionsDir, testInfo, prefix: string) {
  const cwd = realWorkingDir();
  const { entries } = buildSession({ cwd });
  const name = uniqueSessionName(testInfo, prefix);
  const id = writeSession(sessionsDir, name, entries);

  await page.goto(`/session?id=${encodeURIComponent(id)}`);
  await expect(page.locator("#pi-chat-composer")).toHaveAttribute(
    "data-chat-available",
    "true",
  );
  return id;
}

async function runToolyTurnAndCollect(page): Promise<{ phrase: string; at: number }[]> {
  await page.locator("#pi-chat-message").fill("probe [[tooly:1500]]");
  await page.locator("#pi-chat-send").click();

  // Message 1 streams into the preview.
  await expect(page.locator("body")).toContainText(PHRASE_1, {
    timeout: 15_000,
  });
  // Turn completes: the closing message is canonical in #messages.
  await expect(page.locator("#messages")).toContainText(PHRASE_3, {
    timeout: 30_000,
  });

  // Once the turn has settled, the preview host must be empty: every preview
  // chunk (text or thinking-only) is reconciled/removed, not stranded at the
  // bottom of the page. A thinking-only message has no answer text to match,
  // so a stranded chunk would linger here until a refresh.
  await expect
    .poll(
      () =>
        page.evaluate(
          () => document.getElementById("chat-preview-host")?.textContent?.trim() ?? "",
        ),
      { timeout: 15_000 },
    )
    .toBe("");

  return page.evaluate(() => window.__piVanished);
}

test.describe("streaming preview persistence (stubbed pi)", () => {
  test("streamed text survives tool-call flushes and reloads", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "timing-sensitive; one project is enough",
    );

    await openStubSession(page, sessionsDir, testInfo, "stream-persist");
    await installVanishRecorder(page, ALL_PHRASES);

    const vanished = await runToolyTurnAndCollect(page);
    expect(vanished).toEqual([]);
  });

  test("streamed text survives slow session fetches (phone-grade latency)", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "timing-sensitive; one project is enough",
    );

    await openStubSession(page, sessionsDir, testInfo, "stream-slowfetch");
    await installVanishRecorder(page, ALL_PHRASES);

    // Delay every /api/session reload fetch so it resolves ~2s late — the
    // reload triggered by one message's flush then lands inside the NEXT
    // message's silent tool-args window, where its text is preview-only.
    let sentAt = 0;
    await page.route("**/api/session?*", async (route) => {
      if (sentAt) await new Promise((r) => setTimeout(r, 2000));
      return route.continue();
    });

    sentAt = Date.now();
    const vanished = await runToolyTurnAndCollect(page);
    expect(vanished).toEqual([]);
  });

  test("streamed text survives a failed worker-status poll", async ({
    page,
    sessionsDir,
  }, testInfo) => {
    test.skip(
      testInfo.project.name !== "Desktop Chrome",
      "timing-sensitive; one project is enough",
    );

    await openStubSession(page, sessionsDir, testInfo, "stream-pollfail");
    await installVanishRecorder(page, ALL_PHRASES);

    // Fail exactly one status poll mid-turn (>=1s after send) — a transient
    // network error on a phone must not tear down the streaming view.
    let sentAt = 0;
    let failed = false;
    await page.route("**/api/worker-status*", (route) => {
      if (sentAt && !failed && Date.now() - sentAt > 1000) {
        failed = true;
        return route.abort();
      }
      return route.continue();
    });

    await page.locator("#pi-chat-message").fill("probe [[tooly:1500]]");
    await page.locator("#pi-chat-send").click();
    sentAt = Date.now();

    await expect(page.locator("body")).toContainText(PHRASE_1, {
      timeout: 15_000,
    });
    await expect(page.locator("#messages")).toContainText(PHRASE_3, {
      timeout: 30_000,
    });

    expect(failed).toBe(true); // the poll failure actually happened mid-turn
    const vanished = await page.evaluate(() => window.__piVanished);
    expect(vanished).toEqual([]);
  });
});
