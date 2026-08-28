export function getSessionIdFromLocation({ locationImpl = location } = {}) {
  return locationImpl.search.split('id=')[1]?.split('&')[0] || '';
}

export function createSessionEventSource(sessionId, { EventSourceImpl = EventSource } = {}) {
  return new EventSourceImpl('/events?id=' + encodeURIComponent(sessionId));
}

export async function handleSessionReload({
  sessionId,
  fetchImpl = fetch,
  entryState,
  clearChatPreview = () => {},
  clearPendingUser = () => {},
  previewText = () => '',
  // When provided, receives this reload's NEW canonical assistant entries
  // (with content) and owns preview teardown — replacing the legacy
  // clearChatPreview call. The live path uses it to reconcile per-message
  // preview chunks against the canonical entries that actually arrived.
  onNewAssistantEntries = null,
  appendEntry,
  upsertEntry,
  refreshEntriesAffectedByToolResult,
  updateStats = () => {},
  updateTitle = () => {},
  isFollowing = () => false,
  scrollAfterLayout = () => {},
  incrementPending = () => {},
  showFollowButton = () => {},
  onReloaded = () => {},
  onNewEntries = null,
  _requestAnimationFrame = requestAnimationFrame,
} = {}) {
  const response = await fetchImpl('/api/session?id=' + encodeURIComponent(sessionId));
  const data = await response.json();
  const entries = data.entries || [];
  onReloaded({ ...data, entries });
  if (typeof data.name === 'string' && data.name.trim()) {
    updateTitle(data.name);
  }
  let newCount = 0;

  // Two modes:
  //  • Imperative (appendEntry provided): patch #messages DOM directly. Kept
  //    for isolated helper tests and non-Svelte callers.
  //  • Reactive (no appendEntry): the Svelte <SessionContent> owns #messages and
  //    re-renders from the model that onReloaded just updated, so here we only
  //    track which ids are brand-new (for follow/scroll/highlight decisions).
  const reactive = typeof appendEntry !== 'function';
  const newIds = [];

  entries.forEach((entry) => {
    if (!entry.id) return;
    if (reactive) {
      if (!entryState.seen.has(entry.id)) {
        entryState.seen.add(entry.id);
        newCount++;
        newIds.push(entry.id);
      }
      return;
    }
    if (!entryState.seen.has(entry.id)) {
      if (appendEntry(entry, entries)) newCount++;
      if (entry.message && entry.message.role === 'toolResult') {
        refreshEntriesAffectedByToolResult(entry, entries);
      }
    } else if (entryState.liveRendered.has(entry.id)) {
      upsertEntry(entry, entries);
      if (entry.message && entry.message.role === 'toolResult') {
        refreshEntriesAffectedByToolResult(entry, entries);
      }
    } else if (entry.message && entry.message.role === 'toolResult') {
      refreshEntriesAffectedByToolResult(entry, entries);
    }
  });

  // Detect whether any new entries are metadata types (archive, session_info).
  // When a reload brings both metadata and content entries, we defer the preview
  // clear so Svelte can render the new entries first — otherwise the user sees
  // a flash of empty content.
  const hasNewMetadata = newIds.some((id) =>
    ['archive', 'session_info'].includes(entries.find((e) => e.id === id)?.type),
  );

  // Clear the optimistic preview only when a canonical assistant entry with
  // content arrives. A file-watcher reload that only brings the user message
  // (or a label) should keep the streaming preview alive. A reload that brings
  // the assistant's reply means the canonical content is on disk and the
  // preview is no longer needed.
  //
  // "The assistant's reply" must be THE PREVIEWED message, not just any new
  // assistant entry: pi flushes each message only when its tool-call args
  // finish streaming, so text the preview is showing may not be on disk yet
  // while an OLDER message's entry lands late (watcher debounce + slow fetch).
  // Clearing on that entry would vanish the streamed text until its own
  // message flushes. When the preview has visible text, require a new
  // canonical entry that actually contains it; a preview without text
  // (waiting / thinking-only) keeps the legacy any-assistant-entry behaviour.
  const shownText = String(previewText() || '').trim();
  const newAssistantEntries = newIds
    .map((id) => entries.find((e) => e.id === id))
    .filter((e) => e?.message?.role === 'assistant' && e.message.content?.length > 0);
  const canonicalHasShownText = (entry) => {
    if (!shownText) return true;
    const content = entry.message.content;
    if (typeof content === 'string') return content.includes(shownText);
    return (
      Array.isArray(content) &&
      content.some(
        (block) => block?.type === 'text' && String(block.text || '').includes(shownText),
      )
    );
  };
  if (typeof onNewAssistantEntries === 'function') {
    if (newAssistantEntries.length) {
      if (hasNewMetadata) {
        _requestAnimationFrame(() => onNewAssistantEntries(newAssistantEntries));
      } else {
        onNewAssistantEntries(newAssistantEntries);
      }
    }
  } else if (newAssistantEntries.some(canonicalHasShownText)) {
    if (hasNewMetadata) {
      _requestAnimationFrame(() => clearChatPreview());
    } else {
      clearChatPreview();
    }
  }

  const hasNewUserMessage = newIds.some(
    (id) => entries.find((e) => e.id === id)?.message?.role === 'user',
  );
  if (hasNewUserMessage) {
    if (hasNewMetadata) {
      _requestAnimationFrame(() => clearPendingUser());
    } else {
      clearPendingUser();
    }
  }

  if (newCount > 0) {
    updateStats(entries);
    if (isFollowing()) {
      scrollAfterLayout(true);
    } else {
      incrementPending(newCount);
      showFollowButton();
    }
  }

  // Reactive mode: once Svelte has rendered the new entries, flag them so the
  // caller can apply the new-entry highlight.
  if (newIds.length && typeof onNewEntries === 'function') {
    onNewEntries(newIds);
  }

  return { entries, newCount };
}

