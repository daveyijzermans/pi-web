// In-memory prefetch cache for /api/session payloads, used by SessionCard hover
// to start the request before the route actually mounts. loadSessionPageState
// consumes the in-flight promise instead of issuing a fresh fetch when present.
//
// A hover that is never followed by a click leaves a resolved promise behind,
// and a resolved promise is a frozen payload. Entries therefore carry the time
// the request started and expire after TTL_MS, so only a hover that leads
// straight into navigation is reused; anything older falls back to the network
// and the route sees the session's current turns.

const inflight = new Map();
const MAX_ENTRIES = 16;
const TTL_MS = 10_000;

export function prefetchSession(id, { fetchImpl = fetch, now = Date.now } = {}) {
  if (!id) return;
  if (inflight.has(id)) {
    if (now() - inflight.get(id).at <= TTL_MS) return;
    inflight.delete(id);
  }
  if (inflight.size >= MAX_ENTRIES) {
    const oldest = inflight.keys().next().value;
    if (oldest) inflight.delete(oldest);
  }
  const promise = fetchImpl(`/api/session?id=${encodeURIComponent(id)}&paginate=1`, {
    headers: { Accept: 'application/json' },
  })
    .then((resp) => {
      if (!resp.ok) {
        inflight.delete(id);
        throw new Error(resp.status === 404 ? 'not found' : 'load failed');
      }
      return resp.json();
    })
    .catch((err) => {
      inflight.delete(id);
      throw err;
    });
  // Swallow uncaught-rejection warnings: consumeSessionPrefetch handlers add a
  // proper catcher when they read this back.
  promise.catch(() => {});
  inflight.set(id, { promise, at: now() });
}

export function consumeSessionPrefetch(id, { now = Date.now } = {}) {
  if (!id) return null;
  const entry = inflight.get(id);
  if (!entry) return null;
  inflight.delete(id);
  if (now() - entry.at > TTL_MS) return null;
  return entry.promise;
}

export function resetSessionPrefetch() {
  inflight.clear();
}
