export function getSpinnerConfig(windowImpl = typeof window !== 'undefined' ? window : null) {
  let style = 'runcat';
  try {
    if (windowImpl && windowImpl.localStorage) {
      const saved = windowImpl.localStorage.getItem('pi-sessions:spinner-style');
      if (saved === 'braille') {
        style = 'braille';
      }
    }
  } catch (_) {}

  if (style === 'braille') {
    return {
      frames: ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'],
      fontFamily: 'monospace',
      interval: 80,
      width: '12px',
    };
  } else {
    // runcat frames mapping to unicode private use area characters in runcat.ttf font
    return {
      frames: ['', '', '', '', ''],
      fontFamily: "'runcat', monospace",
      interval: 100,
      width: '18px',
    };
  }
}

export function clearChatPreviewState(state, { keepAssistant = false } = {}) {
  if (state.pendingUserEl && state.pendingUserEl.parentNode) {
    state.pendingUserEl.parentNode.removeChild(state.pendingUserEl);
    state.pendingUserEl = null;
  }
  if (!keepAssistant) {
    if (state.chatPreviewEl && state.chatPreviewEl.parentNode) {
      state.chatPreviewEl.parentNode.removeChild(state.chatPreviewEl);
    }
    state.chatPreviewEl = null;
    state.previewText = '';
  }
}

export function finishChatPreviewState(state) {
  if (!state?.chatPreviewEl) return false;
  state.chatPreviewEl.classList.remove('chat-preview-waiting');
  state.chatPreviewEl.classList.add('done');
  return true;
}
// Test placeholder for TestSessionViteSourceShowsAnimatedWorkingPreviewLabel: working<span class="working-dots"

const CREATIVE_MESSAGES = [
  'Working...',
  'Thinking...',
  'Analyzing codebase...',
  'Synthesizing answer...',
  'Consulting model...',
  'Formulating solution...',
  'Checking files...',
  'Drafting response...',
];

export function startWorkingAnimation(
  state,
  {
    setIntervalImpl = setInterval,
    windowImpl = typeof window !== 'undefined' ? window : null,
  } = {},
) {
  stopWorkingAnimation(state);

  const config = getSpinnerConfig(windowImpl);
  let frameIdx = 0;
  let msgIdx = 0;
  let lastMsgChange = Date.now();
  state.activePreviewMessage = null;

  // Sync initial spinner properties if spinner element is already present
  if (state.runningSpinnerEl) {
    const spinnerEl = state.runningSpinnerEl.querySelector('.preview-spinner');
    if (spinnerEl) {
      spinnerEl.style.fontFamily = config.fontFamily;
      spinnerEl.style.width = config.width;
      spinnerEl.textContent = config.frames[0];
    }
  }

  state.spinnerInterval = setIntervalImpl(() => {
    if (!state.runningSpinnerEl) {
      stopWorkingAnimation(state);
      return;
    }

    const spinnerEl = state.runningSpinnerEl.querySelector('.preview-spinner');
    if (spinnerEl) {
      if (spinnerEl.style.fontFamily !== config.fontFamily) {
        spinnerEl.style.fontFamily = config.fontFamily;
        spinnerEl.style.width = config.width;
      }
      frameIdx = (frameIdx + 1) % config.frames.length;
      spinnerEl.textContent = config.frames[frameIdx];
    }

    if (!state.activePreviewMessage && Date.now() - lastMsgChange >= 2000) {
      const textEl = state.runningSpinnerEl.querySelector('.preview-text');
      if (textEl) {
        msgIdx = (msgIdx + 1) % CREATIVE_MESSAGES.length;
        textEl.textContent = CREATIVE_MESSAGES[msgIdx];
        lastMsgChange = Date.now();
      }
    }
  }, config.interval);
}

export function stopWorkingAnimation(state, { clearIntervalImpl = clearInterval } = {}) {
  if (state && state.spinnerInterval) {
    clearIntervalImpl(state.spinnerInterval);
    state.spinnerInterval = null;
  }
  if (state) {
    state.activePreviewMessage = null;
  }
}

export function startRunningSpinner(
  state,
  {
    documentImpl = document,
    windowImpl = typeof window !== 'undefined' ? window : null,
    setIntervalImpl = setInterval,
  } = {},
) {
  if (!state.runningSpinnerEl) {
    const container =
      documentImpl.getElementById('chat-preview-host') ||
      documentImpl.getElementById('content') ||
      documentImpl.body;
    const config = getSpinnerConfig(windowImpl);
    const el = documentImpl.createElement('div');
    el.id = 'chat-running-spinner';
    el.className = 'chat-running-spinner';
    const spinner = documentImpl.createElement('span');
    spinner.className = 'preview-spinner';
    spinner.style.color = 'var(--accent)';
    spinner.style.marginRight = '6px';
    spinner.style.fontFamily = config.fontFamily;
    spinner.style.display = 'inline-block';
    spinner.style.width = config.width;
    spinner.style.textAlign = 'center';
    spinner.textContent = config.frames[0];
    const text = documentImpl.createElement('span');
    text.className = 'preview-text';
    text.style.color = 'var(--muted)';
    text.textContent = 'Working...';
    el.append(spinner, text);
    container.appendChild(el);
    state.runningSpinnerEl = el;
  }
  startWorkingAnimation(state, { setIntervalImpl, windowImpl });
}