export function wireSessionEvents({
  eventSource,
  onReload,
  onChatPreview,
  _clearChatPreview = () => {},
  onAnnotations = null,
  onError = () => {},
  windowImpl = typeof window !== 'undefined' ? window : null,
  CustomEventImpl = typeof CustomEvent !== 'undefined' ? CustomEvent : null,
} = {}) {
  eventSource.onmessage = (event) => {
    if (event.data !== 'reload') return;
    onReload(event);
    // Broadcast so other modules (e.g. chat composer status) can react
    // immediately instead of waiting for their next poll tick.
    if (windowImpl && CustomEventImpl) {
      try {
        windowImpl.dispatchEvent(new CustomEventImpl('pi-session-reload'));
      } catch (_) {}
    }
  };
  eventSource.addEventListener('chat-preview', (event) => {
    try {
      const payload = JSON.parse(event.data);
      onChatPreview(payload);
      // Don't clear the preview or trigger a reload here — the real pi worker
      // sends 'done' BEFORE flushing entries to disk. Clearing the preview now
      // would lose the streaming content. The file-watcher reload fires after
      // the disk write, and handleSessionReload() clears the preview only when
      // a canonical assistant entry with content arrives.
    } catch (error) {
      onError(error);
    }
  });
  if (onAnnotations) {
    eventSource.addEventListener('annotations', (event) => {
      try {
        const data = JSON.parse(event.data);
        onAnnotations(Array.isArray(data.annotations) ? data.annotations : []);
      } catch (error) {
        onError(error);
      }
    });
  }
  // A manual compaction failed server-side. Compaction is fire-and-forget, so
  // the only way the footer/composer learns of a failure is this event; relay
  // it as a window event carrying the error message so listeners can toast it.
  eventSource.addEventListener('compact-error', (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (windowImpl && CustomEventImpl) {
        windowImpl.dispatchEvent(new CustomEventImpl('pi-compact-error', { detail: payload }));
      }
    } catch (error) {
      onError(error);
    }
  });
  eventSource.onerror = onError;
  return eventSource;
}
