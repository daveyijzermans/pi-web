<script>
  // One conversation entry in the message pane, rendered declaratively (the
  // decomposition of the former renderEntry()). {@html} is used only for markdown
  // (safeMarkedParse) — everything else is escaped Svelte template. The wrapper
  // keeps its `entry-<id>` anchor so annotation offsets + scroll/toggle survive.
  // Shared by the live app and the static export (model passed as a prop).
  import { marked } from 'marked';
  import { icon, GitFork, Link2, Paperclip, Tag } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import { safeMarkedParse } from '../../session/render/markdown.js';
  import { formatTimestamp } from '../../session/render/entry-format.js';
  import { resolveToolResult } from '../../session/render/tool-summary.js';
  import { extractAttachmentRefs } from './attachment-refs.js';
  import ToolOutput from './ToolOutput.svelte';
  import ToolCall from './ToolCall.svelte';
  import AskQuestion from './AskQuestion.svelte';

  // `live` (passed from <SessionContent>) gates the fork/label buttons, which
  // need the chat composer; copy-link is always shown. The static export passes
  // false. (Replaces the former renderForkButton/renderLabelButton isLive check —
  // a prop, not a DOM probe, since entries mount before the composer.)
  let { entry, model = null, live = false } = $props();

  const ts = $derived(formatTimestamp(entry?.timestamp));
  const md = (text) => safeMarkedParse(text, { marked });

  const msg = $derived(entry?.type === 'message' ? entry.message : null);
  const rawUserText = $derived.by(() => {
    if (!msg || msg.role !== 'user') return '';
    const c = msg.content;
    return typeof c === 'string'
      ? c
      : c
          .filter((b) => b.type === 'text')
          .map((b) => b.text)
          .join('\n');
  });
  const { text: userText, refs: userAttachments } = $derived(extractAttachmentRefs(rawUserText));
  const userImages = $derived(
    Array.isArray(msg?.content) ? msg.content.filter((b) => b.type === 'image') : [],
  );

  // Group non-text assistant blocks (thinking + toolCalls) into collapsible actions
  // segments, interleaved with text segments, preserving document order. Each segment
  // carries sourceIds (which raw entries its blocks came from) for data-entry-ids.
  const assistantSegments = $derived.by(() => {
    if (!msg || msg.role !== 'assistant') return [];
    const segments = [];
    let run = null;
    const flush = () => {
      if (run && run.length > 0) {
        const sourceIds = [...new Set(run.map((item) => item.sourceId).filter(Boolean))];
        const seg = {
          kind: 'actions',
          items: run,
          timestamp: run[0].timestamp ?? entry.timestamp,
          sourceIds,
        };
        segments.push(seg);
        run = null;
      }
    };
    for (const block of msg.content) {
      if (block.type === 'text' && block.text?.trim()) {
        flush();
        const sourceId = block.sourceId ?? entry.id;
        segments.push({
          kind: 'text',
          text: block.text,
          timestamp: block.timestamp ?? entry.timestamp,
          sourceIds: [sourceId],
        });
      } else if (block.type === 'thinking' && block.thinking?.trim()) {
        if (!run) run = [];
        run.push({
          type: 'thinking',
          text: block.thinking,
          timestamp: block.timestamp,
          sourceId: block.sourceId,
        });
      } else if (block.type === 'toolCall') {
        if (block.name === 'ask_user_question' || block.name === 'pi_web_ask_user_question') {
          flush();
          segments.push({
            kind: 'ask_question',
            block,
            timestamp: block.timestamp ?? entry.timestamp,
            sourceIds: [block.sourceId ?? entry.id].filter(Boolean),
          });
        } else {
          if (!run) run = [];
          run.push({
            type: 'toolCall',
            block,
            timestamp: block.timestamp,
            sourceId: block.sourceId,
          });
        }
      }
    }
    flush();
    return segments;
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG and rendered session markdown -->

{#snippet actions(id)}
  {#if live}<button class="fork-btn" data-entry-id={id} title="Fork session from this message"
      >{@html icon(GitFork, { size: 13 })}</button
    >{/if}
  {#if live}<button
      class="label-btn"
      data-entry-id={id}
      title={t('session.labelEntry')}
      aria-label={t('session.labelEntry')}>{@html icon(Tag, { size: 13 })}</button
    >{/if}
  <button class="copy-link-btn" data-entry-id={id} title="Copy link to this message"
    >{@html icon(Link2, { size: 14 })}</button
  >
{/snippet}
{#snippet timestamp()}{#if ts}<div class="message-timestamp">{ts}</div>{/if}{/snippet}
{#snippet tsFor(rawTs)}{#if rawTs}<div class="message-timestamp">
      {formatTimestamp(rawTs)}
    </div>{/if}{/snippet}

{#if msg && msg.role === 'user'}
  <div class="user-message" id={`entry-${entry.id}`}>
    {@render actions(entry.id)}{@render timestamp()}
    {#if userImages.length > 0}<div class="message-images">
        {#each userImages as img, imgIndex (imgIndex)}<img
            src={`data:${img.mimeType || 'image/png'};base64,${img.data}`}
            class="message-image"
            alt=""
          />{/each}
      </div>{/if}
    {#if userText.trim()}<div class="markdown-content">{@html md(userText)}</div>{/if}
    {#if userAttachments.length > 0}<div class="message-attachments">
        {#each userAttachments as ref, i (i)}<span class="message-attachment" title={ref.path}
            >{@html icon(Paperclip, { size: 13 })}{ref.name}</span
          >{/each}
      </div>{/if}
  </div>
{:else if msg && msg.role === 'assistant'}
  <div class="assistant-message" id={`entry-${entry.id}`}>
    {@render actions(entry.id)}
    {#if assistantSegments.length === 0}{@render tsFor(entry.timestamp)}{/if}
    {#each assistantSegments as seg, segIndex (segIndex)}
      <div class="assistant-group" data-entry-ids={seg.sourceIds.join(' ')}>
        {@render tsFor(seg.timestamp)}
        {#if seg.kind === 'text'}
          <div class="assistant-text markdown-content">
            {@html md(seg.text)}
          </div>
        {:else if seg.kind === 'ask_question'}
          <AskQuestion
            args={seg.block.arguments || {}}
            result={resolveToolResult(model, seg.block.id)}
          />
        {:else}
          {#each seg.items as item, itemIndex (itemIndex)}
            {#if item.type === 'thinking'}
              <div class="thinking-block">
                <div class="thinking-text">{item.text}</div>
                <div class="thinking-collapsed">Thinking ...</div>
              </div>
            {:else if item.type === 'toolCall'}
              <ToolCall call={item.block} {model} />
            {/if}
          {/each}
        {/if}
      </div>
    {/each}
    {#if msg.stopReason === 'aborted'}<div class="error-text">
        Aborted
      </div>{:else if msg.stopReason === 'error'}<div class="error-text">
        Error: {msg.errorMessage || 'Unknown error'}
      </div>{/if}
  </div>
{:else if msg && msg.role === 'bashExecution'}
  <div
    class="tool-execution {msg.cancelled || (msg.exitCode !== 0 && msg.exitCode !== null)
      ? 'error'
      : 'success'}"
    id={`entry-${entry.id}`}
  >
    {@render timestamp()}
    <div class="tool-command">$ {msg.command}</div>
    {#if msg.output}<ToolOutput text={msg.output} />{/if}
    {#if msg.cancelled}<div style="color: var(--warning)">
        (cancelled)
      </div>{:else if msg.exitCode !== 0 && msg.exitCode !== null}<div style="color: var(--error)">
        (exit {msg.exitCode})
      </div>{/if}
  </div>
{:else if entry?.type === 'model_change' && !entry.implicit}
  <div class="model-change" id={`entry-${entry.id}`}>
    {@render timestamp()}Switched to model:
    <span class="model-name">{entry.provider}/{entry.modelId}</span>
  </div>
{:else if entry?.type === 'compaction'}
  <div
    class="compaction"
    id={`entry-${entry.id}`}
    onclick={(e) => {
      if (window.getSelection && window.getSelection().toString()) return;
      e.currentTarget.classList.toggle('expanded');
    }}
    role="presentation"
  >
    <div class="compaction-label">[compaction]</div>
    <div class="compaction-collapsed">
      Compacted from {entry.tokensBefore.toLocaleString()} tokens
    </div>
    <div class="compaction-content">
      <strong>Compacted from {entry.tokensBefore.toLocaleString()} tokens</strong
      >{'\n\n'}{entry.summary}
    </div>
  </div>
{:else if entry?.type === 'branch_summary'}
  <div class="branch-summary" id={`entry-${entry.id}`}>
    {@render timestamp()}
    <div class="branch-summary-header">Branch Summary</div>
    <div class="markdown-content">{@html md(entry.summary)}</div>
  </div>
{:else if entry?.type === 'custom_message' && entry.display}
  <div class="hook-message" id={`entry-${entry.id}`}>
    {@render timestamp()}
    <div class="hook-type">[{entry.customType}]</div>
    <div class="markdown-content">
      {@html md(typeof entry.content === 'string' ? entry.content : JSON.stringify(entry.content))}
    </div>
  </div>
{/if}
