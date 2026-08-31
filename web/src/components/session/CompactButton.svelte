<script>
  import { onMount } from 'svelte';
  import { t } from '../../shared/i18n.js';
  import { showToast } from '../../shared/toast.js';

  // Manual context compaction. This concern is independent of the git bar it
  // renders next to: it POSTs a fire-and-forget compaction (the server returns
  // 202 immediately) and reconciles the real state from worker-status polling,
  // which broadcasts pi-compact-state / pi-compact-error window events. That
  // makes the busy state survive reloads and stay consistent across tabs.

  // Safety net: if no completion signal arrives within this window (e.g. the
  // worker died) stop showing "Compacting…". Kept above the server-side
  // compactRequestTimeout (5m) so the SSE-driven signal wins normally.
  const COMPACT_TIMEOUT_MS = 6 * 60_000;

  let { sessionId = '', windowImpl, fetchImpl, setTimeoutImpl, clearTimeoutImpl } = $props();

  let compacting = $state(false);
  let title = $state(t('git.compact'));

  // Non-reactive turn-local bookkeeping.
  let timer = undefined;
  // True only for a compaction this tab kicked off, so the success toast fires
  // for the initiator but not for other tabs merely reconciling server state.
  let initiated = false;

  const win = () => windowImpl ?? window;
  const clearTimer = () => {
    if (timer === undefined) return;
    (clearTimeoutImpl ?? win().clearTimeout?.bind(win()))?.(timer);
    timer = undefined;
  };

  function clear() {
    compacting = false;
    clearTimer();
  }

  function fail(message) {
    if (!compacting && !initiated) return;
    clear();
    const msg = message || t('git.compactFailed');
    title = msg;
    showToast(msg, { id: 'compact-toast' });
    initiated = false;
  }

  function start(event) {
    event?.preventDefault?.();
    if (compacting) return;
    compacting = true;
    initiated = true;
    title = t('git.compact');
    const setTimer = setTimeoutImpl ?? win().setTimeout?.bind(win());
    if (setTimer) timer = setTimer(() => fail(t('git.compactFailed')), COMPACT_TIMEOUT_MS);
    const doFetch = fetchImpl ?? win().fetch?.bind(win()) ?? fetch;
    Promise.resolve(
      doFetch('/api/chat/compact?id=' + encodeURIComponent(sessionId), { method: 'POST' }),
    )
      .then((resp) =>
        resp
          .json()
          .catch(() => ({}))
          .then((data) => ({ ok: resp.ok, data })),
      )
      .then(({ ok, data }) => {
        // 202 = queued; completion is signalled via worker-status. Only a
        // synchronous rejection (bad request, shutting down) is final here.
        if (!ok) throw new Error(data.error || t('git.compactFailed'));
      })
      .catch((err) => fail(String(err && err.message ? err.message : err)));
  }

  onMount(() => {
    const w = win();
    // Authoritative compaction state from worker-status polling: keeps the
    // button in sync after a reload and across tabs, and fires the success
    // toast for whichever tab started it when it flips off.
    const onCompactState = (e) => {
      const active = !!(e && e.detail && e.detail.compacting);
      if (active) {
        if (!compacting) compacting = true;
        return;
      }
      if (!compacting && !initiated) return;
      clear();
      if (initiated) {
        title = t('git.compact');
        showToast(t('git.compacted'), { id: 'compact-toast' });
        initiated = false;
      }
    };
    const onCompactError = (e) => {
      const detail = e && e.detail;
      fail((detail && detail.error) || t('git.compactFailed'));
    };
    w.addEventListener?.('pi-compact-state', onCompactState);
    w.addEventListener?.('pi-compact-error', onCompactError);
    return () => {
      clearTimer();
      w.removeEventListener?.('pi-compact-state', onCompactState);
      w.removeEventListener?.('pi-compact-error', onCompactError);
    };
  });
</script>

<button
  type="button"
  class="pi-footer-button pi-compact-button"
  id="pi-compact-button"
  {title}
  disabled={compacting}
  onclick={start}
  ><span id="pi-compact-label">{compacting ? t('git.compacting') : t('git.compact')}</span></button
>
