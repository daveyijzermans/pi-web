import { describe, expect, it, vi } from 'vitest';
import { JSDOM } from 'jsdom';
import {
  clearChatPreviewState as clearChatPreview,
  finishChatPreviewState as finishChatPreview,
  reconcilePreviewsWithCanonical,
  renderChatPreviewState as renderChatPreview,
  renderPendingChatState as renderPendingChat,
  startRunningSpinner,
  stopRunningSpinner,
} from './chat-preview.js';

describe('chat preview', () => {
  it('renders, updates, follows, and clears preview', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const state = { chatPreviewEl: null, pendingUserEl: null };
    const forceFollowToBottom = vi.fn();
    const scrollAfterLayout = vi.fn();

    expect(
      renderChatPreview({ content: 'hello', done: false }, state, {
        documentImpl: dom.window.document,
        renderMarkdown: (text) => `<p>${text}</p>`,
        shouldFollow: () => true,
        forceFollowToBottom,
        scrollAfterLayout,
      }),
    ).toBe(true);

    expect(dom.window.document.getElementById('chat-preview-stream')).toBeTruthy();
    expect(state.chatPreviewEl.querySelector('.message-content').innerHTML).toBe('<p>hello</p>');
    // Must include markdown-content so the streaming preview picks up the
    // same heading/hr/list/code styles as the settled assistant message.
    expect(
      state.chatPreviewEl.querySelector('.message-content').classList.contains('markdown-content'),
    ).toBe(true);
    expect(forceFollowToBottom).toHaveBeenCalledWith(false);

    renderChatPreview({ content: 'done', done: true }, state, {
      documentImpl: dom.window.document,
      renderMarkdown: (text) => text,
      shouldFollow: () => false,
    });
    expect(state.chatPreviewEl.classList.contains('done')).toBe(true);
    expect(state.chatPreviewEl.textContent.toLowerCase()).not.toContain('working');

    clearChatPreview(state);
    expect(dom.window.document.getElementById('chat-preview-stream')).toBe(null);
    expect(state.chatPreviewEl).toBe(null);
  });

  it('renders pending user message and working placeholder immediately', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const state = { chatPreviewEl: null, pendingUserEl: null, runningSpinnerEl: null };
    const forceFollowToBottom = vi.fn();

    expect(
      renderPendingChat('hello **pi**', state, {
        documentImpl: dom.window.document,
        renderMarkdown: (text) => `<p>${text}</p>`,
        shouldFollow: () => true,
        forceFollowToBottom,
      }),
    ).toBe(true);

    expect(dom.window.document.getElementById('chat-pending-user')).toBeTruthy();
    expect(dom.window.document.getElementById('chat-pending-user').textContent).toContain(
      'hello **pi**',
    );
    expect(dom.window.document.getElementById('chat-preview-stream')).toBeTruthy();
    expect(forceFollowToBottom).toHaveBeenCalledWith(false);

    // Spinner is created separately via startRunningSpinner, not by renderPendingChat
    startRunningSpinner(state, { documentImpl: dom.window.document });
    expect(dom.window.document.getElementById('chat-running-spinner')).toBeTruthy();
    expect(
      dom.window.document.getElementById('chat-running-spinner').textContent.toLowerCase(),
    ).toContain('working');

    stopRunningSpinner(state);
    expect(dom.window.document.getElementById('chat-running-spinner')).toBe(null);
    expect(state.runningSpinnerEl).toBe(null);

    clearChatPreview(state);
    expect(dom.window.document.getElementById('chat-pending-user')).toBe(null);
    expect(dom.window.document.getElementById('chat-preview-stream')).toBe(null);
  });

  it('can finish a pending preview without removing assistant text', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const state = { chatPreviewEl: null, pendingUserEl: null };

    renderChatPreview({ content: 'final answer', done: false }, state, {
      documentImpl: dom.window.document,
      renderMarkdown: (text) => text,
    });

    expect(finishChatPreview(state)).toBe(true);
    expect(dom.window.document.getElementById('chat-preview-stream').textContent).toContain(
      'final answer',
    );
    expect(
      dom.window.document.getElementById('chat-preview-stream').textContent.toLowerCase(),
    ).not.toContain('working');
    expect(state.chatPreviewEl.classList.contains('done')).toBe(true);
  });

  it('running spinner element is independent of the preview', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const state = { chatPreviewEl: null, pendingUserEl: null, runningSpinnerEl: null };

    renderChatPreview({ content: 'hi', done: false }, state, {
      documentImpl: dom.window.document,
      renderMarkdown: (text) => text,
    });

    startRunningSpinner(state, { documentImpl: dom.window.document });
    expect(dom.window.document.getElementById('chat-running-spinner')).toBeTruthy();

    // Clearing the preview stream does NOT remove the spinner
    clearChatPreview(state);
    expect(dom.window.document.getElementById('chat-preview-stream')).toBe(null);
    expect(dom.window.document.getElementById('chat-running-spinner')).toBeTruthy();

    // Stopping the spinner removes it
    stopRunningSpinner(state);
    expect(dom.window.document.getElementById('chat-running-spinner')).toBe(null);
    expect(state.runningSpinnerEl).toBe(null);
  });

  it('clears pending user but keeps assistant preview when keepAssistant option is true', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const state = { chatPreviewEl: null, pendingUserEl: null };

    renderPendingChat('hello pi', state, {
      documentImpl: dom.window.document,
      renderMarkdown: (text) => text,
    });

    expect(dom.window.document.getElementById('chat-pending-user')).toBeTruthy();
    expect(dom.window.document.getElementById('chat-preview-stream')).toBeTruthy();

    clearChatPreview(state, { keepAssistant: true });
    // pending user element should be removed from the DOM and cleared
    expect(dom.window.document.getElementById('chat-pending-user')).toBeNull();
    expect(state.pendingUserEl).toBeNull();
    // assistant preview should still be in the DOM and NOT cleared
    expect(dom.window.document.getElementById('chat-preview-stream')).toBeTruthy();
    expect(state.chatPreviewEl).toBeTruthy();

    // And clearing it without keepAssistant removes it
    clearChatPreview(state, { keepAssistant: false });
    expect(dom.window.document.getElementById('chat-preview-stream')).toBeNull();
    expect(state.chatPreviewEl).toBeNull();
  });

  // A second message must NOT overwrite the finished first message's text
  // while its canonical entry is still in flight (slow reload fetch): the
  // finished preview is archived and both stay on the page.
  it('archives a finished preview when the next message starts streaming', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const doc = dom.window.document;
    const state = {
      chatPreviewEl: null,
      pendingUserEl: null,
      previewText: '',
      settledPreviews: [],
    };
    const opts = { documentImpl: doc, renderMarkdown: (t) => t };

    renderChatPreview({ content: 'Message one.', done: false }, state, opts);
    renderChatPreview({ content: 'Message one.', done: true }, state, opts);
    // Next message begins — archive the finished one, stream into a fresh el.
    renderChatPreview({ content: 'Message two.', done: false }, state, opts);

    expect(state.settledPreviews).toHaveLength(1);
    expect(state.settledPreviews[0].text).toBe('Message one.');
    expect(doc.getElementById('chat-preview-host').textContent).toContain('Message one.');
    expect(doc.getElementById('chat-preview-host').textContent).toContain('Message two.');
    expect(state.previewText).toBe('Message two.');
  });

  it('reconcile removes only the preview chunks whose canonical entry arrived', () => {
    const dom = new JSDOM(
      '<body><div id="messages"></div><div id="chat-preview-host"></div></body>',
    );
    const doc = dom.window.document;
    const state = {
      chatPreviewEl: null,
      pendingUserEl: null,
      previewText: '',
      settledPreviews: [],
    };
    const opts = { documentImpl: doc, renderMarkdown: (t) => t };

    // Two finished messages archived; a third is mid-stream (not done).
    renderChatPreview({ content: 'One.', done: true }, state, opts);
    renderChatPreview({ content: 'Two.', done: true }, state, opts);
    renderChatPreview({ content: 'Three streaming', done: false }, state, opts);
    expect(state.settledPreviews).toHaveLength(2);

    // Only message One's canonical entry lands.
    reconcilePreviewsWithCanonical(
      state,
      [{ message: { role: 'assistant', content: [{ type: 'text', text: 'One.' }] } }],
      { running: () => true },
    );
    expect(state.settledPreviews.map((p) => p.text)).toEqual(['Two.']);
    // The live mid-stream preview is untouched (its text isn't canonical yet).
    expect(state.chatPreviewEl).toBeTruthy();
    expect(state.previewText).toBe('Three streaming');

    // Two's entry lands, and the live one finishes + becomes canonical.
    state.chatPreviewEl.classList.add('done');
    reconcilePreviewsWithCanonical(
      state,
      [
        { message: { role: 'assistant', content: [{ type: 'text', text: 'Two.' }] } },
        {
          message: { role: 'assistant', content: [{ type: 'text', text: 'Three streaming done' }] },
        },
      ],
      { running: () => false },
    );
    expect(state.settledPreviews).toHaveLength(0);
    expect(state.chatPreviewEl).toBeNull();
    expect(doc.getElementById('chat-preview-host').textContent).toBe('');
  });
});
