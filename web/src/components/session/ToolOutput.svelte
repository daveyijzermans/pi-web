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

  // Very large outputs (e.g. a multi-MB file read) are the dominant page-render
  // cost: the normal collapsible branch renders the FULL block into the DOM and
  // syntax-highlights it even while visually collapsed. For huge outputs we
  // render only a small preview and defer the full text until the user asks for
  // it — and then as a single plain <pre> (no per-line DOM, no highlighting),
  // which the browser handles far better than megabytes of highlighted nodes.
  const HUGE_CHARS = 100_000;
  const huge = $derived((text?.length || 0) > HUGE_CHARS);
  let showFull = $state(false);

  // Preview from a bounded slice so a huge output is never split in full just
  // to show 12 lines.
  const previewSource = $derived(huge ? text.slice(0, 8000) : text);
  const previewSplit = $derived(splitOutputLines(previewSource));
  const hugePreviewLines = $derived(previewSplit.lines.slice(0, PREVIEW_LINES));
  const sizeLabel = $derived(
    (text?.length || 0) >= 1048576
      ? (text.length / 1048576).toFixed(1) + ' MB'
      : Math.round((text?.length || 0) / 1024) + ' KB',
  );

  const split = $derived(huge ? { lines: [] } : splitOutputLines(text));
  const collapsible = $derived(!huge && split.lines.length > COLLAPSE_MIN);
  const previewLines = $derived(split.lines.slice(0, PREVIEW_LINES));
  const hiddenCount = $derived(split.lines.length - PREVIEW_LINES);
</script>

{#if huge}
  <div class="tool-output tool-output-huge">
    <div class="output-preview">
      {#if lang}
        <div class="code-with-gutter">
          <div class="code-gutter">
            {#each hugePreviewLines as _line, i (i)}<span>{i + 1}</span>{/each}
          </div>
          <pre><code class="hljs" data-highlight-pending data-lang={lang}
              >{hugePreviewLines.join('\n')}</code
            ></pre>
        </div>
      {:else}
        {#each hugePreviewLines as line, i (i)}<div>{line}</div>{/each}
      {/if}
    </div>
    {#if showFull}
      <pre class="output-huge-full">{text}</pre>
      <button type="button" class="output-huge-toggle" onclick={() => (showFull = false)}
        >{t('session.toolOutputHide')}</button
      >
    {:else}
      <button type="button" class="output-huge-toggle" onclick={() => (showFull = true)}
        >{t('session.toolOutputShowFull', { size: sizeLabel })}</button
      >
    {/if}
  </div>
{:else if collapsible}
  <!-- eslint-disable-next-line svelte/no-static-element-interactions -->
  <div class="tool-output expandable" onclick={toggleExpanded} role="presentation">
    <div class="output-preview">
      {#if lang}
        <div class="code-with-gutter">
          <div class="code-gutter">
            {#each previewLines as _line, i (i)}<span>{i + 1}</span>{/each}
          </div>
          <pre><code class="hljs" data-highlight-pending data-lang={lang}
              >{previewLines.join('\n')}</code
            ></pre>
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
          <pre><code class="hljs" data-highlight-pending data-lang={lang}
              >{split.lines.join('\n')}</code
            ></pre>
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
