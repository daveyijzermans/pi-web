<script>
  // Subscription-plan window utilization next to the model label. Hidden
  // until /api/plan-usage reports at least one window; polled every minute.
  import { onMount } from 'svelte';
  import { t } from '../../../shared/i18n.js';
  import { fetchPlanUsage, formatResetTime, formatUtilization, planWindows } from './plan-usage.js';

  let { fetchImpl = fetch, pollMs = 60000 } = $props();

  let providers = $state([]);
  let now = $state(Date.now());
  const windows = $derived(planWindows(providers));
  const label = $derived(
    windows.map((window) => `${window.label} ${formatUtilization(window.utilization)}`).join(' · '),
  );
  const title = $derived(
    windows
      .map((window) => {
        const reset = window.resetsAt
          ? ` — ${t('composer.planResets', { time: formatResetTime(window.resetsAt, now) })}`
          : '';
        return `${window.providerLabel} ${window.label}: ${formatUtilization(window.utilization)}${reset}`;
      })
      .join('\n'),
  );

  onMount(() => {
    let active = true;
    const load = async () => {
      const next = await fetchPlanUsage({ fetchImpl });
      if (!active) return;
      providers = next;
      now = Date.now();
    };
    void load();
    const timer = setInterval(load, pollMs);
    return () => {
      active = false;
      clearInterval(timer);
    };
  });
</script>

{#if windows.length > 0}
  <span class="pi-chat-plan-usage" id="pi-chat-plan-usage" {title} aria-label={title}>{label}</span>
{/if}
