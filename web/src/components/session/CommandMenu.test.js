import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';
import { render, cleanup, fireEvent, waitFor } from '@testing-library/svelte';
import CommandMenu from './CommandMenu.svelte';
import { sessionModals, resetSessionModals } from '../../session/session-modals.svelte.js';
import { setSessionPaletteApi } from '../../shared/command-palette-runtime.js';
import { sessionTitle } from '../../session/session-title.svelte.js';
import { sessionArchived } from '../../session/session-archived.svelte.js';

// The menu button (#command-menu-btn) lives in SessionHeader; the menu reads it
// by id, so the test provides it. The session name now flows through the shared
// reactive store, seeded here.
beforeEach(() => {
  document.body.innerHTML = '';
  const btn = document.createElement('button');
  btn.id = 'command-menu-btn';
  document.body.appendChild(btn);
  sessionTitle.name = 'Old';
  sessionArchived.value = false;
  window.matchMedia = vi.fn(() => ({ matches: false }));
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  resetSessionModals();
  setSessionPaletteApi(null);
});

describe('CommandMenu', () => {
  it('toggles archive via the API and flips the menu label', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          new Response(JSON.stringify({ ok: true, archived: true }), { status: 200 }),
        ),
      ),
    );
    render(CommandMenu, { props: { sessionId: 'session.jsonl' } });
    await tick();

    const item = document.querySelector('#command-menu-popover [data-action="archive"]');
    expect(item.textContent).toContain('Archive');
    await fireEvent.click(item);
    await waitFor(() => expect(sessionArchived.value).toBe(true));
    expect(fetch).toHaveBeenCalledWith(
      '/api/archive-session?id=session.jsonl',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ archived: true }) }),
    );
    await tick();
    expect(
      document.querySelector('#command-menu-popover [data-action="archive"]').textContent,
    ).toContain('Restore from archive');
  });

  it('renames via the API and updates the page title', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(new Response(JSON.stringify({ name: 'New Name' }), { status: 200 })),
      ),
    );
    window.prompt = vi.fn(() => ' New Name ');
    render(CommandMenu, { props: { sessionId: 'session.jsonl' } });
    await tick();

    await fireEvent.click(document.querySelector('[data-action="rename"]'));
    await waitFor(() => expect(sessionTitle.name).toBe('New Name'));
    expect(fetch).toHaveBeenCalledWith(
      '/api/rename-session?id=session.jsonl',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('keeps the old title when the rename API fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response(JSON.stringify({ error: 'bad' }), { status: 500 }))),
    );
    window.prompt = vi.fn(() => 'New Name');
    render(CommandMenu, { props: { sessionId: 'session.jsonl' } });
    await tick();

    await fireEvent.click(document.querySelector('[data-action="rename"]'));
    await waitFor(() =>
      expect(document.getElementById('command-menu-toast')?.textContent).toBe('Rename failed'),
    );
    expect(sessionTitle.name).toBe('Old');
  });

  it('opens model usage via the modal store + the session-list palette runtime', async () => {
    const openPalette = vi.fn();
    render(CommandMenu, { props: { sessionId: 's' } });
    await tick();
    setSessionPaletteApi({ open: openPalette });

    await fireEvent.click(document.querySelector('[data-action="model-usage"]'));
    await fireEvent.click(document.querySelector('[data-action="list-sessions"]'));
    expect(sessionModals.modelUsage).toBe(true);
    expect(openPalette).toHaveBeenCalled();
  });
});
