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

    await waitFor(() =>
      expect(fetchSessions).toHaveBeenCalledWith({
        project: '/repo/pi-web',
        limit: 20,
        offset: 0,
      }),
    );
    const current = await screen.findByRole('link', { name: /Current work/ });
    expect(current).toHaveAttribute('aria-current', 'page');
    expect(screen.getByText('pi-web')).toBeInTheDocument();
    expect(screen.getByText('anthropic/sonnet')).toBeInTheDocument();
    const activeIndicator = container.querySelector('.sidebar-session-indicator');
    expect(activeIndicator.textContent).not.toBe('');
    expect(activeIndicator.style.fontFamily).toContain('runcat');
    expect(activeIndicator).toHaveClass('sidebar-session-indicator--running');
  });

  it('runs (animates) the active session cat while its own turn is running', async () => {
    const fetchSessions = vi.fn().mockResolvedValue({
      sessions: [
        {
          ID: 'current.jsonl',
          Name: 'Current work',
          Project: '/repo/pi-web',
          LastActivity: '2026-07-28T10:00:00Z',
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

    await screen.findByRole('link', { name: /Current work/ });
    const indicator = container.querySelector('.sidebar-session-indicator');
    const firstFrame = indicator.textContent;
    // The runcat frames cycle on an interval, so the glyph must change over
    // time — a static frame would mean the running cat isn't running.
    await waitFor(() => expect(indicator.textContent).not.toBe(firstFrame), { timeout: 2000 });
  });

  it('switches the sidebar to sessions from a selected project', async () => {
    const user = userEvent.setup();
    const fetchSessions = vi.fn(({ project }) =>
      Promise.resolve({
        sessions:
          project === '/repo/other'
            ? [{ id: 'other.jsonl', name: 'Other project work' }]
            : [{ id: 'current.jsonl', name: 'Current project work' }],
      }),
    );
    const fetchProjects = vi.fn().mockResolvedValue({
      projects: [
        { path: '/repo/pi-web', sessionCount: 1 },
        { path: '/repo/other', sessionCount: 1 },
      ],
    });

    render(SessionSidebarSessions, {
      props: {
        cwd: '/repo/pi-web',
        currentSessionId: 'current.jsonl',
        fetchSessions,
        fetchProjects,
      },
    });

    await screen.findByRole('link', { name: /Current project work/ });
    await user.click(screen.getByRole('button', { name: /Current project pi-web/ }));

    expect(fetchProjects).toHaveBeenCalledOnce();
    await user.type(screen.getByRole('searchbox', { name: 'Search projects…' }), 'other');
    await user.click(screen.getByRole('button', { name: /other \/repo\/other/ }));

    await waitFor(() =>
      expect(fetchSessions).toHaveBeenCalledWith({
        project: '/repo/other',
        limit: 20,
        offset: 0,
      }),
    );
    expect(await screen.findByRole('link', { name: /Other project work/ })).toBeInTheDocument();
    expect(screen.getByText('Browsing project')).toBeInTheDocument();
    expect(screen.getByText('other')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Current project work/ })).not.toBeInTheDocument();
  });

  it('shows sessions from every project grouped by project when All is selected', async () => {
    const user = userEvent.setup();
    const fetchSessions = vi.fn(({ project }) =>
      Promise.resolve({
        sessions:
          project === undefined
            ? [
                {
                  ID: 'a.jsonl',
                  Name: 'Alpha work',
                  Project: '/repo/pi-web',
                  LastActivity: '2026-07-28T10:00:00Z',
                },
                {
                  ID: 'b.jsonl',
                  Name: 'Beta work',
                  Project: '/repo/other',
                  LastActivity: '2026-07-27T10:00:00Z',
                },
              ]
            : [{ ID: 'a.jsonl', Name: 'Alpha work', Project: '/repo/pi-web' }],
        total: project === undefined ? 2 : 1,
      }),
    );
    const fetchProjects = vi.fn().mockResolvedValue({
      projects: [
        { path: '/repo/pi-web', sessionCount: 1 },
        { path: '/repo/other', sessionCount: 1 },
      ],
    });

    render(SessionSidebarSessions, {
      props: { cwd: '/repo/pi-web', fetchSessions, fetchProjects },
    });

    await screen.findByRole('link', { name: /Alpha work/ });
    await user.click(screen.getByRole('button', { name: /Current project pi-web/ }));
    await user.click(screen.getByRole('button', { name: /All projects/ }));

    await waitFor(() => expect(fetchSessions).toHaveBeenCalledWith({ limit: 20, offset: 0 }));
    // Grouped by project → one heading per project, both projects' sessions shown.
    expect(await screen.findByRole('heading', { name: /pi-web/ })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /other/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Beta work/ })).toBeInTheDocument();
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
    const fetchSessions = vi.fn(({ query }) =>
      Promise.resolve({
        sessions: query
          ? [{ id: 'two.jsonl', name: 'Ship release' }]
          : [
              { id: 'one.jsonl', name: 'Fix sidebar' },
              { id: 'two.jsonl', name: 'Ship release' },
            ],
        total: query ? 1 : 2,
      }),
    );
    render(SessionSidebarSessions, {
      props: {
        cwd: '/repo',
        fetchSessions,
      },
    });

    await screen.findByRole('link', { name: /Fix sidebar/ });
    await user.type(screen.getByRole('searchbox', { name: 'Search project sessions…' }), 'ship');

    await waitFor(() =>
      expect(fetchSessions).toHaveBeenCalledWith({
        project: '/repo',
        limit: 20,
        offset: 0,
        query: 'ship',
      }),
    );
    expect(await screen.findByRole('link', { name: /Ship release/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Fix sidebar/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
    expect(screen.getByText('1–1 / 1 sessions')).toBeInTheDocument();
  });

  it('loads one page of sessions at a time', async () => {
    const user = userEvent.setup();
    const fetchSessions = vi.fn(({ offset }) =>
      Promise.resolve({
        sessions:
          offset === 20
            ? [{ id: 'last.jsonl', name: 'Last session' }]
            : Array.from({ length: 20 }, (_, index) => ({
                id: `${index}.jsonl`,
                name: `Session ${index + 1}`,
              })),
        total: 21,
      }),
    );

    render(SessionSidebarSessions, {
      props: {
        cwd: '/repo',
        fetchSessions,
      },
    });

    expect(await screen.findByText('1–20 / 21 sessions')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Next sessions page' }));

    await waitFor(() =>
      expect(fetchSessions).toHaveBeenCalledWith({
        project: '/repo',
        limit: 20,
        offset: 20,
      }),
    );
    expect(await screen.findByRole('link', { name: /Last session/ })).toBeInTheDocument();
    expect(screen.getByText('21–21 / 21 sessions')).toBeInTheDocument();
  });
});
