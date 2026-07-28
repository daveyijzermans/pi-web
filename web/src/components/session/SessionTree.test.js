import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import SessionTree from './SessionTree.svelte';

describe('SessionTree', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('tabs between projects, project sessions, and the message outline', async () => {
    const user = userEvent.setup();
    render(SessionTree);

    const projectsTab = screen.getByRole('tab', { name: 'Projects' });
    const sessionsTab = screen.getByRole('tab', { name: 'Sessions' });
    const outlineTab = screen.getByRole('tab', { name: 'Outline' });
    expect(sessionsTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('searchbox', { name: 'Search project sessions…' })).toBeInTheDocument();

    await user.click(projectsTab);
    expect(projectsTab).toHaveAttribute('aria-selected', 'true');
    expect(document.getElementById('sidebar-projects-panel')).not.toHaveAttribute('hidden');
    expect(localStorage.getItem('pi-web:v1:left-sidebar-tab')).toBe('projects');

    await user.click(outlineTab);

    expect(outlineTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('searchbox', { name: 'Search outline…' })).toBeInTheDocument();
    expect(document.getElementById('tree-container')).toBeInTheDocument();
    expect(screen.getAllByRole('button').length).toBeGreaterThanOrEqual(5);
  });

  it('keeps Projects selected when the session page remounts', () => {
    localStorage.setItem('pi-web:v1:left-sidebar-tab', 'projects');

    render(SessionTree);

    expect(screen.getByRole('tab', { name: 'Projects' })).toHaveAttribute('aria-selected', 'true');
    expect(document.getElementById('sidebar-projects-panel')).not.toHaveAttribute('hidden');
    expect(document.getElementById('sidebar-sessions-panel')).toHaveAttribute('hidden');
  });
});
