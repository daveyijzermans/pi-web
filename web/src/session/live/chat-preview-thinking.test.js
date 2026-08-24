import { JSDOM } from 'jsdom';
import { describe, it, expect, beforeEach } from 'vitest';
import { renderChatPreviewState } from './chat-preview';

function makeDom() {
  const dom = new JSDOM(`<!DOCTYPE html><html><body>
    <div id="messages"></div>
    <div id="chat-preview-host"></div>
  </body></html>`);
  return {
    documentImpl: dom.window.document,
    windowImpl: dom.window,
  };
}

describe('chat-preview thinking', () => {
  let dom, state, deps;

  beforeEach(() => {
    dom = makeDom();
    state = {
      chatPreviewEl: null,
      pendingUserEl: null,
      runningSpinnerEl: null,
      activePreviewMessage: null,
    };
    deps = {
      documentImpl: dom.documentImpl,
      renderMarkdown: (t) => `<p>${t}</p>`,
    };
  });

  it('renders thinking inline (a .thinking-block/.thinking-text) when present', () => {
    const payload = { content: '', thinking: 'reasoning here', done: false };
    renderChatPreviewState(payload, state, deps);

    const block = state.chatPreviewEl.querySelector('.chat-preview-thinking');
    expect(block).toBeTruthy();
    expect(block.classList.contains('thinking-block')).toBe(true);
    expect(block.style.display).not.toBe('none');
    // Raw text (matching the settled .thinking-text), no chip.
    const textEl = block.querySelector('.thinking-text');
    expect(textEl.textContent).toBe('reasoning here');
    expect(state.chatPreviewEl.querySelector('.chat-preview-thought')).toBeNull();

    const contentEl = state.chatPreviewEl.querySelector('.message-content');
    expect(contentEl.innerHTML).toBe('<p></p>');
  });

  it('renders both thinking and content', () => {
    const payload = { content: 'The answer', thinking: 'reasoning here', done: false };
    renderChatPreviewState(payload, state, deps);

    const block = state.chatPreviewEl.querySelector('.chat-preview-thinking');
    expect(block.style.display).not.toBe('none');
    expect(block.querySelector('.thinking-text').textContent).toBe('reasoning here');

    const contentEl = state.chatPreviewEl.querySelector('.message-content');
    expect(contentEl.innerHTML).toContain('The answer');
  });

  it('hides the thinking block when thinking is empty', () => {
    const payload = { content: 'The answer', thinking: '', done: false };
    renderChatPreviewState(payload, state, deps);

    const block = state.chatPreviewEl.querySelector('.chat-preview-thinking');
    expect(block).toBeTruthy();
    expect(block.style.display).toBe('none');
  });

  it('updates the thinking text across re-renders', () => {
    renderChatPreviewState({ content: '', thinking: 'first', done: false }, state, deps);
    let textEl = state.chatPreviewEl.querySelector('.chat-preview-thinking .thinking-text');
    expect(textEl.textContent).toBe('first');

    renderChatPreviewState({ content: '', thinking: 'first second', done: false }, state, deps);
    textEl = state.chatPreviewEl.querySelector('.chat-preview-thinking .thinking-text');
    expect(textEl.textContent).toBe('first second');
    // Still inline, still no chip.
    expect(state.chatPreviewEl.querySelector('.chat-preview-thought')).toBeNull();
  });
});
