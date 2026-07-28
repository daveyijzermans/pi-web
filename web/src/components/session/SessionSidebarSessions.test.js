import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import SessionSidebarSessions from './SessionSidebarSessions.svelte';

describe('SessionSidebarSessions', () => {
  it('loads sessions for the active project and highlights the current session', async () => {
    const fetchSessions = vi.fn().mockResolvedValue({
      sessions: [
        {
          ID: 'current.jsonl',
          Name: 'Current work',
          Project: '/repo/pi-web',
          LastActivity: '2026-07-28T10:00:00Z',
          ModelProvider: 'anthropic',
          Model: 'sonnet',
        },
        {
          ID: 'older.jsonl',
          Name: 'Older work',
          Project: '/repo/pi-web',
          LastActivity: '2026-07-27T10:00:00Z',
        },
      ],
    });

    const { container } = render(SessionSidebarSessions, {
      props: {
        cwd: '/repo/pi-web',
        currentSessionId: 'current.jsonl',
        fetchSessions,
        runningSessionIds: new Set(['current.jsonl']),
      },
    });

    await waitFor(() => expect(fetchSessions).toHaveBeenCalledWith({ project: '/repo/pi-web' }));
    const current = await screen.findByRole('link', { name: /Current work/ });
    expect(current).toHaveAttribute('aria-current', 'page');
    expect(screen.getByText('pi-web')).toBeInTheDocument();
    expect(screen.getByText('anthropic/sonnet')).toBeInTheDocument();
    const spinner = container.querySelector('[data-running-spinner]');
    expect(spinner).toBeInTheDocument();
    expect(spinner.style.fontFamily).toContain('runcat');
  });

  it('groups sessions by recency', async () => {
    const now = Date.now();
    render(SessionSidebarSessions, {
      props: {
        cwd: '/repo',
        fetchSessions: vi.fn().mockResolvedValue({
          sessions: [
            { id: 'today.jsonl', name: 'Work today', lastActivity: new Date(now).toISOString() },
            {
              id: 'older.jsonl',
              name: 'Older work',
              lastActivity: new Date(now - 40 * 86400000).toISOString(),
            },
          ],
        }),
      },
    });

    expect(await screen.findByRole('heading', { name: /Today/ })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /Older/ })).toBeInTheDocument();
  });

  it('filters the loaded project sessions by title', async () => {
    const user = userEvent.setup();
    render(SessionSidebarSessions, {
      props: {
        cwd: '/repo',
        fetchSessions: vi.fn().mockResolvedValue({
          sessions: [
            { id: 'one.jsonl', name: 'Fix sidebar' },
            { id: 'two.jsonl', name: 'Ship release' },
          ],
        }),
      },
    });

    await screen.findByRole('link', { name: /Fix sidebar/ });
    await user.type(screen.getByRole('searchbox', { name: 'Search project sessions…' }), 'ship');

    expect(screen.queryByRole('link', { name: /Fix sidebar/ })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Ship release/ })).toBeInTheDocument();
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
  });
});
