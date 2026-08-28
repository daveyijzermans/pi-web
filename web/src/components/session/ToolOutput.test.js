import { describe, expect, it, afterEach } from 'vitest';
import { render, cleanup, fireEvent } from '@testing-library/svelte';
import ToolOutput from './ToolOutput.svelte';
import { applyToggleStateToNode } from '../../session/ui/toggle-state.js';

afterEach(() => cleanup());

const lines = (n) => Array.from({ length: n }, (_, i) => `line ${i + 1}`).join('\n');

describe('ToolOutput', () => {
  it('renders a short output as a plain, non-collapsible block', () => {
    const { container } = render(ToolOutput, { props: { text: lines(5) } });
    const out = container.querySelector('.tool-output');
    expect(out).not.toBeNull();
    expect(out.classList.contains('expandable')).toBe(false);
    expect(container.querySelector('.output-preview')).toBeNull();
  });

  it('renders a long output as collapsible with a preview, full view, and hint', () => {
    const { container } = render(ToolOutput, { props: { text: lines(40) } });
    const out = container.querySelector('.tool-output.expandable');
    expect(out).not.toBeNull();
    expect(container.querySelector('.output-preview')).not.toBeNull();
    expect(container.querySelector('.output-full')).not.toBeNull();
    // Preview shows the first 12 lines; full shows all 40.
    expect(
      container.querySelectorAll('.output-preview > div:not(.output-expand-hint)').length,
    ).toBe(12);
    expect(container.querySelectorAll('.output-full > div').length).toBe(40);
    expect(container.querySelector('.output-expand-hint').textContent).toBe('Show 28 more lines');
  });

  it('renders a huge output as a bounded preview with the full text deferred', async () => {
    const huge = lines(20000); // >100KB — triggers the huge branch
    const { container, getByRole } = render(ToolOutput, { props: { text: huge } });
    expect(container.querySelector('.tool-output-huge')).not.toBeNull();
    // The full multi-line block must NOT be in the DOM until asked for.
    expect(container.querySelector('.output-huge-full')).toBeNull();
    // Preview shows only the first 12 lines.
    expect(container.querySelectorAll('.output-preview > div').length).toBe(12);
    // Expanding renders the full text as a single plain <pre>.
    await fireEvent.click(getByRole('button', { name: /Show full output/ }));
    const full = container.querySelector('.output-huge-full');
    expect(full).not.toBeNull();
    expect(full.textContent).toBe(huge);
  });

  it('is expanded by the tool-outputs toggle default (applyToggleStateToNode)', () => {
    const { container } = render(ToolOutput, { props: { text: lines(40) } });
    const out = container.querySelector('.tool-output.expandable');
    expect(out.classList.contains('expanded')).toBe(false);

    // Setting on -> the block loads expanded.
    applyToggleStateToNode(container, { toolOutputsExpanded: true });
    expect(out.classList.contains('expanded')).toBe(true);

    // Setting off -> collapsed again.
    applyToggleStateToNode(container, { toolOutputsExpanded: false });
    expect(out.classList.contains('expanded')).toBe(false);
  });

  it('collapses long code output too (with a language)', () => {
    const { container } = render(ToolOutput, { props: { text: lines(40), lang: 'bash' } });
    const out = container.querySelector('.tool-output.expandable');
    expect(out).not.toBeNull();
    expect(container.querySelector('.output-preview .code-with-gutter')).not.toBeNull();
    expect(container.querySelector('.output-full .code-with-gutter')).not.toBeNull();
  });
});
