import { describe, expect, it } from 'vitest';
import { SessionDataModel } from './session-data.svelte.js';

// A small two-branch session: root → (old leaf) and root → mid → leaf.
const entries = [
  {
    id: 'root',
    timestamp: '2026-01-01T00:00:00Z',
    type: 'message',
    message: { role: 'user', content: 'start' },
  },
  {
    id: 'old',
    parentId: 'root',
    timestamp: '2026-01-01T00:01:00Z',
    type: 'message',
    message: { role: 'assistant', content: 'old branch' },
  },
  {
    id: 'mid',
    parentId: 'root',
    timestamp: '2026-01-01T00:02:00Z',
    type: 'message',
    message: { role: 'assistant', content: 'mid' },
  },
  {
    id: 'leaf',
    parentId: 'mid',
    timestamp: '2026-01-01T00:03:00Z',
    type: 'message',
    message: { role: 'user', content: 'tell me about widgets' },
  },
];

function model(extra = {}) {
  return new SessionDataModel({ entries, header: { cwd: '/x' }, leafId: 'leaf', ...extra });
}

describe('SessionDataModel', () => {
  it('hydrates raw data and view state from a plain payload', () => {
    const m = model();
    expect(m.entries).toHaveLength(4);
    expect(m.header.cwd).toBe('/x');
    expect(m.currentLeafId).toBe('leaf');
    expect(m.currentTargetId).toBe('leaf');
  });

  it('derives lookups (byId / toolCallMap / labelMap)', () => {
    const m = model();
    expect(m.byId.get('mid').parentId).toBe('root');
    expect([...m.byId.keys()]).toEqual(['root', 'old', 'mid', 'leaf']);
  });

  it('derives the tree from entries', () => {
    const m = model();
    expect(m.tree.map((n) => n.entry.id)).toEqual(['root']);
    expect(m.tree[0].children.map((n) => n.entry.id)).toEqual(['old', 'mid']);
  });

  it('derives the active path from the current leaf', () => {
    const m = model();
    expect([...m.activePathIds].sort()).toEqual(['leaf', 'mid', 'root']);
    // 'old' is on the other branch, so it is not on the active path.
    expect(m.activePathIds.has('old')).toBe(false);
  });

  it('recomputes the active path when navigating', () => {
    const m = model();
    m.navigateTo('old');
    expect(m.currentLeafId).toBe('old');
    expect([...m.activePathIds].sort()).toEqual(['old', 'root']);
    expect(m.activePathIds.has('leaf')).toBe(false);
  });

  it('reactively recomputes derived state when entries change (live update)', () => {
    const m = model();
    expect(m.byId.has('leaf2')).toBe(false);

    m.applyLiveUpdate({
      entries: [
        ...entries,
        {
          id: 'leaf2',
          parentId: 'leaf',
          timestamp: '2026-01-01T00:04:00Z',
          type: 'message',
          message: { role: 'assistant', content: 'widgets are great' },
        },
      ],
      header: { cwd: '/x' },
      leafId: 'leaf2',
    });

    expect(m.byId.has('leaf2')).toBe(true);
    expect(m.nodeMap.get('leaf').children.map((n) => n.entry.id)).toEqual(['leaf2']);
    // view state preserved across a live update (we were on 'leaf')
    expect(m.currentLeafId).toBe('leaf');
  });

  it('reconcile() merges new entries in place and advances the active leaf', () => {
    const m = model();
    m.navigateTo('leaf');
    m.reconcile([
      ...entries,
      {
        id: 'leaf2',
        parentId: 'leaf',
        timestamp: '2026-01-01T00:04:00Z',
        type: 'message',
        message: { role: 'assistant', content: 'more' },
      },
    ]);
    expect(m.byId.has('leaf2')).toBe(true);
    // active leaf follows to the newest descendant of where we were.
    expect(m.currentLeafId).toBe('leaf2');
    expect(m.leafId).toBe('leaf2');
  });

  it('reconcile() ignores non-array input', () => {
    const m = model();
    m.reconcile(undefined);
    expect(m.entries).toHaveLength(4);
  });

  it('reconcile() prepends earlier entries without moving the active leaf off-branch', () => {
    const m = model();
    m.navigateTo('old');
    m.reconcile(entries);
    // staying on 'old' (a leaf), the newest descendant is itself.
    expect(m.currentLeafId).toBe('old');
  });

  it('reconcile() skips an out-of-order (stale) reload that lacks the newest entry', () => {
    const m = model();
    // Two reloads raced and the older response landed last: it overlaps our
    // state but is missing the newest entry. Applying it would vanish 'leaf'.
    m.reconcile(entries.slice(0, 2));
    expect(m.entries).toHaveLength(4);
    expect(m.byId.has('leaf')).toBe(true);
  });

  it('reconcile() keeps load-earlier prefixes when a smaller tail window arrives', () => {
    const m = model();
    // The user loaded an earlier window: model now holds MORE than the server
    // tail window. A live reload delivering the (smaller) tail window plus a
    // new entry must merge — not be skipped, and not drop the earlier prefix.
    const tailWindow = [
      ...entries.slice(2), // mid, leaf — the window the server still returns
      {
        id: 'leaf2',
        parentId: 'leaf',
        timestamp: '2026-01-01T00:04:00Z',
        type: 'message',
        message: { role: 'assistant', content: 'new turn' },
      },
    ];
    m.reconcile(tailWindow);
    expect(m.entries.map((e) => e.id)).toEqual(['root', 'old', 'mid', 'leaf', 'leaf2']);
    expect(m.byId.has('root')).toBe(true);
    expect(m.byId.has('leaf2')).toBe(true);
  });

  it('reconcile() appends a disjoint newer window after the existing entries', () => {
    const m = model();
    // The tail window slid entirely past what we hold (many entries landed at
    // once). Nothing overlaps; the new window belongs after our entries.
    m.reconcile([
      {
        id: 'far1',
        parentId: 'leaf',
        timestamp: '2026-01-01T00:05:00Z',
        type: 'message',
        message: { role: 'assistant', content: 'far away' },
      },
    ]);
    expect(m.entries.map((e) => e.id)).toEqual(['root', 'old', 'mid', 'leaf', 'far1']);
  });

  it('reconcile() skips no-op reloads (identical data guard)', () => {
    const m = model();
    const before = m.entries;
    m.reconcile(entries);
    // entries array should be the same reference — no splice triggered
    expect(m.entries).toBe(before);
    expect(m.entries).toHaveLength(4);
  });

  it('derives the ordered active path (root→leaf)', () => {
    const m = model();
    expect(m.activePath.map((e) => e.id)).toEqual(['root', 'mid', 'leaf']);
    m.navigateTo('old');
    expect(m.activePath.map((e) => e.id)).toEqual(['root', 'old']);
  });

  // Issue #123: after an unarchive, a model_change is written with a null parentId
  // mid-session. Left unlinked it forks the tree, so the earlier conversation drops
  // off the active path and the session looks like it lost all prior messages.
  it('keeps earlier history on the active path after an orphan model_change (unarchive)', () => {
    const unarchived = [
      { id: 'h1', type: 'message', timestamp: '2026-01-01T00:00:00Z', message: { role: 'user' } },
      {
        id: 'h2',
        parentId: 'h1',
        type: 'message',
        timestamp: '2026-01-01T00:01:00Z',
        message: { role: 'assistant' },
      },
      { type: 'archive', archived: false, timestamp: '2026-01-01T00:02:00Z' },
      { id: 'mc', type: 'model_change', parentId: null, timestamp: '2026-01-01T00:03:00Z' },
      {
        id: 'nu',
        parentId: 'mc',
        type: 'message',
        timestamp: '2026-01-01T00:04:00Z',
        message: { role: 'user' },
      },
      {
        id: 'na',
        parentId: 'nu',
        type: 'message',
        timestamp: '2026-01-01T00:05:00Z',
        message: { role: 'assistant' },
      },
    ];
    const m = new SessionDataModel({ entries: unarchived, header: { cwd: '/x' }, leafId: 'na' });
    expect(m.activePath.map((e) => e.id)).toEqual(['h1', 'h2', 'mc', 'nu', 'na']);
    // reconcile (live reload path) must repair it too: the model holds the
    // pre-unarchive history and a reload delivers the full set including the
    // orphaned model_change.
    const m2 = new SessionDataModel({
      entries: unarchived.slice(0, 2),
      header: { cwd: '/x' },
      leafId: 'h2',
    });
    m2.reconcile(unarchived);
    expect(m2.activePath.map((e) => e.id)).toContain('h1');
    expect(m2.activePath.map((e) => e.id)).toContain('h2');
    expect(m2.activePath.map((e) => e.id)).toContain('na');
  });

  it('applies the search filter reactively', () => {
    const m = model();
    const unfiltered = m.filteredNodes.length;
    m.searchQuery = 'widgets';
    const filtered = m.filteredNodes.map((f) => f.node.entry.id);
    expect(filtered).toContain('leaf'); // matches "tell me about widgets"
    expect(m.filteredNodes.length).toBeLessThan(unfiltered);
  });

  it('builds a reactive model from an embedded payload via fromPayload', () => {
    const m = SessionDataModel.fromPayload(
      { header: {}, entries, leafId: 'leaf' },
      new URLSearchParams('targetId=mid'),
    );
    expect(m.currentLeafId).toBe('leaf');
    expect(m.currentTargetId).toBe('mid');
  });
});
