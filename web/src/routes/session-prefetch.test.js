import { afterEach, describe, expect, it } from 'vitest';
import {
  consumeSessionPrefetch,
  prefetchSession,
  resetSessionPrefetch,
} from './session-prefetch.js';

afterEach(() => resetSessionPrefetch());

describe('session-prefetch', () => {
  it('starts an /api/session fetch and lets consume await the same promise', async () => {
    const calls = [];
    const fetchImpl = async (url) => {
      calls.push(url);
      return { ok: true, json: async () => ({ name: 'Prefetched' }) };
    };

    prefetchSession('s.jsonl', { fetchImpl });
    const data = await consumeSessionPrefetch('s.jsonl');

    expect(calls).toEqual(['/api/session?id=s.jsonl&paginate=1']);
    expect(data).toEqual({ name: 'Prefetched' });
  });

  it('dedupes concurrent prefetches for the same id', () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls++;
      return { ok: true, json: async () => ({}) };
    };
    prefetchSession('s.jsonl', { fetchImpl });
    prefetchSession('s.jsonl', { fetchImpl });
    expect(calls).toBe(1);
  });

  it('removes the entry once consumed so the next call goes back to the network', async () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls++;
      return { ok: true, json: async () => ({}) };
    };
    prefetchSession('s.jsonl', { fetchImpl });
    await consumeSessionPrefetch('s.jsonl');
    expect(consumeSessionPrefetch('s.jsonl')).toBe(null);
    expect(calls).toBe(1);
  });

  it('ignores a hover older than the TTL so the route refetches', async () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls++;
      return { ok: true, json: async () => ({ name: 'Stale' }) };
    };
    prefetchSession('s.jsonl', { fetchImpl, now: () => 0 });
    await Promise.resolve();
    expect(consumeSessionPrefetch('s.jsonl', { now: () => 10_001 })).toBe(null);
    expect(calls).toBe(1);
  });

  it('restarts the request when a stale entry is hovered again', () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls++;
      return { ok: true, json: async () => ({}) };
    };
    prefetchSession('s.jsonl', { fetchImpl, now: () => 0 });
    prefetchSession('s.jsonl', { fetchImpl, now: () => 10_001 });
    expect(calls).toBe(2);
  });

  it('returns null when there is no prefetch for the id', () => {
    expect(consumeSessionPrefetch('nope')).toBe(null);
  });

  it('drops a rejected prefetch so callers fall back to a fresh fetch', async () => {
    const fetchImpl = async () => ({ ok: false, status: 500 });
    prefetchSession('s.jsonl', { fetchImpl });
    const promise = consumeSessionPrefetch('s.jsonl');
    expect(promise).not.toBe(null);
    await expect(promise).rejects.toThrow();
    // Entry was already removed when consumed; another consume returns null.
    expect(consumeSessionPrefetch('s.jsonl')).toBe(null);
  });
});
