import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import SessionTree from './SessionTree.svelte';

describe('SessionTree', () => {
  it('tabs between project sessions and the message outline', async () => {
    const user = userEvent.setup();
    render(SessionTree);

    const sessionsTab = screen.getByRole('tab', { name: 'Sessions' });
    const outlineTab = screen.getByRole('tab', { name: 'Outline' });
    expect(sessionsTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('searchbox', { name: 'Search project sessions…' })).toBeInTheDocument();

    await user.click(outlineTab);

    expect(outlineTab).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('searchbox', { name: 'Search outline…' })).toBeInTheDocument();
    expect(document.getElementById('tree-container')).toBeInTheDocument();
    expect(screen.getAllByRole('button').length).toBeGreaterThanOrEqual(5);
  });
});
