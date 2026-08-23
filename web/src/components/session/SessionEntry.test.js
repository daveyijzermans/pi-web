import { describe, expect, it, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import SessionEntry from './SessionEntry.svelte';

afterEach(cleanup);

function model(entries = []) {
  return { entries, renderedTools: null };
}

describe('SessionEntry', () => {
  it('renders a user message with its text under an entry anchor', () => {
    const entry = { id: 'u', type: 'message', message: { role: 'user', content: 'hello' } };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });
    const node = container.querySelector('#entry-u');
    expect(node).not.toBeNull();
    expect(node).toHaveClass('user-message');
    expect(node.textContent).toContain('hello');
  });

  it('renders an assistant message wrapped in assistant-group', () => {
    const entry = {
      id: 'a',
      type: 'message',
      message: { role: 'assistant', content: [{ type: 'text', text: 'hi' }] },
    };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });
    const node = container.querySelector('#entry-a');
    expect(node).toHaveClass('assistant-message');
    expect(node.textContent).toContain('hi');

    const group = container.querySelector('.assistant-group');
    expect(group).not.toBeNull();
    expect(group.getAttribute('data-entry-ids')).toBe('a');

    const textEl = group.querySelector('.assistant-text');
    expect(textEl).not.toBeNull();
    expect(textEl.getAttribute('data-entry-ids')).toBeNull();
  });

  it('renders thinking + tool calls inline inside assistant-group', () => {
    const entry = {
      id: 'a',
      type: 'message',
      message: {
        role: 'assistant',
        content: [
          { type: 'thinking', thinking: 'thinking...', sourceId: 'a' },
          { type: 'toolCall', id: 'tc1', name: 'read', arguments: {}, sourceId: 'a' },
        ],
      },
    };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });

    const group = container.querySelector('.assistant-group');
    expect(group).not.toBeNull();
    expect(group.getAttribute('data-entry-ids')).toBe('a');

    // Inline rendering: a thinking block and an inline tool execution, no chip button.
    expect(group.querySelector('.thinking-text')?.textContent).toContain('thinking...');
    expect(group.querySelector('.tool-execution')).not.toBeNull();
    expect(group.querySelector('.tool-chip')).toBeNull();
  });

  it('renders nothing for tool-result entries', () => {
    const entry = {
      id: 'r',
      type: 'message',
      message: { role: 'toolResult', toolCallId: 'c', content: [] },
    };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });
    expect(container.querySelector('#entry-r')).toBeNull();
  });

  it('renders a model change but omits implicit ones', () => {
    const entry = { id: 'm', type: 'model_change', provider: 'p', modelId: 'x' };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });
    expect(container.querySelector('#entry-m.model-change')?.textContent).toContain('p/x');

    cleanup();
    const implicit = {
      id: 'm2',
      type: 'model_change',
      provider: 'p',
      modelId: 'x',
      implicit: true,
    };
    const { container: c2 } = render(SessionEntry, {
      props: { entry: implicit, model: model([implicit]) },
    });
    expect(c2.querySelector('#entry-m2')).toBeNull();
  });

  it('renders attachment chips for user messages with attachment refs', () => {
    const entry = {
      id: 'u',
      type: 'message',
      message: {
        role: 'user',
        content: 'Please review\n[Attached file: /home/user/report.csv (text/csv, 1024 bytes)]',
      },
    };
    const { container } = render(SessionEntry, { props: { entry, model: model([entry]) } });
    const node = container.querySelector('#entry-u');
    expect(node).not.toBeNull();

    const chip = node.querySelector('.message-attachment');
    expect(chip).not.toBeNull();
    expect(chip.getAttribute('title')).toBe('/home/user/report.csv');
    expect(chip.textContent).toContain('report.csv');

    const markdownContent = node.querySelector('.markdown-content');
    expect(markdownContent?.textContent).not.toContain('[Attached file:');
  });
});
