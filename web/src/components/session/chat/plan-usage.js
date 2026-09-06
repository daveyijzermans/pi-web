// Client for GET /api/plan-usage plus the small pure helpers the composer and
// the send-later picker share. Provider-neutral: a "window" is any rate-limit
// bucket with a utilization percentage and an optional reset time.

export async function fetchPlanUsage({ fetchImpl = fetch } = {}) {
  try {
    const resp = await fetchImpl('/api/plan-usage', { headers: { Accept: 'application/json' } });
    if (!resp.ok) return [];
    const data = await resp.json();
    return Array.isArray(data?.providers) ? data.providers : [];
  } catch {
    return [];
  }
}

/** Flatten every provider's windows into one list, keeping the provider label. */
export function planWindows(providers = []) {
  const out = [];
  for (const provider of providers) {
    for (const window of provider?.windows || []) {
      out.push({ ...window, provider: provider.provider, providerLabel: provider.label });
    }
  }
  return out;
}

/** The soonest future reset across all windows, or null. */
export function nextWindowReset(providers = [], now = Date.now()) {
  let best = null;
  for (const window of planWindows(providers)) {
    const ms = Date.parse(window.resetsAt || '');
    if (!Number.isFinite(ms) || ms <= now) continue;
    if (!best || ms < best.ms) best = { ms, window };
  }
  return best;
}

export function formatUtilization(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) return '';
  return `${Math.round(n)}%`;
}

export function formatResetTime(iso, now = Date.now(), locale = undefined) {
  const ms = Date.parse(iso || '');
  if (!Number.isFinite(ms)) return '';
  const date = new Date(ms);
  const sameDay = new Date(now).toDateString() === date.toDateString();
  const time = date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  if (sameDay) return time;
  return `${date.toLocaleDateString(locale, { weekday: 'short' })} ${time}`;
}
