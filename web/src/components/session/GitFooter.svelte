<script module>
  // Prompt text the split button injects into the composer for each git action.
  export const DRAFT_PR_PROMPT =
    'Commit any uncommitted changes on this branch with a clear message, push ' +
    'the branch to the remote, then open a draft pull request with ' +
    '`gh pr create --draft`. Review the diff first (`git status`, `git diff`, ' +
    '`git log`) and write a clear PR title, a description summarizing what ' +
    'changed and why, and a short test plan.';

  export const COMMIT_PUSH_PROMPT =
    'Review the current changes (run `git status` and `git diff`), then stage ' +
    'and commit them with a clear message summarizing what changed and why, and ' +
    'push to the remote.';

  export const MERGE_PR_PROMPT =
    'Merge the pull request for this branch once its checks are green: run ' +
    '`gh pr merge` (use a squash merge unless the project prefers otherwise) and ' +
    'delete the branch after merging.';
</script>

<script>
  import { onMount } from 'svelte';
  import { icon, ChevronDown, ExternalLink, ArrowUp, ArrowDown } from '../../shared/icons.js';
  import { t } from '../../shared/i18n.js';
  import * as defaultGitApi from '../../session/chat/git-api.js';
  import { openDirtyFiles } from '../../session/session-modals.svelte.js';
  import CompactButton from './CompactButton.svelte';

  // The branch indicator + smart git action control beneath the chat composer.
  // The bar stays visible (even outside a git repo) because it also hosts the
  // always-available btw button. gitApi is injectable for tests.
  const GIT_REFRESH_MS = 15_000;
  let {
    sessionId = '',
    gitApi = defaultGitApi,
    windowImpl,
    setIntervalImpl,
    clearIntervalImpl,
  } = $props();

  onMount(() => {
    const documentImpl = document;
    const wi = windowImpl ?? window;
    const si = setIntervalImpl ?? wi.setInterval.bind(wi);
    const ci = clearIntervalImpl ?? wi.clearInterval.bind(wi);
    const bar = documentImpl.getElementById('pi-git-bar');
    if (!bar || !gitApi) return;

    const cleanups = [];
    const on = (host, type, handler, opts) => {
      host.addEventListener(type, handler, opts);
      cleanups.push(() => host.removeEventListener(type, handler, opts));
    };

    const branchWrap = documentImpl.getElementById('pi-git-branch');
    const nameEl = documentImpl.getElementById('pi-git-branch-name');
    const statusEl = documentImpl.getElementById('pi-git-status');

    let isDirty = false;
    const prWrap = documentImpl.getElementById('pi-git-pr');
    const primaryBtn = documentImpl.getElementById('pi-git-primary');
    const primaryLabel = documentImpl.getElementById('pi-git-primary-label');
    const caretBtn = documentImpl.getElementById('pi-git-caret');
    const prMenu = documentImpl.getElementById('pi-git-pr-menu');
    const items = {
      view: documentImpl.getElementById('pi-git-pr-view'),
      draft: documentImpl.getElementById('pi-git-pr-draft'),
      manual: documentImpl.getElementById('pi-git-pr-manual'),
      merge: documentImpl.getElementById('pi-git-pr-merge'),
      commit: documentImpl.getElementById('pi-git-pr-commit'),
    };

    let prCreateUrl = '';
    let existingPrUrl = '';
    let primaryAction = () => {};

    const show = (el, visible) => {
      if (el) el.hidden = !visible;
    };

    function insertPrompt(text) {
      const textarea = documentImpl.getElementById('pi-chat-message');
      if (!textarea) return;
      textarea.value = text;
      const EventCtor = windowImpl.Event || (typeof Event !== 'undefined' ? Event : null);
      if (EventCtor) textarea.dispatchEvent(new EventCtor('input', { bubbles: true }));
      if (typeof textarea.focus === 'function') textarea.focus();
    }
    function openUrl(url) {
      if (url && typeof windowImpl.open === 'function') windowImpl.open(url, '_blank', 'noopener');
    }

    // Each action is { label, run, external? }. `external` actions open a URL in
    // a new tab, so their label gets a trailing external-link icon. The plan
    // picks one primary plus a list of secondary actions shown under the caret.
    const ACTIONS = {
      draft: { label: t('git.createPr'), run: () => insertPrompt(DRAFT_PR_PROMPT) },
      manual: { label: t('git.createPrManually'), external: true, run: () => openUrl(prCreateUrl) },
      view: { label: t('git.viewPr'), external: true, run: () => openUrl(existingPrUrl) },
      merge: { label: t('git.mergePr'), run: () => insertPrompt(MERGE_PR_PROMPT) },
      commit: { label: t('git.commitPush'), run: () => insertPrompt(COMMIT_PUSH_PROMPT) },
    };

    // Decide the primary action + secondary list from the current git state.
    // "Commit & push" only appears when there is actually something to push.
    function planActions({ isDefault, hasPr, hasChanges }) {
      if (isDefault) {
        return { primary: hasChanges ? 'commit' : null, secondary: [] };
      }
      if (!hasPr) {
        return { primary: 'draft', secondary: ['manual'] };
      }
      if (hasChanges) return { primary: 'commit', secondary: ['view', 'merge'] };
      return { primary: 'view', secondary: ['merge'] };
    }

    function applyInfo(info) {
      if (!info || !info.isRepo || !info.branch) {
        // Not a git repo: hide the git controls but keep the bar itself visible,
        // since it also hosts the always-available btw button.
        show(branchWrap, false);
        show(prWrap, false);
        bar.hidden = false;
        return;
      }
      show(branchWrap, true);
      prCreateUrl = info.prCreateUrl || '';
      existingPrUrl = info.prUrl || '';
      if (nameEl) nameEl.textContent = info.branch;
      if (statusEl) {
        isDirty = !!info.dirty;
        statusEl.replaceChildren();

        const addBadge = (cls, text, title) => {
          const span = documentImpl.createElement('span');
          span.className = `pi-git-status-badge ${cls}`;
          span.textContent = text;
          span.title = title;
          statusEl.appendChild(span);
        };
        const addCommitBadge = (cls, iconNode, n, title) => {
          const span = documentImpl.createElement('span');
          span.className = `pi-git-status-badge ${cls}`;
          span.innerHTML = icon(iconNode, { size: 11 }) + String(n);
          span.title = title;
          statusEl.appendChild(span);
        };

        if (info.modified > 0)
          addBadge(
            'pi-git-status-modified',
            'M ' + info.modified,
            t('git.modifiedFiles').replace('{n}', String(info.modified)),
          );
        if (info.added > 0)
          addBadge(
            'pi-git-status-added',
            'N ' + info.added,
            t('git.newFiles').replace('{n}', String(info.added)),
          );
        if (info.deleted > 0)
          addBadge(
            'pi-git-status-deleted',
            'D ' + info.deleted,
            t('git.deletedFiles').replace('{n}', String(info.deleted)),
          );
        if (info.ahead > 0)
          addCommitBadge(
            'pi-git-status-ahead',
            ArrowUp,
            info.ahead,
            t('git.ahead').replace('{n}', String(info.ahead)),
          );
        if (info.behind > 0)
          addCommitBadge(
            'pi-git-status-behind',
            ArrowDown,
            info.behind,
            t('git.behind').replace('{n}', String(info.behind)),
          );

        const hasAny =
          info.modified > 0 ||
          info.added > 0 ||
          info.deleted > 0 ||
          info.ahead > 0 ||
          info.behind > 0;
        statusEl.hidden = !hasAny;
      }
      if (items.manual) items.manual.title = prCreateUrl ? prCreateUrl : t('git.noRemote');

      const isDefault = !!info.isDefault;
      const hasPr = !isDefault && !!existingPrUrl;
      const hasChanges = !!info.hasChanges;

      const plan = planActions({ isDefault, hasPr, hasChanges });
      const primary = plan.primary ? ACTIONS[plan.primary] : null;
      if (primary) {
        if (primaryLabel) {
          primaryLabel.textContent = primary.label;
          if (primary.external) primaryLabel.innerHTML += ' ' + icon(ExternalLink, { size: 12 });
        }
        primaryAction = primary.run;
      } else {
        primaryAction = () => {};
      }
      show(primaryBtn, !!primary);

      const secondary = new Set(plan.secondary);
      Object.keys(items).forEach((key) => show(items[key], secondary.has(key)));
      show(caretBtn, plan.secondary.length > 0);
      if (plan.secondary.length === 0) setMenuOpen(false);

      show(prWrap, !!primary || plan.secondary.length > 0);
      bar.hidden = false;
    }

    function refresh() {
      return gitApi
        .getGitInfo(sessionId)
        .then(applyInfo)
        .catch(() => {});
    }

    // ── Split button ──
    function setMenuOpen(open) {
      if (!prMenu || !caretBtn) return;
      prMenu.hidden = !open;
      caretBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
    }
    if (primaryBtn) {
      on(primaryBtn, 'click', (e) => {
        e.preventDefault();
        setMenuOpen(false);
        primaryAction();
      });
    }
    if (caretBtn) {
      on(caretBtn, 'click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setMenuOpen(prMenu ? prMenu.hidden : false);
      });
    }
    on(documentImpl, 'click', (e) => {
      if (prMenu && !prMenu.hidden && prWrap && !prWrap.contains(e.target)) setMenuOpen(false);
    });

    Object.keys(items).forEach((key) => {
      const el = items[key];
      if (!el) return;
      on(el, 'click', (e) => {
        e.preventDefault();
        setMenuOpen(false);
        ACTIONS[key].run();
      });
    });

    // ── Dirty files modal trigger ──
    if (branchWrap) {
      on(branchWrap, 'click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (isDirty) openDirtyFiles();
      });
    }

    // ── Periodic poll + session-reload listener ──
    let intervalId = undefined;
    if (si) intervalId = si(refresh, GIT_REFRESH_MS);

    const onSessionReload = () => {
      void refresh();
    };
    wi.addEventListener?.('pi-session-reload', onSessionReload);

    refresh();

    return () => {
      for (const fn of cleanups) fn();
      if (intervalId !== undefined) ci(intervalId);
      wi.removeEventListener?.('pi-session-reload', onSessionReload);
    };
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -- trusted: Lucide icon SVG and rendered session markdown -->

