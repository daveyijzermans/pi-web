import { describe, expect, it, vi } from 'vitest';
import { createQueueApi } from './queue-api.js';

describe('createQueueApi attachments + scheduling', () => {
  it('posts multipart when files are present and JSON otherwise', async () => {
    const fetchImpl = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ position: 1 }), { status: 201 })),
    );
    const api = createQueueApi({ sessionId: 's', fetchImpl });
    await api.add('hi', 'hi', { notBefore: '2030-01-01T00:00:00Z' });
    expect(JSON.parse(fetchImpl.mock.calls[0][1].body)).toEqual({
      message: 'hi',
      displayText: 'hi',
      notBefore: '2030-01-01T00:00:00Z',
    });

    const file = new File(['x'], 'shot.png', { type: 'image/png' });
    await api.add('look', 'look', { files: [file], notBefore: '' });
    const body = fetchImpl.mock.calls[1][1].body;
    expect(body).toBeInstanceOf(FormData);
    expect(body.get('message')).toBe('look');
    expect(body.getAll('images')).toHaveLength(1);
    expect(body.has('notBefore')).toBe(false);
  });

  it('reschedule PATCHes position + notBefore and sendNow POSTs to /send', async () => {
    const fetchImpl = vi.fn(() =>
      Promise.resolve(new Response(JSON.stringify({ ok: true }), { status: 200 })),
    );
    const api = createQueueApi({ sessionId: 's', fetchImpl });
    await api.reschedule(3, '');
    expect(fetchImpl.mock.calls[0][1].method).toBe('PATCH');
    expect(JSON.parse(fetchImpl.mock.calls[0][1].body)).toEqual({ position: 3, notBefore: '' });
    await api.sendNow(3);
    expect(fetchImpl.mock.calls[1][0]).toBe('/api/chat/queue/send?id=s');
    expect(JSON.parse(fetchImpl.mock.calls[1][1].body)).toEqual({ position: 3 });
  });
});
