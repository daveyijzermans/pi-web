import { describe, expect, it, vi } from 'vitest';
import { fetchPlanUsage, formatUtilization, nextWindowReset, planWindows } from './plan-usage.js';

const providers = [
  {
    provider: 'anthropic',
    label: 'Anthropic',
    windows: [
      { key: 'five_hour', label: '5h', utilization: 33, resetsAt: '2030-01-01T07:00:00Z' },
      { key: 'seven_day', label: '7d', utilization: 13, resetsAt: '2030-01-06T00:00:00Z' },
    ],
  },
  { provider: 'other', label: 'Other', windows: [{ key: 'w', label: 'W', utilization: 1 }] },
];

describe('plan-usage helpers', () => {
  it('flattens windows with their provider', () => {
    const windows = planWindows(providers);
    expect(windows).toHaveLength(3);
    expect(windows[0]).toMatchObject({ key: 'five_hour', providerLabel: 'Anthropic' });
  });

  it('picks the soonest future reset and ignores past/missing ones', () => {
    const now = Date.parse('2030-01-01T00:00:00Z');
    expect(nextWindowReset(providers, now)?.window.key).toBe('five_hour');
    const later = Date.parse('2030-01-02T00:00:00Z');
    expect(nextWindowReset(providers, later)?.window.key).toBe('seven_day');
    expect(nextWindowReset(providers, Date.parse('2031-01-01T00:00:00Z'))).toBeNull();
    expect(nextWindowReset([], now)).toBeNull();
  });

  it('formats utilization as a rounded percentage', () => {
    expect(formatUtilization(33.4)).toBe('33%');
    expect(formatUtilization('x')).toBe('');
  });

  it('fetchPlanUsage returns [] on failure', async () => {
    const fetchImpl = vi.fn(() => Promise.resolve(new Response('{}', { status: 500 })));
    expect(await fetchPlanUsage({ fetchImpl })).toEqual([]);
    const ok = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ providers }), { status: 200 })),
    );
    expect(await fetchPlanUsage({ fetchImpl: ok })).toHaveLength(2);
  });
});
