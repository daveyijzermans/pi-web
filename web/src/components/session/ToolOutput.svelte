<script module>
  // Click-to-toggle expandable tool output. Toggling `.expanded` is also driven
  // by the header "tool outputs" control and its per-user default setting via
  // applyToggleStateToNode(), which targets `.tool-output.expandable`.
  // Plain click-to-expand; does nothing if text is selected.
  export function toggleExpanded(e) {
    if (window.getSelection && window.getSelection().toString()) return;
    e.currentTarget.classList.toggle('expanded');
  }
</script>

<script>
  import { splitOutputLines } from '../../session/render/entry-format.js';
  import { t } from '../../shared/i18n.js';

  let { text = '', lang = null } = $props();

  // Long outputs collapse to a short preview by default; the "tool outputs"
  // toggle (and its Session Display default) expands them by adding `.expanded`.
  // Only collapse when enough is hidden to be worth a click.
  const PREVIEW_LINES = 12;
  const COLLAPSE_MIN = 20;

  const split = $derived(splitOutputLines(text));
  const collapsible = $derived(split.lines.length > COLLAPSE_MIN);
  const previewLines = $derived(split.lines.slice(0, PREVIEW_LINES));
  const hiddenCount = $derived(split.lines.length - PREVIEW_LINES);
</script>

{#if collapsible}
  <!-- eslint-disable-next-line svelte/no-static-element-interactions -->
  <div class="tool-output expandable" onclick={toggleExpanded} role="presentation">
    <div class="output-preview">
      {#if lang}
        <div class="code-with-gutter">
          <div class="code-gutter">
            {#each previewLines as _line, i (i)}<span>{i + 1}</span>{/each}
          </div>
          <pre><code class="hljs" data-highlight-pending data-lang={lang}>{previewLines.join(
              '\n',
            )}</code></pre>
        </div>
      {:else}
        {#each previewLines as line, i (i)}<div>{line}</div>{/each}
      {/if}
      <div class="output-expand-hint">{t('session.toolOutputMore', { n: hiddenCount })}</div>
    </div>
    <div class="output-full">
      {#if lang}
        <div class="code-with-gutter">
          <div class="code-gutter">
            {#each split.lines as _line, i (i)}<span>{i + 1}</span>{/each}
          </div>
          <pre><code class="hljs" data-highlight-pending data-lang={lang}>{split.lines.join(
              '\n',
            )}</code></pre>
        </div>
      {:else}
        {#each split.lines as line, i (i)}<div>{line}</div>{/each}
      {/if}
    </div>
  </div>
{:else if lang}
  <div class="tool-output">
    <div class="code-with-gutter">
      <div class="code-gutter">
        {#each split.lines as _line, i (i)}<span>{i + 1}</span>{/each}
      </div>
      <pre><code class="hljs" data-highlight-pending data-lang={lang}>{split.lines.join('\n')}</code
        ></pre>
    </div>
  </div>
{:else}
  <div class="tool-output">
    {#each split.lines as line, lineIndex (lineIndex)}<div>{line}</div>{/each}
  </div>
{/if}