<div class="pi-git-bar" id="pi-git-bar">
  <div class="pi-git-branch" id="pi-git-branch" hidden>
    <span class="pi-git-branch-name" id="pi-git-branch-name" title={t('git.currentBranch')}
    ></span><span class="pi-git-status" id="pi-git-status" hidden></span>
  </div>
  <div class="pi-git-right">
    <CompactButton
      {sessionId}
    /><button type="button" class="pi-footer-button pi-btw-button" id="pi-btw-button" title="btw"
      >btw</button
    >
    <div class="pi-git-pr" id="pi-git-pr" hidden>
      <button type="button" class="pi-footer-button pi-git-primary" id="pi-git-primary"
        ><span id="pi-git-primary-label">{t('git.createPr')}</span></button
      ><button
        type="button"
        class="pi-footer-button pi-git-caret"
        id="pi-git-caret"
        aria-haspopup="true"
        aria-expanded="false"
        aria-label={t('git.moreActions')}>{@html icon(ChevronDown, { size: 12 })}</button
      >
      <div class="pi-git-pr-menu" id="pi-git-pr-menu" role="menu" hidden>
        <button type="button" class="pi-git-pr-item" id="pi-git-pr-view" role="menuitem" hidden
          >{t('git.viewPr')} {@html icon(ExternalLink, { size: 12 })}</button
        ><button type="button" class="pi-git-pr-item" id="pi-git-pr-draft" role="menuitem" hidden
          >{t('git.createDraftPr')}</button
        ><button type="button" class="pi-git-pr-item" id="pi-git-pr-manual" role="menuitem"
          >{t('git.createPrManually')} {@html icon(ExternalLink, { size: 12 })}</button
        ><button type="button" class="pi-git-pr-item" id="pi-git-pr-merge" role="menuitem" hidden
          >{t('git.mergePr')}</button
        ><button type="button" class="pi-git-pr-item" id="pi-git-pr-commit" role="menuitem" hidden
          >{t('git.commitPush')}</button
        >
      </div>
    </div>
  </div>
</div>
