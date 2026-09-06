<script>
  // "Send later" — queues the composed turn (text + attachments) with a
  // not-before time. The server-side drainer fires it when due, so it works
  // with the browser closed. Presets are relative offsets plus, when a
  // subscription provider reports a rate-limit window, "when the window
  // resets".
  import { onMount } from 'svelte';
  import { icon, CalendarClock } from '../../../shared/icons.js';
  import { t } from '../../../shared/i18n.js';
  import { fetchPlanUsage, formatResetTime, nextWindowReset } from './plan-usage.js';

  let { store, chatAvailable = true, fetchImpl = fetch } = $props();

  let open = $state(false);
  let value = $state('');
  let hint = $state('');
  let providers = $state([]);
  let rootEl = $state(null);
  let buttonEl = $state(null);
  let inputEl = $state(null);
  // The toolbar clips overflow, so the popover is fixed-positioned above the
  // button (recomputed on open and on resize).
  let popoverStyle = $state('');
  const reset = $derived(nextWindowReset(providers));

  function toLocalInput(date) {
    const pad = (n) => String(n).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }

  function setOffset(ms) {
    value = toLocalInput(new Date(Date.now() + ms));
  }

  function setTomorrowMorning() {
    const now = new Date();
    value = toLocalInput(
      new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 9, 0, 0, 0),
    );
  }

  function setWindowReset() {
    if (!reset) return;
    // One minute past the reset so the provider has definitely rolled over.
    value = toLocalInput(new Date(reset.ms + 60000));
  }

  function placePopover() {
    const rect = buttonEl?.getBoundingClientRect?.();
    if (!rect) return;
    const bottom = Math.max(0, window.innerHeight - rect.top + 6);
    const right = Math.max(8, window.innerWidth - rect.right);
    popoverStyle = `bottom:${bottom}px;right:${right}px`;
  }

  async function toggle() {
    if (open) {
      open = false;
      return;
    }
    hint = '';
    if (!value) setOffset(60 * 60 * 1000);
    placePopover();
    open = true;
    providers = await fetchPlanUsage({ fetchImpl });
    inputEl?.focus?.();
  }

  function schedule() {
    const ms = Date.parse(value);
    if (!Number.isFinite(ms)) {
      hint = t('composer.sendLaterInvalid');
      return;
    }
    if (ms <= Date.now()) {
      hint = t('composer.sendLaterPast');
      return;
    }
    if (!store.actions.hasComposerContent?.()) {
      hint = t('composer.sendLaterEmpty');
      return;
    }
    const ok = store.actions.enqueueLater?.(new Date(ms).toISOString());
    if (!ok) {
      hint = t('composer.sendLaterEmpty');
      return;
    }
    open = false;
  }

  onMount(() => {
    const onDocClick = (event) => {
      if (open && rootEl && !rootEl.contains(event.target)) open = false;
    };
    const onKey = (event) => {
      if (event.key === 'Escape' && open) {
        event.stopPropagation();
        open = false;
      }
    };
    const onResize = () => {
      if (open) placePopover();
    };
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKey, true);
    window.addEventListener('resize', onResize);
    return () => {
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onKey, true);
      window.removeEventListener('resize', onResize);
    };
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG -->
<div class="pi-send-later" bind:this={rootEl}>
  <button
    type="button"
    id="pi-chat-send-later"
    class="pi-chat-icon-button pi-chat-send-later-button"
    title={t('composer.sendLaterHint')}
    aria-label={t('composer.sendLater')}
    aria-haspopup="dialog"
    aria-expanded={open}
    disabled={!chatAvailable}
    bind:this={buttonEl}
    onclick={toggle}>{@html icon(CalendarClock, { size: 14 })}</button
  >
  {#if open}
    <div
      class="pi-send-later-popover"
      role="dialog"
      aria-label={t('composer.sendLater')}
      style={popoverStyle}
    >
      <div class="pi-send-later-title">{t('composer.sendLater')}</div>
      <input
        type="datetime-local"
        class="pi-send-later-input"
        bind:this={inputEl}
        bind:value
        aria-label={t('composer.sendLaterAt')}
        onkeydown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            schedule();
          }
        }}
      />
      <div class="pi-send-later-presets">
        <button type="button" onclick={() => setOffset(60 * 60 * 1000)}>+1h</button>
        <button type="button" onclick={() => setOffset(4 * 60 * 60 * 1000)}>+4h</button>
        <button type="button" onclick={setTomorrowMorning}>{t('composer.sendLaterTomorrow')}</button
        >
        {#if reset}
          <button
            type="button"
            class="pi-send-later-reset"
            title={`${reset.window.providerLabel} ${reset.window.label} · ${formatResetTime(reset.window.resetsAt)}`}
            onclick={setWindowReset}>{t('composer.sendLaterWindowReset')}</button
          >
        {/if}
      </div>
      {#if hint}<div class="pi-send-later-hint">{hint}</div>{/if}
      <div class="pi-send-later-actions">
        <button type="button" class="pi-send-later-cancel" onclick={() => (open = false)}
          >{t('common.cancel')}</button
        >
        <button type="button" class="pi-send-later-confirm" onclick={schedule}
          >{t('composer.sendLaterConfirm')}</button
        >
      </div>
    </div>
  {/if}
</div>
