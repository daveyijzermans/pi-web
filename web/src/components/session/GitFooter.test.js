import { describe, expect, it, vi, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import GitFooter, {
  DRAFT_PR_PROMPT,
  COMMIT_PUSH_PROMPT,
  MERGE_PR_PROMPT,
} from './GitFooter.svelte';

const flush = () => new Promise((r) => setTimeout(r, 0));
const id = (x) => document.getElementById(x);

// GitFooter wires the composer's textarea (#pi-chat-message), which lives in
// <ChatComposer>, not in GitFooter itself — provide one for the prompt-insert
// assertions.
let textarea;
function renderFooter(gitApi, extras) {
  textarea = document.createElement('textarea');
  textarea.id = 'pi-chat-message';
  document.body.appendChild(textarea);
  return render(GitFooter, {
    props: { sessionId: 's', gitApi, windowImpl: window, ...extras },
  });
}

afterEach(() => {
  cleanup();
  textarea?.remove();
  textarea = undefined;
  vi.restoreAllMocks();
});

function makeFakeWindow() {
  const handlers = new Map();
  return {
    addEventListener(type, handler) {
      handlers.set(type, handler);
    },
    removeEventListener(type) {
      handlers.delete(type);
    },
    dispatchEvent(event) {
      const handler = handlers.get(event.type);
      if (handler) handler(event);
    },
  };
}

describe('GitFooter', () => {
  it('hides the git controls but keeps the bar visible (for btw) when the cwd is not a git repo', async () => {
    renderFooter({ getGitInfo: vi.fn().mockResolvedValue({ isRepo: false }) });
    await flush();
    expect(id('pi-git-bar').hidden).toBe(false);
    expect(id('pi-git-branch').hidden).toBe(true);
    expect(id('pi-git-pr').hidden).toBe(true);
  });

  it('feature branch, no PR -> primary Create PR (commit+push+create), only manual under the caret', async () => {
    renderFooter({
      getGitInfo: vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'feature/x',
        isDefault: false,
        hasChanges: true,
        prUrl: '',
      }),
    });
    await flush();
    expect(id('pi-git-primary-label').textContent).toBe('Create PR');
    expect(id('pi-git-caret').hidden).toBe(false);
    expect(id('pi-git-pr-manual').hidden).toBe(false);
    expect(id('pi-git-pr-commit').hidden).toBe(true);
    expect(id('pi-git-pr-view').hidden).toBe(true);
    expect(id('pi-git-pr-merge').hidden).toBe(true);
    id('pi-git-primary').click();
    expect(id('pi-chat-message').value).toBe(DRAFT_PR_PROMPT);
  });

  it('feature branch, open PR + local changes -> primary Commit & push, secondary view + merge', async () => {
    renderFooter({
      getGitInfo: vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'feature/x',
        isDefault: false,
        hasChanges: true,
        prUrl: 'https://github.com/o/r/pull/42',
      }),
    });
    await flush();
    expect(id('pi-git-primary-label').textContent).toBe('Commit & push');
    expect(id('pi-git-pr-view').hidden).toBe(false);
    expect(id('pi-git-pr-merge').hidden).toBe(false);
    expect(id('pi-git-pr-draft').hidden).toBe(true);
    expect(id('pi-git-pr-manual').hidden).toBe(true);
    id('pi-git-primary').click();
    expect(id('pi-chat-message').value).toBe(COMMIT_PUSH_PROMPT);
  });

  it('feature branch, open PR + no changes -> primary View PR, secondary merge only (no commit)', async () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);
    renderFooter({
      getGitInfo: vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'feature/x',
        isDefault: false,
        hasChanges: false,
        prUrl: 'https://github.com/o/r/pull/42',
      }),
    });
    await flush();
    expect(id('pi-git-primary-label').textContent.trim()).toBe('View PR');
    expect(id('pi-git-primary-label').querySelector('svg')).not.toBeNull();
    expect(id('pi-git-pr-merge').hidden).toBe(false);
    expect(id('pi-git-pr-commit').hidden).toBe(true);
    id('pi-git-primary').click();
    expect(open).toHaveBeenCalledWith('https://github.com/o/r/pull/42', '_blank', 'noopener');
  });

  it('default branch + changes -> primary Commit & push, no caret', async () => {
    renderFooter({
      getGitInfo: vi
        .fn()
        .mockResolvedValue({ isRepo: true, branch: 'main', isDefault: true, hasChanges: true }),
    });
    await flush();
    expect(id('pi-git-primary-label').textContent).toBe('Commit & push');
    expect(id('pi-git-caret').hidden).toBe(true);
    id('pi-git-primary').click();
    expect(id('pi-chat-message').value).toBe(COMMIT_PUSH_PROMPT);
  });

  it('default branch + no changes -> action control hidden, only the branch shows', async () => {
    renderFooter({
      getGitInfo: vi
        .fn()
        .mockResolvedValue({ isRepo: true, branch: 'main', isDefault: true, hasChanges: false }),
    });
    await flush();
    expect(id('pi-git-bar').hidden).toBe(false);
    expect(id('pi-git-pr').hidden).toBe(true);
    expect(id('pi-git-primary').hidden).toBe(true);
  });

  it('menu items run their actions (Merge PR injects merge prompt)', async () => {
    renderFooter({
      getGitInfo: vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'feature/x',
        isDefault: false,
        hasChanges: true,
        prUrl: 'https://github.com/o/r/pull/42',
      }),
    });
    await flush();
    id('pi-git-pr-merge').click();
    expect(id('pi-chat-message').value).toBe(MERGE_PR_PROMPT);
  });

  it('keeps the status container hidden when all counts are zero', async () => {
    renderFooter({
      getGitInfo: vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'main',
        isDefault: true,
        hasChanges: false,
        dirty: false,
        modified: 0,
        added: 0,
        deleted: 0,
        untracked: 0,
        ahead: 0,
        behind: 0,
      }),
    });
    await flush();
    expect(id('pi-git-status').hidden).toBe(true);
  });

  describe('compact button', () => {
    it('sits next to the btw button in the footer bar', async () => {
      renderFooter({ getGitInfo: vi.fn().mockResolvedValue({ isRepo: false }) });
      await flush();
      const compact = id('pi-compact-button');
      const btw = id('pi-btw-button');
      expect(compact).not.toBeNull();
      expect(btw).not.toBeNull();
      expect(compact.parentElement).toBe(btw.parentElement);
      expect(compact.nextElementSibling).toBe(btw);
    });

    it('posts to /api/chat/compact and stays busy (202) until a reload lands', async () => {
      const fetchMock = vi
        .fn()
        .mockResolvedValue({ ok: true, json: async () => ({ ok: true, status: 'queued' }) });
      vi.stubGlobal('fetch', fetchMock);
      renderFooter({ getGitInfo: vi.fn().mockResolvedValue({ isRepo: false }) });
      await flush();

      id('pi-compact-button').click();
      await flush();
      await flush();

      expect(fetchMock).toHaveBeenCalledWith('/api/chat/compact?id=s', { method: 'POST' });
      // Fire-and-forget: the 202 leaves the button busy; completion is by SSE.
      expect(id('pi-compact-button').disabled).toBe(true);
      expect(id('pi-compact-label').textContent).toBe('Compacting…');

      window.dispatchEvent(new CustomEvent('pi-session-reload'));
      await flush();

      expect(id('pi-compact-button').disabled).toBe(false);
      expect(id('pi-compact-label').textContent).toBe('compact');
    });

    it('clears busy and shows the error from a pi-compact-error SSE event', async () => {
      const fetchMock = vi
        .fn()
        .mockResolvedValue({ ok: true, json: async () => ({ ok: true, status: 'queued' }) });
      vi.stubGlobal('fetch', fetchMock);
      renderFooter({ getGitInfo: vi.fn().mockResolvedValue({ isRepo: false }) });
      await flush();

      id('pi-compact-button').click();
      await flush();
      await flush();
      expect(id('pi-compact-button').disabled).toBe(true);

      window.dispatchEvent(
        new CustomEvent('pi-compact-error', {
          detail: { error: 'Nothing to compact (session too small)' },
        }),
      );
      await flush();

      expect(id('pi-compact-button').title).toBe('Nothing to compact (session too small)');
      expect(id('pi-compact-button').disabled).toBe(false);
      expect(id('pi-compact-label').textContent).toBe('compact');
    });

    it('shows the error in the title when the request is rejected synchronously', async () => {
      const fetchMock = vi.fn().mockResolvedValue({
        ok: false,
        json: async () => ({ error: 'chat unavailable' }),
      });
      vi.stubGlobal('fetch', fetchMock);
      renderFooter({ getGitInfo: vi.fn().mockResolvedValue({ isRepo: false }) });
      await flush();

      id('pi-compact-button').click();
      await flush();
      await flush();

      expect(id('pi-compact-button').title).toBe('chat unavailable');
      expect(id('pi-compact-button').disabled).toBe(false);
      expect(id('pi-compact-label').textContent).toBe('compact');
    });
  });

  describe('refresh polling', () => {
    it('starts a periodic poll that re-fetches git info', async () => {
      const getGitInfo = vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'main',
        isDefault: true,
        hasChanges: false,
      });
      let intervalFn = null;
      const intervalId = Symbol('interval');
      renderFooter(
        { getGitInfo },
        {
          setIntervalImpl: (fn, _ms) => {
            intervalFn = fn;
            return intervalId;
          },
          clearIntervalImpl: vi.fn(),
        },
      );
      await flush();

      // Initial fetch on mount.
      expect(getGitInfo).toHaveBeenCalledTimes(1);

      // Trigger the poll callback.
      intervalFn();
      await flush();
      expect(getGitInfo).toHaveBeenCalledTimes(2);
    });

    it('refreshes on pi-session-reload event', async () => {
      const fakeWindow = makeFakeWindow();
      const getGitInfo = vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'main',
        isDefault: true,
        hasChanges: false,
      });
      renderFooter(
        { getGitInfo },
        {
          windowImpl: fakeWindow,
          setIntervalImpl: () => 0,
          clearIntervalImpl: vi.fn(),
        },
      );
      await flush();
      expect(getGitInfo).toHaveBeenCalledTimes(1);

      // Fire session reload.
      fakeWindow.dispatchEvent({ type: 'pi-session-reload' });
      await flush();
      expect(getGitInfo).toHaveBeenCalledTimes(2);
    });

    it('cleanup disposes the interval and removes the reload listener', async () => {
      const fakeWindow = makeFakeWindow();
      const clearIntervalMock = vi.fn();
      const getGitInfo = vi.fn().mockResolvedValue({
        isRepo: true,
        branch: 'main',
        isDefault: true,
        hasChanges: false,
      });
      const { unmount } = renderFooter(
        { getGitInfo },
        {
          windowImpl: fakeWindow,
          setIntervalImpl: (_fn, _ms) => {
            return 42;
          },
          clearIntervalImpl: clearIntervalMock,
        },
      );
      await flush();

      // Confirm reload listener is active.
      fakeWindow.dispatchEvent({ type: 'pi-session-reload' });
      await flush();
      expect(getGitInfo).toHaveBeenCalledTimes(2);

      // Unmount.
      unmount();

      // Interval should be cleared.
      expect(clearIntervalMock).toHaveBeenCalledWith(42);

      // Reload listener should be removed — dispatching again should not trigger.
      fakeWindow.dispatchEvent({ type: 'pi-session-reload' });
      await flush();
      expect(getGitInfo).toHaveBeenCalledTimes(2);
    });
  });
});
