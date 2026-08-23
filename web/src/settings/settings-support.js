import { configureSettingsSync, hydrateSettings, writeSetting } from '../shared/settings-store.js';

export async function loadSettings({ windowImpl = window } = {}) {
  const fetchImpl = windowImpl.fetch ? windowImpl.fetch.bind(windowImpl) : undefined;
  configureSettingsSync({ fetchImpl });
  const storage = windowImpl.localStorage;
  return (await hydrateSettings({ fetchImpl, storage })) || {};
}

export function valueFor(settings, key, fallback = '', { storage = localStorage } = {}) {
  if (settings && key in settings) return settings[key];
  try {
    const stored = storage?.getItem(key);
    if (stored != null) return stored;
  } catch {}
  return fallback;
}

export function boolFor(settings, key, fallback = false, opts = {}) {
  return String(valueFor(settings, key, fallback ? 'true' : 'false', opts)) === 'true';
}

export function persistSetting(key, value, { storage = localStorage } = {}) {
  writeSetting(key, value, { storage });
}

export async function fetchModelGroups({ fetchImpl = fetch } = {}) {
  let models;
  try {
    const resp = await fetchImpl('/api/models', { headers: { Accept: 'application/json' } });
    if (!resp.ok) return [];
    const data = await resp.json();
    models = Array.isArray(data?.models) ? data.models : null;
  } catch {
    return [];
  }
  if (!models?.length) return [];

  const byProvider = new Map();
  for (const m of models) {
    const id = m.id || m.modelId || '';
    const provider = m.provider || '';
    if (!id || !provider) continue;

    // Strip pi's inline base-URL suffix (everything after the first '=')
    const key = provider.split('=')[0];
    let group = byProvider.get(key);
    if (!group) {
      group = { provider: key, models: new Map() };
      byProvider.set(key, group);
    }
    // Dedupe by id within the group
    if (!group.models.has(id)) {
      group.models.set(id, { id, name: m.name || id, value: `${key}/${id}` });
    }
  }

  return Array.from(byProvider.keys())
    .sort((a, b) => a.localeCompare(b))
    .map((key) => {
      const { provider, models } = byProvider.get(key);
      return { provider, models: Array.from(models.values()) };
    });
}
