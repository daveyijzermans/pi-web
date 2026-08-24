import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import CompactButton from './CompactButton.svelte';
import { showToast } from '../../shared/toast.js';

vi.mock('../../shared/toast.js', () => ({ showToast: vi.fn() }));

const flush = () => new Promise((r) => setTimeout(r, 0));
const id = (x) => document.getElementById(x);

function renderButton(extra = {}) {
  return render(CompactButton, { props: { sessionId: 's', windowImpl: window, ...extra } });
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('CompactButton', () => {
  it('posts to /api/chat/compact and stays busy (202) until worker-status clears it', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue({ ok: true, json: async () => ({ ok: true, status: 'queued' }) });
    renderButton({ fetchImpl: fetchMock });
    await flush();

    id('pi-compact-button').click();
    await flush();
    await flush();

    expect(fetchMock).toHaveBeenCalledWith('/api/chat/compact?id=s', { method: 'POST' });
    // Fire-and-forget: the 202 leaves the button busy; completion is driven by
    // the worker-status pi-compact-state event.
    expect(id('pi-compact-button').disabled).toBe(true);
    expect(id('pi-compact-label').textContent).toBe('Compacting…');

    window.dispatchEvent(new CustomEvent('pi-compact-state', { detail: { compacting: false } }));
    await flush();

    expect(id('pi-compact-button').disabled).toBe(false);
    expect(id('pi-compact-label').textContent).toBe('compact');
    // The initiating tab gets a success toast.
    expect(showToast).toHaveBeenCalledWith('Session compacted', { id: 'compact-toast' });
  });

  it('reconciles an in-flight compaction from worker-status after a reload', async () => {
    renderButton();
    await flush();

    // No click: a fresh page load learns compaction is active from polling.
    window.dispatchEvent(new CustomEvent('pi-compact-state', { detail: { compacting: true } }));
    await flush();
    expect(id('pi-compact-button').disabled).toBe(true);
    expect(id('pi-compact-label').textContent).toBe('Compacting…');

    window.dispatchEvent(new CustomEvent('pi-compact-state', { detail: { compacting: false } }));
    await flush();
    expect(id('pi-compact-button').disabled).toBe(false);
    expect(id('pi-compact-label').textContent).toBe('compact');
  });

  it('clears busy and shows the error from a pi-compact-error event', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue({ ok: true, json: async () => ({ ok: true, status: 'queued' }) });
    renderButton({ fetchImpl: fetchMock });
    await flush();

    id('pi-compact-button').click();
    await flush();
    await flush();
    expect(id('pi-compact-button').disabled).toBe(true);

    window.dispatchEvent(
      new CustomEvent('pi-compact-error', {
        detail: { error: 'Nothing to compact (session too small)' },
      }),
    );
    await flush();

    expect(id('pi-compact-button').title).toBe('Nothing to compact (session too small)');
    expect(id('pi-compact-button').disabled).toBe(false);
    expect(id('pi-compact-label').textContent).toBe('compact');
  });

  it('shows the error in the title when the request is rejected synchronously', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ error: 'chat unavailable' }),
    });
    renderButton({ fetchImpl: fetchMock });
    await flush();

    id('pi-compact-button').click();
    await flush();
    await flush();

    expect(id('pi-compact-button').title).toBe('chat unavailable');
    expect(id('pi-compact-button').disabled).toBe(false);
    expect(id('pi-compact-label').textContent).toBe('compact');
  });

  it('does not toast when another tab initiated the compaction (reconcile only)', async () => {
    // compacting flips true→false with no local click: reconcile only, no toast.
    renderButton();
    await flush();
    window.dispatchEvent(new CustomEvent('pi-compact-state', { detail: { compacting: true } }));
    await flush();
    window.dispatchEvent(new CustomEvent('pi-compact-state', { detail: { compacting: false } }));
    await flush();
    expect(showToast).not.toHaveBeenCalled();
    expect(id('pi-compact-button').disabled).toBe(false);
  });
});
