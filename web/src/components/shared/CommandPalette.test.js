import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@testing-library/svelte';
import CommandPalette, {
  filterPaletteSessions,
  normalizePaletteSession,
} from './CommandPalette.svelte';
import {
  getSessionPaletteApi,
  openSessionPalette,
  setSessionPaletteApi,
} from '../../shared/command-palette-runtime.js';

afterEach(() => {
  cleanup();
  setSessionPaletteApi(null);
  delete window.__piOpenSessionPalette;
  delete window.__piSessionPalette;
});

describe('CommandPalette', () => {
  it('normalizes and filters sessions', () => {
    const session = normalizePaletteSession({ ID: 'abc', Name: 'Fix bug', Project: '/repo' });
    expect(session.href).toBe('/session?id=abc');
    expect(filterPaletteSessions([session], 'fix')).toHaveLength(1);
    expect(filterPaletteSessions([session], 'missing')).toHaveLength(0);
  });

  it('opens through the window bridge and navigates a selected session', async () => {
    const seen = [];
    render(CommandPalette, {
      props: {
        loadSessions: async () => [{ id: 's1', name: 'Session one', model: 'm' }],
        navigate: (url) => seen.push(url),
      },
    });
    await window.__piOpenSessionPalette();
    await screen.findByText('Session one');
    await fireEvent.click(screen.getByText('Session one'));
    expect(seen).toEqual(['/session?id=s1']);
  });

  it('registers the explicit session palette runtime API', async () => {
    render(CommandPalette, {
      props: {
        loadSessions: async () => [{ id: 's1', name: 'Session one', model: 'm' }],
      },
    });
    expect(getSessionPaletteApi()).toBeTruthy();
    await openSessionPalette();
    expect(await screen.findByText('Session one')).toBeTruthy();
  });

  it('shows the configured running animation beside active sessions', async () => {
    const { container } = render(CommandPalette, {
      props: {
        loadSessions: async () => [{ id: 's1', name: 'Session one', model: 'm' }],
        runningSessionIds: new Set(['s1']),
      },
    });
    await openSessionPalette();
    await screen.findByText('Session one');

    const spinner = container.querySelector('[data-running-spinner]');
    expect(spinner).toBeTruthy();
    expect(spinner.textContent).not.toBe('');
    expect(spinner.style.fontFamily).toContain('runcat');
  });

  it('only renders supported session actions', () => {
    const { container } = render(CommandPalette);
    expect(container.querySelector('[data-new-session-btn]')).toBeTruthy();
    expect(container.querySelector('[data-import-session-btn]')).toBeNull();
  });
});
