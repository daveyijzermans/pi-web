import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import SendLater from './SendLater.svelte';
import { QueueStore } from './queue-store.svelte.js';

afterEach(() => cleanup());

function makeStore({ hasContent = true } = {}) {
  const store = new QueueStore();
  store.actions.hasComposerContent = () => hasContent;
  store.actions.enqueueLater = vi.fn(() => true);
  return store;
}

const usage = {
  providers: [
    {
      provider: 'anthropic',
      label: 'Anthropic',
      windows: [
        {
          key: 'five_hour',
          label: '5h',
          utilization: 40,
          resetsAt: new Date(Date.now() + 2 * 3600 * 1000).toISOString(),
        },
      ],
    },
  ],
};

describe('SendLater', () => {
  it('schedules the composed turn at the picked time', async () => {
    const store = makeStore();
    const fetchImpl = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ providers: [] }), { status: 200 })),
    );
    render(SendLater, { props: { store, fetchImpl } });
    await fireEvent.click(screen.getByRole('button', { name: 'Send later' }));
    const input = await screen.findByLabelText('Send at');
    await fireEvent.input(input, { target: { value: '2030-01-02T03:04' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Schedule' }));
    expect(store.actions.enqueueLater).toHaveBeenCalledWith(
      new Date('2030-01-02T03:04').toISOString(),
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('refuses past times and empty composers', async () => {
    const store = makeStore({ hasContent: false });
    const fetchImpl = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ providers: [] }), { status: 200 })),
    );
    render(SendLater, { props: { store, fetchImpl } });
    await fireEvent.click(screen.getByRole('button', { name: 'Send later' }));
    const input = await screen.findByLabelText('Send at');
    await fireEvent.input(input, { target: { value: '2001-01-01T00:00' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Schedule' }));
    expect(screen.getByText('Pick a time in the future')).toBeInTheDocument();
    await fireEvent.input(input, { target: { value: '2030-01-01T00:00' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Schedule' }));
    expect(screen.getByText('Type a message or attach a file first')).toBeInTheDocument();
    expect(store.actions.enqueueLater).not.toHaveBeenCalled();
  });

  it('offers "when window resets" only when a provider reports a reset time', async () => {
    const store = makeStore();
    const fetchImpl = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify(usage), { status: 200 })),
    );
    render(SendLater, { props: { store, fetchImpl } });
    await fireEvent.click(screen.getByRole('button', { name: 'Send later' }));
    const preset = await screen.findByRole('button', { name: 'When window resets' });
    await fireEvent.click(preset);
    const input = screen.getByLabelText('Send at');
    const picked = Date.parse(input.value);
    const reset = Date.parse(usage.providers[0].windows[0].resetsAt);
    expect(picked).toBeGreaterThan(reset);
    expect(picked - reset).toBeLessThan(2 * 60 * 1000);
    await fireEvent.click(screen.getByRole('button', { name: 'Schedule' }));
    await waitFor(() => expect(store.actions.enqueueLater).toHaveBeenCalled());
  });
});
