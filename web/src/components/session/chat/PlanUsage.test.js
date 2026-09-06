import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';
import PlanUsage from './PlanUsage.svelte';

afterEach(() => cleanup());

describe('PlanUsage', () => {
  it('renders nothing without providers and a compact label with them', async () => {
    const empty = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ providers: [] }), { status: 200 })),
    );
    const { container, unmount } = render(PlanUsage, { props: { fetchImpl: empty } });
    await new Promise((r) => setTimeout(r, 0));
    expect(container.querySelector('.pi-chat-plan-usage')).toBeNull();
    unmount();

    const full = vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            providers: [
              {
                provider: 'anthropic',
                label: 'Anthropic',
                windows: [
                  { key: 'five_hour', label: '5h', utilization: 33.2 },
                  { key: 'seven_day', label: '7d', utilization: 13 },
                ],
              },
            ],
          }),
          { status: 200 },
        ),
      ),
    );
    render(PlanUsage, { props: { fetchImpl: full } });
    expect(await screen.findByText('5h 33% · 7d 13%')).toBeInTheDocument();
  });
});
