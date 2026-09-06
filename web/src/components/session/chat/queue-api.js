// Tiny HTTP client for the per-session chat queue (/api/chat/queue).
// QueueStore depends on this for hydration + mutations; the runtime glue
// (steer-queue.js) calls through it for the user actions that mutate the
// queue (enqueue, remove, pause).
//
// All methods return a Promise; failures throw so callers can fall back to
// "best-effort" behaviour (the UI shows an error toast but the panel stays
// usable).

export function createQueueApi({ sessionId, fetchImpl = fetch, FormDataImpl = FormData } = {}) {
  if (!sessionId) throw new Error('createQueueApi: sessionId is required');
  const base = `/api/chat/queue?id=${encodeURIComponent(sessionId)}`;
  const sendBase = `/api/chat/queue/send?id=${encodeURIComponent(sessionId)}`;

  async function readJSON(resp) {
    if (!resp.ok) {
      let detail = '';
      try {
        const body = await resp.json();
        detail = body?.error || '';
      } catch {
        /* ignore */
      }
      throw new Error(detail || `chat queue request failed (${resp.status})`);
    }
    return resp.json();
  }

  return {
    async list() {
      return readJSON(await fetchImpl(base, { headers: { Accept: 'application/json' } }));
    },
    // Text-only items go as JSON; anything with files is the same multipart
    // form /api/chat takes, so uploads are saved server-side at enqueue time.
    async add(message, displayText, { files = [], notBefore = '' } = {}) {
      if (files.length === 0) {
        return readJSON(
          await fetchImpl(base, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
            body: JSON.stringify({ message, displayText, notBefore: notBefore || '' }),
          }),
        );
      }
      const body = new FormDataImpl();
      body.set('message', message);
      body.set('displayText', displayText || '');
      if (notBefore) body.set('notBefore', notBefore);
      for (const file of files) body.append('images', file);
      return readJSON(
        await fetchImpl(base, { method: 'POST', headers: { Accept: 'application/json' }, body }),
      );
    },
    async reschedule(position, notBefore) {
      return readJSON(
        await fetchImpl(base, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ position, notBefore: notBefore || '' }),
        }),
      );
    },
    async sendNow(position) {
      return readJSON(
        await fetchImpl(sendBase, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ position }),
        }),
      );
    },
    async remove(position) {
      const url = `${base}&position=${encodeURIComponent(position)}`;
      return readJSON(await fetchImpl(url, { method: 'DELETE' }));
    },
    async setPaused(paused) {
      return readJSON(
        await fetchImpl(base, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ paused: !!paused }),
        }),
      );
    },
  };
}