export function stopRunningSpinner(state, { clearIntervalImpl = clearInterval } = {}) {
  stopWorkingAnimation(state, { clearIntervalImpl });
  if (state.runningSpinnerEl && state.runningSpinnerEl.parentNode) {
    state.runningSpinnerEl.parentNode.removeChild(state.runningSpinnerEl);
  }
  state.runningSpinnerEl = null;
}

function getActiveMessage(content) {
  if (!content) return null;

  // Check if there is an active/open thinking block
  const openThoughtIdx = content.lastIndexOf('<thought>');
  const closeThoughtIdx = content.lastIndexOf('</thought>');
  if (openThoughtIdx !== -1 && openThoughtIdx > closeThoughtIdx) {
    return 'Thinking...';
  }

  // Check if there is an active/open code block
  const codeBlockCount = (content.match(/```/g) || []).length;
  if (codeBlockCount % 2 === 1) {
    return 'Writing code...';
  }

  return 'Generating response...';
}

function setMarkdownContent(el, html) {
  // `renderMarkdown` returns sanitized markdown HTML (or escaped fallback). This
  // is content rendering, not structural view construction; the surrounding
  // preview DOM is built with elements so the helper stays narrowly scoped.
  if (el) el.innerHTML = html;
}

function createMarkdownBlock(documentImpl, className) {
  const el = documentImpl.createElement('div');
  el.className = className;
  return el;
}

function createAssistantPreview(documentImpl, { waiting = false, windowImpl = null } = {}) {
  void windowImpl;
  const el = documentImpl.createElement('div');
  el.id = 'chat-preview-stream';
  el.className = 'assistant-message chat-preview-stream' + (waiting ? ' chat-preview-waiting' : '');
  // Streaming reasoning renders inline as a .thinking-block/.thinking-text —
  // matching the settled message view — not a collapsed "Thought" chip.
  const thinkingBlock = documentImpl.createElement('div');
  thinkingBlock.className = 'thinking-block chat-preview-thinking';
  thinkingBlock.style.display = 'none';
  const thinkingTextEl = documentImpl.createElement('div');
  thinkingTextEl.className = 'thinking-text';
  thinkingBlock.append(thinkingTextEl);
  el.append(thinkingBlock);
  el.append(createMarkdownBlock(documentImpl, 'message-content assistant-text markdown-content'));
  return el;
}

export function renderPendingChatState(
  message,
  state,
  {
    documentImpl = document,
    windowImpl = typeof window !== 'undefined' ? window : null,
    renderMarkdown,
    shouldFollow = () => false,
    forceFollowToBottom = () => {},
    scrollAfterLayout = () => {},
    _setIntervalImpl = setInterval,
  } = {},
) {
  void _setIntervalImpl;
  const text = String(message || '').trim();
  if (!text) return false;
  // Render in the dedicated preview host outside #messages so it survives
  // Svelte re-renders of <SessionContent> inside #messages.
  const container =
    documentImpl.getElementById('chat-preview-host') ||
    documentImpl.getElementById('messages') ||
    documentImpl.getElementById('content') ||
    documentImpl.body;
  clearChatPreviewState(state);

  state.pendingUserEl = documentImpl.createElement('div');
  state.pendingUserEl.id = 'chat-pending-user';
  state.pendingUserEl.className = 'user-message chat-pending-user';
  const userContent = createMarkdownBlock(documentImpl, 'markdown-content');
  setMarkdownContent(userContent, renderMarkdown(text));
  state.pendingUserEl.appendChild(userContent);
  container.appendChild(state.pendingUserEl);

  state.chatPreviewEl = createAssistantPreview(documentImpl, { waiting: true, windowImpl });
  state.previewText = '';
  container.appendChild(state.chatPreviewEl);

  if (shouldFollow()) {
    forceFollowToBottom(false);
    scrollAfterLayout(false, state.chatPreviewEl);
  }
  return true;
}

// A finished (done) preview whose canonical entry hasn't rendered yet is the
// ONLY place its text exists — pi flushes a message to disk only after its
// tool-call args finish, and the reload fetch can lag seconds behind on a
// slow link. When the next message starts streaming, don't overwrite that
// element: archive it in place and stream into a fresh one. Archived chunks
// are removed by reconcilePreviewsWithCanonical when their canonical entry
// arrives.
function archiveDonePreview(state) {
  const el = state.chatPreviewEl;
  if (!el || !el.classList.contains('done')) return;
  el.removeAttribute('id');
  if (!state.settledPreviews) state.settledPreviews = [];
  state.settledPreviews.push({ el, text: String(state.previewText || '') });
  state.chatPreviewEl = null;
  state.previewText = '';
}

// Remove archived preview chunks (and a finished live preview) whose text has
// arrived as canonical assistant entries. Called from the session reload with
// the NEW assistant entries of that reload. A live preview without text
// (waiting / thinking-only) follows the legacy rule: any new assistant entry
// clears it once it is done (or the worker is no longer running).
export function reconcilePreviewsWithCanonical(state, entries, { running = () => false } = {}) {
  const texts = [];
  for (const entry of entries || []) {
    const content = entry?.message?.content;
    if (typeof content === 'string') {
      texts.push(content);
    } else if (Array.isArray(content)) {
      for (const block of content) {
        if (block?.type === 'text' && block.text) texts.push(String(block.text));
      }
    }
  }
  if (!texts.length) return;
  const canonical = (text) => !!text && texts.some((t) => t.includes(text));

  state.settledPreviews = (state.settledPreviews || []).filter(({ el, text }) => {
    if (canonical(String(text || '').trim())) {
      if (el && el.parentNode) el.parentNode.removeChild(el);
      return false;
    }
    return true;
  });

  const el = state.chatPreviewEl;
  if (!el) return;
  const done = el.classList.contains('done');
  const shown = String(state.previewText || '').trim();
  const clearable = shown ? canonical(shown) : done || !running();
  if (clearable) {
    if (el.parentNode) el.parentNode.removeChild(el);
    state.chatPreviewEl = null;
    state.previewText = '';
  }
}

export function renderChatPreviewState(
  payload,
  state,
  {
    documentImpl = document,
    windowImpl = typeof window !== 'undefined' ? window : null,
    renderMarkdown,
    shouldFollow = () => false,
    forceFollowToBottom = () => {},
    scrollAfterLayout = () => {},
    _setIntervalImpl = setInterval,
  } = {},
) {
  void _setIntervalImpl;
  if (!payload || typeof payload.content !== 'string') return false;
  // Render in the dedicated preview host outside #messages.
  const container =
    documentImpl.getElementById('chat-preview-host') ||
    documentImpl.getElementById('messages') ||
    documentImpl.getElementById('content') ||
    documentImpl.body;
  // A done preview belongs to a finished message; this payload starts the
  // next one. Preserve the finished text until its canonical entry lands.
  archiveDonePreview(state);
  if (!state.chatPreviewEl) {
    state.chatPreviewEl = createAssistantPreview(documentImpl, { windowImpl });
    container.appendChild(state.chatPreviewEl);
  }

  const thinkingText = typeof payload.thinking === 'string' ? payload.thinking : '';
  const activeMsg =
    getActiveMessage(payload.content) || (thinkingText.trim() ? 'Thinking...' : null);
  if (activeMsg) {
    state.activePreviewMessage = activeMsg;
    const textEl = state.runningSpinnerEl && state.runningSpinnerEl.querySelector('.preview-text');
    if (textEl) textEl.textContent = activeMsg;
  }

  state.chatPreviewEl.classList.remove('chat-preview-waiting');
  const thinkingBlock = state.chatPreviewEl.querySelector('.chat-preview-thinking');
  const thinkingTextEl = state.chatPreviewEl.querySelector('.chat-preview-thinking .thinking-text');
  if (thinkingText.trim()) {
    if (thinkingTextEl) thinkingTextEl.textContent = thinkingText;
    if (thinkingBlock) thinkingBlock.style.display = '';
  } else {
    if (thinkingTextEl) thinkingTextEl.textContent = '';
    if (thinkingBlock) thinkingBlock.style.display = 'none';
  }
  const content = state.chatPreviewEl.querySelector('.message-content');
  setMarkdownContent(content, renderMarkdown(payload.content));
  // Raw source of what the preview shows — handleSessionReload compares it
  // against incoming canonical entries to decide whether the preview may be
  // cleared (only the previewed message's own entry may clear it).
  state.previewText = payload.content;
  if (payload.done) finishChatPreviewState(state);
  else state.chatPreviewEl.classList.remove('done');
  if (shouldFollow()) {
    forceFollowToBottom(false);
    scrollAfterLayout(false, state.chatPreviewEl);
  }
  return true;
}
