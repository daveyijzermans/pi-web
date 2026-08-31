<script>
  // Live reload (SSE) — drives the streaming chat preview, follow/scroll, stats,
  // and reconciles the shared reactive model when the session JSONL changes. The
  // Svelte <SessionContent> owns #messages and re-renders from the model, so this
  // never patches the message DOM (reactive-only): on reload it reconciles the
  // model through the session runtime context and only tracks brand-new ids for the
  // follow/scroll/highlight decisions. Live-only: never imported by the static
  // export bundle.
  //
  // The old live-reload runner has been split between this component and focused
  // live-only helpers in session/live/: connection/reconnect lifecycle, reload
  // events, follow-scroll, stats, and chat-preview all have focused unit tests.
  import { onMount } from 'svelte';
  import { marked } from 'marked';
  import { escapeHtml } from '../../session/render/session-format.js';
  import { safeMarkedParse } from '../../session/render/markdown.js';
  import {
    clearChatPreviewState,
    finishChatPreviewState,
    reconcilePreviewsWithCanonical,
    renderChatPreviewState,
    renderPendingChatState,
    startRunningSpinner,
    stopRunningSpinner,
  } from '../../session/live/chat-preview.js';
  import { getSessionIdFromLocation, handleSessionReload } from '../../session/live/live-events.js';

  import { setupSessionLiveConnection } from '../../session/live/live-connection.js';
  import { createFollowScrollController } from '../../session/live/live-follow.js';
  import { findNewEntryHighlightTargets } from '../../session/live/new-entry-highlight.js';
  import { updateStatsDom } from '../../session/live/live-stats.js';
  import { sessionRuntime } from '../../session/session-runtime.js';
  import { getSessionRuntime } from '../../session/session-runtime-context.js';
  import { setSessionTitle } from '../../session/session-title.svelte.js';
  import { chatRunningStore } from './chat/chat-toolbar-state.svelte.js';

  onMount(() => {
    const documentImpl = document;
    const windowImpl = window;
    const runtime = getSessionRuntime();
    const model = runtime.model;
    const reconcileEntries = runtime.reconcileEntries || (() => {});
    globalThis.__PI_TEST_LIVE_RELOAD_HOOK__?.();

    const fetchImpl = windowImpl.fetch.bind(windowImpl);
    const requestAnimationFrame = windowImpl.requestAnimationFrame.bind(windowImpl);
    const setTimeout = windowImpl.setTimeout.bind(windowImpl);

    const cleanups = [];
    const on = (host, type, handler, opts) => {
      host.addEventListener(type, handler, opts);
      cleanups.push(() => host.removeEventListener(type, handler, opts));
    };

    // Markdown for the streaming preview — globally-configured (sanitized) marked
    // with an escapeHtml fallback (matches the former live-renderer.renderMarkdown).
    const renderMarkdown = (text) => {
      try {
        return safeMarkedParse(text, { marked });
      } catch {
        return escapeHtml(text, { documentImpl });
      }
    };

    // New-entry highlight (after Svelte renders the reactive path).
    function highlightNewEntry(node) {
      node.classList.add('new-entry-highlight');
      setTimeout(() => {
        node.classList.remove('new-entry-highlight');
      }, 1500);
    }
    function highlightNewEntries(newIds) {
      requestAnimationFrame(() => {
        findNewEntryHighlightTargets(newIds, documentImpl).forEach(highlightNewEntry);
      });
    }

    // "seen" set seeded from the model (the DOM may not be flushed yet at startup).
    const LIVE_ENTRY_STATE = {
      seen: new Set((model?.entries || []).map((e) => e.id).filter(Boolean)),
      liveRendered: new Set(),
    };

    // ── Follow mode (auto-scroll + follow-button decisions) ────────────────────
    // The controller registers its own scroll/wheel/touch/keydown listeners and
    // performs the initial scroll-to-bottom; we just dispose it on unmount.
    const followScroll = createFollowScrollController({
      documentImpl,
      windowImpl,
      requestAnimationFrameImpl: requestAnimationFrame,
      setTimeoutImpl: setTimeout,
    });
    cleanups.push(followScroll.dispose);
    const {
      shouldFollow,
      forceFollowToBottom,
      scrollAfterLayout,
      showFollowButton,
      incrementPending,
      isFollowing,
      isAtBottom,
    } = followScroll;

    on(windowImpl, 'pi-chat-message-sent', (event) => {
      followScroll.extendPreviewFollow(30000);
      // Drop any archived preview chunk whose canonical entry already renders
      // in #messages before appending the pending prompt — otherwise the new
      // prompt paints ABOVE the previous turn's stale bottom-pinned copy.
      reconcilePreviewsWithCanonical(CHAT_PREVIEW_STATE, [], {
        running: () => chatRunning,
        allEntries: model ? model.entries : [],
      });
      if (event && event.detail && event.detail.message) {
        renderPendingChat(event.detail.message);
      } else {
        forceFollowToBottom(true);
      }
    });

    function updateStats(entries) {
      return updateStatsDom(entries, { documentImpl });
    }
    function updateTitle(name) {
      setSessionTitle(name);
    }

    const sessId = getSessionIdFromLocation({ locationImpl: windowImpl.location });

    // ── Streaming chat preview ─────────────────────────────────────────────────
    const CHAT_PREVIEW_STATE = {
      chatPreviewEl: null,
      pendingUserEl: null,
      runningSpinnerEl: null,
      previewText: '',
      settledPreviews: [],
    };

    function clearChatPreview() {
      const hasDoneClass =
        CHAT_PREVIEW_STATE.chatPreviewEl &&
        CHAT_PREVIEW_STATE.chatPreviewEl.classList.contains('done');
      const keepAssistant = !!(chatRunning && !hasDoneClass);
      return clearChatPreviewState(CHAT_PREVIEW_STATE, { keepAssistant });
    }
    function clearPendingUser() {
      return clearChatPreviewState(CHAT_PREVIEW_STATE, { keepAssistant: true });
    }
    function finishChatPreview() {
      finishChatPreviewState(CHAT_PREVIEW_STATE);
    }
    function renderChatPreview(payload) {
      return renderChatPreviewState(payload, CHAT_PREVIEW_STATE, {
        documentImpl,
        windowImpl,
        renderMarkdown,
        shouldFollow,
        forceFollowToBottom,
        scrollAfterLayout,
      });
    }
    function renderPendingChat(message) {
      return renderPendingChatState(message, CHAT_PREVIEW_STATE, {
        documentImpl,
        windowImpl,
        renderMarkdown,
        shouldFollow,
        forceFollowToBottom,
        scrollAfterLayout,
      });
    }

    // ── Reload (fetch /api/session → reconcile the model) ──────────────────────
    function triggerReload() {
      return handleSessionReload({
        sessionId: sessId,
        fetchImpl,
        entryState: LIVE_ENTRY_STATE,
        clearChatPreview,
        clearPendingUser,
        previewText: () => CHAT_PREVIEW_STATE.previewText || '',
        // Per-message preview teardown: live-preview TEXT is cleared only by
        // the reload that delivered its canonical entry; archived chunks and
        // textless (thinking-only) previews reconcile against everything the
        // model holds, every reload, so a raced reload can't strand them at
        // the bottom of the transcript.
        onNewAssistantEntries: (newEntries, allEntries) =>
          reconcilePreviewsWithCanonical(CHAT_PREVIEW_STATE, newEntries, {
            running: () => chatRunning,
            allEntries,
          }),
        // Reactive mode: the Svelte model owns #messages, so no DOM patchers.
        updateStats,
        updateTitle,
        isFollowing,
        isAtBottom,
        scrollAfterLayout,
        incrementPending,
        showFollowButton,
        onReloaded: (data) => {
          reconcileEntries(data.entries);
        },
        onNewEntries: highlightNewEntries,
      }).catch((err) => {
        console.error('Live update failed:', err);
      });
    }

    // Driven by the worker-running store (same source as the cancel button), not the SSE
    // streaming lifecycle: the spinner lives as long as the worker runs, surviving preview
    // teardown on reloads.
    let chatRunning = false;

    function handleWorkerDone() {
      finishChatPreview();

      // Real pi flushes canonical entries to disk ~1-2s AFTER the worker goes idle, and the
      // file-watcher reload event is unreliable for brand-new sessions, so actively poll
      // triggerReload until the preview is cleared (which happens when reconcile lands the
      // canonical assistant entry with content).
      let attempt = 0;
      const MAX_ATTEMPTS = 10;
      const RETRY_DELAY = 700;
      const tryReload = () => {
        triggerReload().finally(() => {
          const previewsRemain =
            CHAT_PREVIEW_STATE.chatPreviewEl != null ||
            (CHAT_PREVIEW_STATE.settledPreviews || []).length > 0;
          if (!previewsRemain || ++attempt >= MAX_ATTEMPTS) {
            return;
          }
          setTimeout(tryReload, RETRY_DELAY);
        });
      };
      tryReload();
    }

    const unsubscribeRunning = chatRunningStore.subscribe((running) => {
      const wasRunning = chatRunning;
      chatRunning = running;
      if (running) {
        startRunningSpinner(CHAT_PREVIEW_STATE, { documentImpl, windowImpl });
      } else {
        stopRunningSpinner(CHAT_PREVIEW_STATE);
        if (wasRunning) handleWorkerDone();
      }
    });
    cleanups.push(unsubscribeRunning);
    cleanups.push(() => stopRunningSpinner(CHAT_PREVIEW_STATE));

    const liveConnection = setupSessionLiveConnection({
      documentImpl,
      windowImpl,
      sessionId: sessId,
      onReload: triggerReload,
      onChatPreview: renderChatPreview,
      clearChatPreview,
      onAnnotations: (list) => sessionRuntime.annotations?.setAnnotations(list),
    });
    liveConnection.connect();
    cleanups.push(liveConnection.dispose);

    return () => {
      for (const fn of cleanups) fn();
    };
  });
</script>
